package web

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxAIPDFPages       = 40
	maxAIPDFImageBytes  = 48 * 1024 * 1024
	maxAIPDFBatchImages = 4
)

// extractPDFLocallyOrWithVision keeps PDF preprocessing on the gateway. Text
// PDFs never leave the gateway as files; scanned pages are rasterized locally
// and only their images are sent to the configured DeepSeek vision model.
func (manager *aiManager) extractPDFLocallyOrWithVision(ctx context.Context, provider aiProviderSettings, filePath string) (string, error) {
	if text := extractLocalPDFText(ctx, filePath); usefulPDFText(text) {
		if len(text) > maxOCRBytes {
			return "", errors.New("PDF 文本超过处理上限，请拆分文档后重试")
		}
		return text, nil
	}

	pages, cleanup, err := renderLocalPDFPages(ctx, manager.tempDir, filePath)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return "", err
	}
	return manager.extractTextFromPDFPages(ctx, provider, pages)
}

func extractLocalPDFText(ctx context.Context, filePath string) string {
	if binary, err := exec.LookPath("pdftotext"); err == nil {
		command := exec.CommandContext(ctx, binary, "-layout", "-enc", "UTF-8", "-nopgbrk", filePath, "-")
		if raw, err := command.Output(); err == nil && usefulPDFText(string(raw)) {
			return strings.TrimSpace(string(raw))
		}
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return extractEmbeddedPDFText(raw)
}

func renderLocalPDFPages(ctx context.Context, tempRoot, filePath string) ([]string, func(), error) {
	binary, err := exec.LookPath("pdftoppm")
	if err != nil {
		return nil, nil, errors.New("该 PDF 未包含可提取文字；扫描件需要在网关安装 PDF 页面转换组件 pdftoppm 后才能由 DeepSeek 视觉模型识别")
	}
	directory, err := os.MkdirTemp(tempRoot, "pdf-pages-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	prefix := filepath.Join(directory, "page")
	command := exec.CommandContext(ctx, binary, "-jpeg", "-r", "120", "-f", "1", "-l", fmt.Sprint(maxAIPDFPages), filePath, prefix)
	if raw, err := command.CombinedOutput(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("PDF 页面转换失败: %s", valueOrDefault(strings.TrimSpace(string(raw)), err.Error()))
	}
	pages, _ := filepath.Glob(prefix + "-*.jpg")
	sort.Strings(pages)
	if len(pages) == 0 {
		cleanup()
		return nil, nil, errors.New("PDF 页面转换后没有生成可识别图像")
	}
	var total int64
	for _, page := range pages {
		info, statErr := os.Stat(page)
		if statErr != nil {
			cleanup()
			return nil, nil, statErr
		}
		total += info.Size()
	}
	if total > maxAIPDFImageBytes {
		cleanup()
		return nil, nil, errors.New("PDF 转换后的页面图像超过 48 MB，请拆分文档或降低扫描分辨率后重试")
	}
	return pages, cleanup, nil
}

func (manager *aiManager) extractTextFromPDFPages(ctx context.Context, provider aiProviderSettings, pages []string) (string, error) {
	var extracted strings.Builder
	for start := 0; start < len(pages); start += maxAIPDFBatchImages {
		end := start + maxAIPDFBatchImages
		if end > len(pages) {
			end = len(pages)
		}
		content := []interface{}{map[string]interface{}{
			"type": "text",
			"text": "逐页抄录这些工业设备文档页面。完整保留设备通信参数、表头、每一行点位、地址、功能码、寄存器、IOA、对象引用、数据类型、字节序、单位、倍率和小数位。不要总结，不要推断缺失值。",
		}}
		for _, page := range pages[start:end] {
			raw, err := os.ReadFile(page)
			if err != nil {
				return "", err
			}
			content = append(content, map[string]interface{}{
				"type":      "image_url",
				"image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw)},
			})
		}
		payload := map[string]interface{}{
			"model": provider.Model,
			"messages": []interface{}{map[string]interface{}{
				"role": "user", "content": content,
			}},
			"stream": false,
		}
		var response struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := manager.doJSON(ctx, "POST", apiEndpoint(provider.BaseURL, "chat/completions"), provider.APIKey, payload, &response); err != nil {
			return "", fmt.Errorf("DeepSeek 扫描页识别失败（第 %d-%d 页）: %w", start+1, end, err)
		}
		if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
			return "", fmt.Errorf("DeepSeek 未返回第 %d-%d 页的识别内容", start+1, end)
		}
		fmt.Fprintf(&extracted, "\n--- PDF 第 %d-%d 页 ---\n%s\n", start+1, end, response.Choices[0].Message.Content)
		if extracted.Len() > maxOCRBytes {
			return "", errors.New("PDF 识别结果过大，请拆分文档后重试")
		}
	}
	return strings.TrimSpace(extracted.String()), nil
}

func extractEmbeddedPDFText(raw []byte) string {
	var chunks []string
	for _, stream := range pdfContentStreams(raw) {
		chunks = append(chunks, pdfTextOperators(stream))
		if inflated, ok := inflatePDFStream(stream); ok {
			chunks = append(chunks, pdfTextOperators(inflated))
		}
	}
	return strings.TrimSpace(strings.Join(chunks, "\n"))
}

func pdfContentStreams(raw []byte) [][]byte {
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

func inflatePDFStream(raw []byte) ([]byte, bool) {
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, false
	}
	defer reader.Close()
	inflated, err := io.ReadAll(io.LimitReader(reader, maxOCRBytes+1))
	return inflated, err == nil && len(inflated) <= maxOCRBytes
}

var (
	pdfLiteralText = regexp.MustCompile(`\(([^()]*)\)\s*Tj`)
	pdfArrayText   = regexp.MustCompile(`(?s)\[(.*?)\]\s*TJ`)
	pdfNestedText  = regexp.MustCompile(`\(([^()]*)\)`)
	pdfHexText     = regexp.MustCompile(`<([0-9A-Fa-f]{4,})>\s*Tj`)
)

func pdfTextOperators(raw []byte) string {
	text := string(raw)
	var parts []string
	for _, item := range pdfLiteralText.FindAllStringSubmatch(text, -1) {
		parts = append(parts, unescapePDFLiteral(item[1]))
	}
	for _, item := range pdfArrayText.FindAllStringSubmatch(text, -1) {
		for _, nested := range pdfNestedText.FindAllStringSubmatch(item[1], -1) {
			parts = append(parts, unescapePDFLiteral(nested[1]))
		}
	}
	for _, item := range pdfHexText.FindAllStringSubmatch(text, -1) {
		if decoded, err := hex.DecodeString(item[1]); err == nil {
			parts = append(parts, readablePDFText(decoded))
		}
	}
	return strings.Join(parts, "\n")
}

func unescapePDFLiteral(value string) string {
	replacer := strings.NewReplacer(`\\`, `\`, `\(`, `(`, `\)`, `)`, `\n`, "\n", `\r`, "\n", `\t`, "\t")
	return replacer.Replace(value)
}

func readablePDFText(raw []byte) string {
	if !utf8.Valid(raw) {
		return ""
	}
	var builder strings.Builder
	for _, r := range string(raw) {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func usefulPDFText(value string) bool {
	value = strings.TrimSpace(value)
	if len([]rune(value)) < 40 {
		return false
	}
	var useful int
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			useful++
		}
	}
	return useful >= 24
}

func aiPDFProtocolSupport(protocol string) (bool, string) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "opcda" {
		return false, "OPC DA 点位应通过一键浏览从服务器获取，不使用 PDF 生成；PDF 导入主要用于厂家点表和协议说明书"
	}
	switch protocol {
	case "modbus-tcp", "modbus-rtu", "dlt645", "iec104", "siemens-s7", "iec61850", "iec61850-goose", "opcua":
		return true, ""
	default:
		return false, "当前协议暂未配置 PDF 字段映射，不能安全生成设备和点位"
	}
}

func appendUniqueStrings(existing []string, values ...string) []string {
	seen := make(map[string]bool, len(existing)+len(values))
	result := make([]string, 0, len(existing)+len(values))
	for _, value := range append(existing, values...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
