package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/edgecompute"
)

const (
	maxConfigPointWindow      = 1000
	largeConfigPointThreshold = 50000
)

var trailingPointNumber = regexp.MustCompile(`^(.*?)(\d+)$`)

type configBootstrapResponse struct {
	Content     string         `json:"content"`
	PointCounts map[string]int `json:"pointCounts"`
	TotalPoints int            `json:"totalPoints"`
	LargeMode   bool           `json:"largeMode"`
}

func (s *Server) runtimeConfig() (config.Config, error) {
	if s.runtime != nil {
		return s.runtime.Config(), nil
	}
	return config.Load(s.configPath)
}

func compactConfigForUI(source config.Config) (config.Config, map[string]int, int) {
	result := source
	counts := make(map[string]int, len(source.Devices)+len(source.ForwardDevices))
	result.Points = nil
	result.Devices = append([]config.DeviceConfig(nil), source.Devices...)
	total := 0
	for index := range result.Devices {
		count := len(source.Devices[index].Points)
		counts[result.Devices[index].DeviceKey] = count
		total += count
		result.Devices[index].Points = nil
	}
	result.ForwardDevices = append([]config.ForwardDeviceConfig(nil), source.ForwardDevices...)
	for index := range result.ForwardDevices {
		count := len(source.ForwardDevices[index].Points)
		counts[result.ForwardDevices[index].DeviceKey] = count
		total += count
		result.ForwardDevices[index].Points = nil
	}
	return config.RedactSecrets(result), counts, total
}

func (s *Server) configBootstrap(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	compact, counts, total := compactConfigForUI(cfg)
	raw, err := json.Marshal(compact)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if revision, err := configFileRevision(s.configPath); err == nil {
		writer.Header().Set("ETag", quoteETag(revision))
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(configBootstrapResponse{
		Content: string(raw), PointCounts: counts, TotalPoints: total, LargeMode: total > largeConfigPointThreshold,
	})
}

func (s *Server) configPoints(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	deviceKey := strings.TrimSpace(request.URL.Query().Get("deviceKey"))
	if deviceKey == "" {
		http.Error(writer, "deviceKey is required", http.StatusBadRequest)
		return
	}
	offset := queryNonNegativeInt(request, "offset", 0)
	limit := queryNonNegativeInt(request, "limit", 200)
	if limit < 1 {
		limit = 200
	}
	if limit > maxConfigPointWindow {
		limit = maxConfigPointWindow
	}
	search := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("search")))
	function := queryNonNegativeInt(request, "function", 0)
	cfg, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, device := range cfg.Devices {
		if device.DeviceKey != deviceKey {
			continue
		}
		items := make([]config.PointConfig, 0, limit)
		total := 0
		for _, point := range device.Points {
			if function > 0 && int(point.Function) != function {
				continue
			}
			if search != "" && !strings.Contains(strings.ToLower(point.Name+" "+point.Metric), search) {
				continue
			}
			if total >= offset && len(items) < limit {
				point.Password = ""
				items = append(items, point)
			}
			total++
		}
		writeConfigPointWindow(writer, offset, limit, total, items)
		return
	}
	for _, device := range cfg.ForwardDevices {
		if device.DeviceKey != deviceKey {
			continue
		}
		items := make([]config.ForwardPointConfig, 0, limit)
		total := 0
		for _, point := range device.Points {
			if function > 0 && int(point.Function) != function {
				continue
			}
			if search != "" && !strings.Contains(strings.ToLower(point.Name+" "+point.Metric), search) {
				continue
			}
			if total >= offset && len(items) < limit {
				items = append(items, point)
			}
			total++
		}
		writeForwardConfigPointWindow(writer, offset, limit, total, items)
		return
	}
	http.Error(writer, "device not found", http.StatusNotFound)
}

// saveCompactConfig saves resource/channel/device metadata while preserving
// the server-side point arrays omitted from the large-project bootstrap.
func (s *Server) saveCompactConfig(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if expected := strings.Trim(strings.TrimSpace(request.Header.Get("If-Match")), "\""); expected != "" && expected != "*" {
		currentRevision, err := configFileRevision(s.configPath)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if expected != currentRevision {
			http.Error(writer, "配置已被其他会话修改，请重新加载后再保存", http.StatusConflict)
			return
		}
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 16*1024*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	next, err := config.Parse(raw)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	current, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	collectPoints := make(map[string][]config.PointConfig, len(current.Devices))
	for _, device := range current.Devices {
		collectPoints[device.DeviceKey] = device.Points
	}
	for index := range next.Devices {
		next.Devices[index].Points = collectPoints[next.Devices[index].DeviceKey]
	}
	forwardPoints := make(map[string][]config.ForwardPointConfig, len(current.ForwardDevices))
	for _, device := range current.ForwardDevices {
		forwardPoints[device.DeviceKey] = device.Points
	}
	for index := range next.ForwardDevices {
		next.ForwardDevices[index].Points = forwardPoints[next.ForwardDevices[index].DeviceKey]
	}
	config.PreserveMaskedSecrets(current, &next)
	next.ApplyDefaults()
	if err := next.ValidateNewGlobalIdentifierDuplicates(current); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := edgecompute.Validate(next); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.Save(s.configPath, next); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.applyConfig(next); err != nil {
		_ = config.Save(s.configPath, current)
		_ = s.applyConfig(current)
		http.Error(writer, "apply config failed and previous configuration was restored: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if revision, err := configFileRevision(s.configPath); err == nil {
		writer.Header().Set("ETag", quoteETag(revision))
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "restartRequired": false})
}

type pointMutationRequest struct {
	DeviceKey string          `json:"deviceKey"`
	Action    string          `json:"action"`
	Metrics   []string        `json:"metrics,omitempty"`
	OldMetric string          `json:"oldMetric,omitempty"`
	NewMetric string          `json:"newMetric,omitempty"`
	Range     string          `json:"range,omitempty"`
	Search    string          `json:"search,omitempty"`
	Function  int             `json:"function,omitempty"`
	Field     string          `json:"field,omitempty"`
	Mode      string          `json:"mode,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
	Step      float64         `json:"step,omitempty"`
	Prefix    string          `json:"prefix,omitempty"`
	Suffix    string          `json:"suffix,omitempty"`
}

func (s *Server) mutateConfigPoints(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body pointMutationRequest
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 2*1024*1024)).Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	body.DeviceKey = strings.TrimSpace(body.DeviceKey)
	body.Action = strings.ToLower(strings.TrimSpace(body.Action))
	if body.DeviceKey == "" {
		http.Error(writer, "deviceKey is required", http.StatusBadRequest)
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if !s.matchesConfigRevision(writer, request) {
		return
	}
	current, err := s.runtimeConfig()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	next := current
	deviceIndex := -1
	for index := range next.Devices {
		if next.Devices[index].DeviceKey == body.DeviceKey {
			deviceIndex = index
			break
		}
	}
	if deviceIndex < 0 {
		http.Error(writer, "collection device not found", http.StatusNotFound)
		return
	}
	next.Devices = append([]config.DeviceConfig(nil), current.Devices...)
	device := &next.Devices[deviceIndex]
	beforeCount := len(current.Devices[deviceIndex].Points)
	// Deleting every point does not need a second 300k-element working copy.
	// The current configuration retains the original slice for rollback.
	deleteWholeDevice := deletesWholeDevice(body)
	if deleteWholeDevice {
		device.Points = nil
	} else {
		device.Points = append([]config.PointConfig(nil), current.Devices[deviceIndex].Points...)
	}
	selected := make(map[string]struct{}, len(body.Metrics))
	for _, metric := range body.Metrics {
		selected[metric] = struct{}{}
	}
	matches := func(point config.PointConfig) bool {
		if body.Range != "all" {
			_, ok := selected[point.Metric]
			return ok
		}
		if body.Function > 0 && int(point.Function) != body.Function {
			return false
		}
		search := strings.ToLower(strings.TrimSpace(body.Search))
		return search == "" || strings.Contains(strings.ToLower(point.Name+" "+point.Metric), search)
	}
	created := []string{}
	switch body.Action {
	case "add":
		used := configPointMetrics(next)
		metric := nextAvailableMetric(protocolMetricPrefix(device.Protocol)+"-P01", used)
		function := body.Function
		if function < 1 || function > 4 {
			function = 3
		}
		point := config.PointConfig{DeviceKey: device.DeviceKey, ChannelKey: device.ChannelKey, Name: device.Name + "@" + metric, Metric: metric, Protocol: device.Protocol, Address: device.Address, SlaveID: device.SlaveID, CommonAddress: device.CommonAddress, Function: byte(function), IOA: uint32(len(device.Points) + 1), Quantity: 1, DataType: "uint16", ByteOrder: "big", WordOrder: "big", Scale: 1}
		if function == 1 || function == 2 {
			point.DataType = "bool"
		}
		device.Points = append(device.Points, point)
		created = append(created, metric)
	case "duplicate":
		used := configPointMetrics(next)
		var copies []config.PointConfig
		for _, point := range device.Points {
			if !matches(point) {
				continue
			}
			copyPoint := point
			copyPoint.Metric = nextAvailableMetric(point.Metric, used)
			copyPoint.Name = copiedPointName(point.Name, point.Metric, copyPoint.Metric)
			used[strings.ToLower(copyPoint.Metric)] = struct{}{}
			copies = append(copies, copyPoint)
			created = append(created, copyPoint.Metric)
		}
		device.Points = append(device.Points, copies...)
	case "delete":
		if !deleteWholeDevice {
			kept := device.Points[:0]
			for _, point := range device.Points {
				if !matches(point) {
					kept = append(kept, point)
				}
			}
			device.Points = kept
		}
	case "batch":
		if body.Field == "metric" {
			clonePointReferences(&next)
		}
		if err := applyPointBatch(device, body, matches, next); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Field == "metric" {
			renames := make(map[string]string)
			for index, point := range device.Points {
				oldMetric := current.Devices[deviceIndex].Points[index].Metric
				if oldMetric != point.Metric {
					renames[oldMetric] = point.Metric
				}
			}
			updatePointMetricReferences(&next, device.DeviceKey, renames)
		}
	case "rename":
		clonePointReferences(&next)
		if err := renamePointMetric(&next, device, body.OldMetric, body.NewMetric); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(writer, "unsupported point mutation action", http.StatusBadRequest)
		return
	}
	if err := config.Save(s.configPath, next); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.applyConfig(next); err != nil {
		_ = config.Save(s.configPath, current)
		_ = s.applyConfig(current)
		http.Error(writer, "apply config failed and previous configuration was restored: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if revision, err := configFileRevision(s.configPath); err == nil {
		writer.Header().Set("ETag", quoteETag(revision))
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok": true, "count": len(device.Points), "deleted": beforeCount - len(device.Points), "createdMetrics": created,
	})
}

func renamePointMetric(cfg *config.Config, device *config.DeviceConfig, oldMetric, newMetric string) error {
	oldMetric = strings.TrimSpace(oldMetric)
	newMetric = strings.TrimSpace(newMetric)
	if oldMetric == "" {
		return fmt.Errorf("原标识符不能为空")
	}
	if newMetric == "" {
		return fmt.Errorf("新标识符不能为空")
	}
	if oldMetric == newMetric {
		return nil
	}
	targetIndex := -1
	for index := range device.Points {
		if device.Points[index].Metric == oldMetric {
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		return fmt.Errorf("点位标识符 %s 不存在，请重新加载后再试", oldMetric)
	}
	for metric := range configPointMetrics(*cfg) {
		if strings.EqualFold(metric, newMetric) && !strings.EqualFold(metric, oldMetric) {
			return fmt.Errorf("点位标识符 %s 已存在，请使用全局唯一标识符", newMetric)
		}
	}
	point := &device.Points[targetIndex]
	point.Metric = newMetric
	point.Name = copiedPointName(point.Name, oldMetric, newMetric)
	updatePointMetricReferences(cfg, device.DeviceKey, map[string]string{oldMetric: newMetric})
	return nil
}

func clonePointReferences(cfg *config.Config) {
	cfg.ForwardDevices = append([]config.ForwardDeviceConfig(nil), cfg.ForwardDevices...)
	for deviceIndex := range cfg.ForwardDevices {
		cfg.ForwardDevices[deviceIndex].Points = append([]config.ForwardPointConfig(nil), cfg.ForwardDevices[deviceIndex].Points...)
	}
	cfg.EdgeComputing.Groups = append([]config.EdgeComputeGroupConfig(nil), cfg.EdgeComputing.Groups...)
	for groupIndex := range cfg.EdgeComputing.Groups {
		cfg.EdgeComputing.Groups[groupIndex].Points = append([]config.EdgeComputedPointConfig(nil), cfg.EdgeComputing.Groups[groupIndex].Points...)
		for pointIndex := range cfg.EdgeComputing.Groups[groupIndex].Points {
			point := &cfg.EdgeComputing.Groups[groupIndex].Points[pointIndex]
			point.Inputs = append([]config.EdgeComputeInputConfig(nil), point.Inputs...)
		}
	}
}

// Identifier changes are atomic from the user's perspective. Keep every
// forwarding and edge-compute source reference attached to the same point.
func updatePointMetricReferences(cfg *config.Config, deviceKey string, renames map[string]string) {
	if len(renames) == 0 {
		return
	}
	for deviceIndex := range cfg.ForwardDevices {
		for pointIndex := range cfg.ForwardDevices[deviceIndex].Points {
			forwardPoint := &cfg.ForwardDevices[deviceIndex].Points[pointIndex]
			newMetric, ok := renames[forwardPoint.SourceMetric]
			if forwardPoint.SourceDeviceKey != deviceKey || !ok {
				continue
			}
			oldSourceMetric := forwardPoint.SourceMetric
			forwardPoint.SourceMetric = newMetric
			if oldMetric := forwardPoint.Metric; renames[oldMetric] == newMetric {
				forwardPoint.Metric = newMetric
			}
			forwardPoint.Name = copiedPointName(forwardPoint.Name, oldSourceMetric, newMetric)
		}
	}
	for groupIndex := range cfg.EdgeComputing.Groups {
		for pointIndex := range cfg.EdgeComputing.Groups[groupIndex].Points {
			for inputIndex := range cfg.EdgeComputing.Groups[groupIndex].Points[pointIndex].Inputs {
				input := &cfg.EdgeComputing.Groups[groupIndex].Points[pointIndex].Inputs[inputIndex]
				if newMetric, ok := renames[input.SourceMetric]; input.SourceDeviceKey == deviceKey && ok {
					input.SourceMetric = newMetric
				}
			}
		}
	}
}

func deletesWholeDevice(body pointMutationRequest) bool {
	return body.Action == "delete" &&
		body.Range == "all" &&
		body.Function <= 0 &&
		strings.TrimSpace(body.Search) == ""
}

func (s *Server) matchesConfigRevision(writer http.ResponseWriter, request *http.Request) bool {
	expected := strings.Trim(strings.TrimSpace(request.Header.Get("If-Match")), "\"")
	if expected == "" || expected == "*" {
		return true
	}
	current, err := configFileRevision(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return false
	}
	if expected != current {
		http.Error(writer, "配置已被其他会话修改，请重新加载后再操作", http.StatusConflict)
		return false
	}
	return true
}

func configPointMetrics(cfg config.Config) map[string]struct{} {
	used := make(map[string]struct{}, len(cfg.Points))
	for _, device := range cfg.Devices {
		for _, point := range device.Points {
			used[strings.ToLower(point.Metric)] = struct{}{}
		}
	}
	for _, group := range cfg.EdgeComputing.Groups {
		for _, point := range group.Points {
			used[strings.ToLower(point.Metric)] = struct{}{}
		}
	}
	return used
}

func nextAvailableMetric(source string, used map[string]struct{}) string {
	source = strings.TrimSpace(source)
	if source != "" {
		if _, exists := used[strings.ToLower(source)]; !exists {
			return source
		}
	}
	match := trailingPointNumber.FindStringSubmatch(source)
	prefix, width, number := source+"-", 3, 1
	if len(match) == 3 {
		prefix, width = match[1], len(match[2])
		number, _ = strconv.Atoi(match[2])
		number++
	}
	for {
		metric := prefix + fmt.Sprintf("%0*d", width, number)
		if _, exists := used[strings.ToLower(metric)]; !exists {
			return metric
		}
		number++
	}
}

func protocolMetricPrefix(protocol string) string {
	switch strings.ToLower(protocol) {
	case "modbus-tcp":
		return "mbtcp"
	case "modbus-rtu":
		return "mbrtu"
	case "iec104":
		return "iec104"
	case "siemens-s7":
		return "s7"
	case "opcua":
		return "opcua"
	case "iec61850":
		return "iec61850"
	default:
		return "point"
	}
}

func copiedPointName(name, oldMetric, newMetric string) string {
	if strings.HasSuffix(name, oldMetric) {
		return strings.TrimSuffix(name, oldMetric) + newMetric
	}
	return name + "@" + newMetric
}

func applyPointBatch(device *config.DeviceConfig, body pointMutationRequest, matches func(config.PointConfig) bool, cfg config.Config) error {
	var text string
	var number float64
	_ = json.Unmarshal(body.Value, &text)
	if err := json.Unmarshal(body.Value, &number); err != nil {
		number, _ = strconv.ParseFloat(text, 64)
	}
	if body.Step == 0 {
		body.Step = 1
	}
	used := configPointMetrics(cfg)
	for _, point := range device.Points {
		if matches(point) && body.Field == "metric" {
			delete(used, strings.ToLower(point.Metric))
		}
	}
	index := 0
	for pointIndex := range device.Points {
		point := &device.Points[pointIndex]
		if !matches(*point) {
			continue
		}
		currentNumber := number
		if body.Mode == "increment" {
			currentNumber += float64(index) * body.Step
		} else if body.Mode == "decrement" {
			currentNumber -= float64(index) * body.Step
		}
		valueText := text
		if body.Mode == "increment" || body.Mode == "decrement" {
			valueText = strconv.FormatFloat(currentNumber, 'f', -1, 64)
		}
		valueText = body.Prefix + valueText + body.Suffix
		switch body.Field {
		case "name":
			point.Name = valueText
		case "metric":
			metric := nextAvailableMetric(valueText, used)
			oldMetric := point.Metric
			point.Metric = metric
			point.Name = copiedPointName(point.Name, oldMetric, metric)
			used[strings.ToLower(metric)] = struct{}{}
		case "register":
			point.Register = uint16(currentNumber)
		case "quantity":
			point.Quantity = uint16(currentNumber)
		case "ioa":
			point.IOA = uint32(currentNumber)
		case "dataType":
			point.DataType = valueText
		case "byteOrder":
			point.ByteOrder = valueText
		case "scale":
			point.Scale = currentNumber
		case "offset":
			point.Offset = currentNumber
		case "unit":
			point.Unit = valueText
		case "decimals":
			point.Decimals = int(currentNumber)
		default:
			return fmt.Errorf("unsupported batch field %s", body.Field)
		}
		index++
	}
	return nil
}

func queryNonNegativeInt(request *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(request.URL.Query().Get(key))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func writeConfigPointWindow(writer http.ResponseWriter, offset, limit, total int, points []config.PointConfig) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"offset": offset, "limit": limit, "total": total, "points": points,
	})
}

func writeForwardConfigPointWindow(writer http.ResponseWriter, offset, limit, total int, points []config.ForwardPointConfig) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"offset": offset, "limit": limit, "total": total, "points": points,
	})
}
