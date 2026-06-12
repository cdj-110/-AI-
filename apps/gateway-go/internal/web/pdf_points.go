package web

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const maxPDFUploadBytes = 15 << 20

type pdfPointPreviewResponse struct {
	Points     []pdfPointPreviewItem `json:"points"`
	Warnings   []string              `json:"warnings,omitempty"`
	TextSample string                `json:"textSample,omitempty"`
}

type pdfPointPreviewItem struct {
	config.PointConfig
	Selected   bool    `json:"selected"`
	Confidence float64 `json:"confidence"`
	SourceLine string  `json:"sourceLine,omitempty"`
}

func (s *Server) previewPDFPoints(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxPDFUploadBytes)
	if err := request.ParseMultipartForm(maxPDFUploadBytes); err != nil {
		http.Error(writer, "PDF 文件过大或表单格式不正确："+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		http.Error(writer, "请选择 PDF 文件", http.StatusBadRequest)
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		http.Error(writer, "读取 PDF 失败："+err.Error(), http.StatusBadRequest)
		return
	}
	if len(raw) == 0 {
		http.Error(writer, "PDF 文件为空", http.StatusBadRequest)
		return
	}
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") && !bytes.HasPrefix(raw, []byte("%PDF")) {
		http.Error(writer, "当前只支持 PDF 文件", http.StatusBadRequest)
		return
	}

	options := pointParseOptions{
		DeviceKey: request.FormValue("deviceKey"),
		Protocol:  valueOrDefault(request.FormValue("protocol"), "modbus-tcp"),
		Address:   request.FormValue("address"),
		SlaveID:   parseByteDefault(request.FormValue("slaveId"), 1),
	}
	text := extractPDFText(raw)
	points, warnings := parseModbusPointsFromText(text, options)
	if len(points) == 0 {
		warnings = append(warnings, "未识别到寄存器表。请确认 PDF 不是扫描图片，或表格中包含地址/功能码/数据类型等文本。")
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(pdfPointPreviewResponse{
		Points:     points,
		Warnings:   warnings,
		TextSample: sampleText(text, 1200),
	})
}

type pointParseOptions struct {
	DeviceKey string
	Protocol  string
	Address   string
	SlaveID   byte
}

type parsedPointCandidate struct {
	name         string
	metric       string
	functionCode uint8
	register     uint16
	quantity     uint16
	dataType     string
	scale        float64
	offset       float64
	score        int
	line         string
}

func extractPDFText(raw []byte) string {
	var chunks []string
	chunks = append(chunks, readableText(string(raw)))
	for _, stream := range pdfStreams(raw) {
		chunks = append(chunks, readableText(string(stream)))
		if inflated, ok := inflateZlib(stream); ok {
			chunks = append(chunks, readableText(string(inflated)))
			chunks = append(chunks, pdfTextOperators(inflated))
		}
		chunks = append(chunks, pdfTextOperators(stream))
	}
	return strings.Join(chunks, "\n")
}

func pdfStreams(raw []byte) [][]byte {
	var streams [][]byte
	search := raw
	for {
		start := bytes.Index(search, []byte("stream"))
		if start < 0 {
			break
		}
		contentStart := start + len("stream")
		if contentStart < len(search) && search[contentStart] == '\r' {
			contentStart++
		}
		if contentStart < len(search) && search[contentStart] == '\n' {
			contentStart++
		}
		end := bytes.Index(search[contentStart:], []byte("endstream"))
		if end < 0 {
			break
		}
		streams = append(streams, search[contentStart:contentStart+end])
		search = search[contentStart+end+len("endstream"):]
	}
	return streams
}

func inflateZlib(raw []byte) ([]byte, bool) {
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, false
	}
	defer reader.Close()
	inflated, err := io.ReadAll(reader)
	return inflated, err == nil
}

func pdfTextOperators(raw []byte) string {
	text := string(raw)
	var parts []string
	literal := regexp.MustCompile(`\(([^()]*)\)\s*Tj`).FindAllStringSubmatch(text, -1)
	for _, item := range literal {
		parts = append(parts, unescapePDFString(item[1]))
	}
	arrayLiteral := regexp.MustCompile(`\[(.*?)\]\s*TJ`).FindAllStringSubmatch(text, -1)
	for _, item := range arrayLiteral {
		for _, nested := range regexp.MustCompile(`\(([^()]*)\)`).FindAllStringSubmatch(item[1], -1) {
			parts = append(parts, unescapePDFString(nested[1]))
		}
	}
	hexText := regexp.MustCompile(`<([0-9A-Fa-f]{4,})>\s*Tj`).FindAllStringSubmatch(text, -1)
	for _, item := range hexText {
		if decoded, err := hex.DecodeString(item[1]); err == nil {
			parts = append(parts, readableText(string(decoded)))
		}
	}
	return strings.Join(parts, "\n")
}

func unescapePDFString(value string) string {
	replacer := strings.NewReplacer(`\\`, `\`, `\(`, `(`, `\)`, `)`, `\n`, "\n", `\r`, "\n", `\t`, "\t")
	return replacer.Replace(value)
}

func readableText(value string) string {
	var builder strings.Builder
	lastSpace := false
	for _, r := range value {
		keep := r == '\n' || r == '\r' || r == '\t' || unicode.IsPrint(r)
		if !keep {
			if !lastSpace {
				builder.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				builder.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		builder.WriteRune(r)
		lastSpace = false
	}
	return builder.String()
}

func parseModbusPointsFromText(text string, options pointParseOptions) ([]pdfPointPreviewItem, []string) {
	lines := normalizePDFLines(text)
	var candidates []parsedPointCandidate
	for _, line := range lines {
		if candidate, ok := parseModbusPointLine(line); ok {
			candidates = append(candidates, candidate)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].register < candidates[j].register
	})
	seen := map[string]bool{}
	var points []pdfPointPreviewItem
	for _, candidate := range candidates {
		key := fmt.Sprintf("%d-%d-%s", candidate.functionCode, candidate.register, candidate.metric)
		if seen[key] {
			continue
		}
		seen[key] = true
		point := config.PointConfig{
			DeviceKey: options.DeviceKey,
			Name:      candidate.name,
			Metric:    candidate.metric,
			Protocol:  options.Protocol,
			Address:   options.Address,
			SlaveID:   options.SlaveID,
			Function:  candidate.functionCode,
			Register:  candidate.register,
			Quantity:  candidate.quantity,
			DataType:  candidate.dataType,
			ByteOrder: "big",
			WordOrder: "normal",
			Scale:     candidate.scale,
			Offset:    candidate.offset,
		}
		point.ApplyDefaults()
		points = append(points, pdfPointPreviewItem{
			PointConfig: point,
			Selected:    true,
			Confidence:  confidenceFromScore(candidate.score),
			SourceLine:  candidate.line,
		})
		if len(points) >= 200 {
			break
		}
	}
	var warnings []string
	if len(candidates) > len(points) {
		warnings = append(warnings, fmt.Sprintf("已去重 %d 个疑似重复点位", len(candidates)-len(points)))
	}
	return points, warnings
}

func confidenceFromScore(score int) float64 {
	confidence := 0.35 + float64(score)*0.12
	if confidence > 0.98 {
		return 0.98
	}
	if confidence < 0.2 {
		return 0.2
	}
	return confidence
}

func normalizePDFLines(text string) []string {
	text = strings.ReplaceAll(text, "\r", "\n")
	splitter := regexp.MustCompile(`[\n;。]+`)
	rawLines := splitter.Split(text, -1)
	var lines []string
	for _, line := range rawLines {
		line = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(line, " "))
		if len([]rune(line)) >= 8 {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseModbusPointLine(line string) (parsedPointCandidate, bool) {
	register, functionCode, ok := findRegister(line)
	if !ok {
		return parsedPointCandidate{}, false
	}
	dataType, quantity := inferDataType(line)
	name := inferPointName(line, register)
	metric := inferMetric(name, register)
	scale := inferNumberAfterKeywords(line, []string{"倍率", "系数", "scale", "ratio"}, 1)
	offset := inferNumberAfterKeywords(line, []string{"偏移", "offset"}, 0)
	score := 1
	if strings.Contains(strings.ToLower(line), "modbus") || strings.Contains(line, "寄存器") {
		score += 2
	}
	if dataType != "uint16" {
		score++
	}
	if name != "" {
		score++
	}
	return parsedPointCandidate{
		name:         name,
		metric:       metric,
		functionCode: functionCode,
		register:     register,
		quantity:     quantity,
		dataType:     dataType,
		scale:        scale,
		offset:       offset,
		score:        score,
		line:         line,
	}, true
}

func findRegister(line string) (uint16, uint8, bool) {
	lower := strings.ToLower(line)
	if match := regexp.MustCompile(`0x([0-9a-f]{1,4})`).FindStringSubmatch(lower); len(match) == 2 {
		value, _ := strconv.ParseUint(match[1], 16, 16)
		return uint16(value), inferFunctionCode(line, uint32(value)), true
	}
	for _, match := range regexp.MustCompile(`\b([0-9]{1,6})\b`).FindAllStringSubmatch(line, -1) {
		raw := match[1]
		value64, _ := strconv.ParseUint(raw, 10, 32)
		value := uint32(value64)
		if value > 65535 && !(value >= 300001 && value <= 465535) {
			continue
		}
		if value >= 400001 {
			return uint16(value - 400001), 3, true
		}
		if value >= 40001 {
			return uint16(value - 40001), 3, true
		}
		if value >= 300001 {
			return uint16(value - 300001), 4, true
		}
		if value >= 30001 {
			return uint16(value - 30001), 4, true
		}
		if value >= 100001 {
			return uint16(value - 100001), 2, true
		}
		if value >= 10001 {
			return uint16(value - 10001), 2, true
		}
		if value >= 1 && value <= 9999 && regexp.MustCompile(`(?i)(寄存器|register|addr|address|地址|modbus|保持|输入|线圈)`).MatchString(line) {
			return uint16(value - 1), inferFunctionCode(line, value), true
		}
		if value <= 65535 && regexp.MustCompile(`(?i)(0-based|零基|偏移|offset)`).MatchString(line) {
			return uint16(value), inferFunctionCode(line, value), true
		}
	}
	return 0, 0, false
}

func inferFunctionCode(line string, _ uint32) uint8 {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(line, "线圈") || strings.Contains(lower, "coil") || regexp.MustCompile(`\bfc0?1\b`).MatchString(lower):
		return 1
	case strings.Contains(line, "离散") || strings.Contains(lower, "discrete") || regexp.MustCompile(`\bfc0?2\b`).MatchString(lower):
		return 2
	case strings.Contains(line, "输入寄存器") || strings.Contains(lower, "input register") || regexp.MustCompile(`\bfc0?4\b`).MatchString(lower):
		return 4
	default:
		return 3
	}
}

func inferDataType(line string) (string, uint16) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "float") || strings.Contains(line, "浮点"):
		return "float32", 2
	case strings.Contains(lower, "int32") || strings.Contains(line, "有符号32"):
		return "int32", 2
	case strings.Contains(lower, "uint32") || strings.Contains(line, "无符号32"):
		return "uint32", 2
	case strings.Contains(lower, "int16") || strings.Contains(line, "有符号16"):
		return "int16", 1
	case strings.Contains(lower, "bool") || strings.Contains(line, "开关") || strings.Contains(line, "状态"):
		return "bool", 1
	default:
		return "uint16", 1
	}
}

func inferPointName(line string, register uint16) string {
	line = regexp.MustCompile(`0x[0-9A-Fa-f]+|\b[0-9]{1,6}\b`).ReplaceAllString(line, " ")
	line = regexp.MustCompile(`(?i)modbus|register|address|addr|fc0?[1-4]|uint16|int16|uint32|int32|float32|float|bool`).ReplaceAllString(line, " ")
	line = strings.NewReplacer("寄存器", " ", "地址", " ", "功能码", " ", "保持", " ", "输入", " ", "线圈", " ", "数据类型", " ").Replace(line)
	line = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(line, " "))
	runes := []rune(line)
	if len(runes) > 24 {
		line = string(runes[:24])
	}
	if strings.TrimSpace(line) == "" {
		return fmt.Sprintf("点位%d", register)
	}
	return line
}

func inferMetric(name string, register uint16) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		if r == '_' || r == '-' {
			builder.WriteRune('_')
		}
	}
	metric := strings.Trim(builder.String(), "_")
	if metric == "" {
		metric = fmt.Sprintf("point_%d", register)
	}
	return metric
}

func inferNumberAfterKeywords(line string, keywords []string, fallback float64) float64 {
	for _, keyword := range keywords {
		pattern := regexp.MustCompile(regexp.QuoteMeta(keyword) + `\s*[:：=]?\s*(-?[0-9]+(?:\.[0-9]+)?)`)
		if match := pattern.FindStringSubmatch(strings.ToLower(line)); len(match) == 2 {
			if value, err := strconv.ParseFloat(match[1], 64); err == nil {
				return value
			}
		}
	}
	return fallback
}

func parseByteDefault(value string, fallback byte) byte {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 8)
	if err != nil || parsed == 0 {
		return fallback
	}
	return byte(parsed)
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func sampleText(text string, limit int) string {
	text = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(text, " "))
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
