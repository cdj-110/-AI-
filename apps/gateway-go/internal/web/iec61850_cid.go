package web

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const maxCIDUploadBytes = 8 << 20

type cidPointPreviewResponse struct {
	Points   []cidPointPreviewItem `json:"points"`
	Warnings []string              `json:"warnings,omitempty"`
	Summary  cidSummary            `json:"summary"`
}

type cidPointPreviewItem struct {
	config.PointConfig
	Selected bool   `json:"selected"`
	Source   string `json:"source,omitempty"`
}

type cidSummary struct {
	IEDName string `json:"iedName,omitempty"`
	LDCount int    `json:"ldCount"`
	LNCount int    `json:"lnCount"`
}

type sclFile struct {
	XMLName       xml.Name       `xml:"SCL"`
	Header        sclHeader      `xml:"Header"`
	IEDs          []sclIED       `xml:"IED"`
	DataTypeTmpls sclDataTypeTpl `xml:"DataTypeTemplates"`
}

type sclHeader struct {
	NameStructure string `xml:"nameStructure,attr"`
}

type sclIED struct {
	Name        string           `xml:"name,attr"`
	AccessPoint []sclAccessPoint `xml:"AccessPoint"`
}

type sclAccessPoint struct {
	Server sclServer `xml:"Server"`
}

type sclServer struct {
	LDevices []sclLDevice `xml:"LDevice"`
}

type sclLDevice struct {
	Inst string  `xml:"inst,attr"`
	Desc string  `xml:"desc,attr"`
	LN0  *sclLN0 `xml:"LN0"`
	LNs  []sclLN `xml:"LN"`
}

type sclLN0 struct {
	Prefix string `xml:"prefix,attr"`
	Class  string `xml:"lnClass,attr"`
	Inst   string `xml:"inst,attr"`
	Desc   string `xml:"desc,attr"`
	LNType string `xml:"lnType,attr"`
}

type sclLN struct {
	Prefix string `xml:"prefix,attr"`
	Class  string `xml:"lnClass,attr"`
	Inst   string `xml:"inst,attr"`
	Desc   string `xml:"desc,attr"`
	LNType string `xml:"lnType,attr"`
}

type sclDataTypeTpl struct {
	LNodeTypes []sclLNodeType `xml:"LNodeType"`
	DOTypes    []sclDOType    `xml:"DOType"`
	DATypes    []sclDAType    `xml:"DAType"`
}

type sclLNodeType struct {
	ID  string     `xml:"id,attr"`
	DOs []sclDORef `xml:"DO"`
}

type sclDORef struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
	Desc string `xml:"desc,attr"`
}

type sclDOType struct {
	ID  string     `xml:"id,attr"`
	DAs []sclDARef `xml:"DA"`
}

type sclDARef struct {
	Name  string `xml:"name,attr"`
	FC    string `xml:"fc,attr"`
	BType string `xml:"bType,attr"`
	Type  string `xml:"type,attr"`
}

type sclDAType struct {
	ID   string      `xml:"id,attr"`
	BDAs []sclBDARef `xml:"BDA"`
}

type sclBDARef struct {
	Name  string `xml:"name,attr"`
	BType string `xml:"bType,attr"`
	Type  string `xml:"type,attr"`
}

type cidTemplateIndex struct {
	lnTypes map[string]sclLNodeType
	doTypes map[string]sclDOType
	daTypes map[string]sclDAType
}

type cidLeaf struct {
	Path  string
	FC    string
	BType string
}

type cidPointOptions struct {
	DeviceKey string
	Protocol  string
	Address   string
}

func (s *Server) previewIEC61850CID(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxCIDUploadBytes)
	if err := request.ParseMultipartForm(maxCIDUploadBytes); err != nil {
		http.Error(writer, "CID/ICD upload is too large or invalid: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		http.Error(writer, "please choose a CID/ICD file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		http.Error(writer, "failed to read CID/ICD file: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.ToLower(header.Filename)
	if !strings.HasSuffix(name, ".cid") && !strings.HasSuffix(name, ".icd") && !strings.HasSuffix(name, ".scd") && !bytes.Contains(raw, []byte("<SCL")) {
		http.Error(writer, "only CID/ICD/SCD files are supported", http.StatusBadRequest)
		return
	}

	points, summary, warnings, err := parseCIDPoints(raw, cidPointOptions{
		DeviceKey: request.FormValue("deviceKey"),
		Protocol:  valueOrDefault(request.FormValue("protocol"), "iec61850"),
		Address:   request.FormValue("address"),
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(cidPointPreviewResponse{Points: points, Summary: summary, Warnings: warnings})
}

func parseCIDPoints(raw []byte, options cidPointOptions) ([]cidPointPreviewItem, cidSummary, []string, error) {
	var scl sclFile
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	if err := decoder.Decode(&scl); err != nil {
		return nil, cidSummary{}, nil, fmt.Errorf("failed to parse CID/ICD: %w", err)
	}

	index := cidTemplateIndex{
		lnTypes: map[string]sclLNodeType{},
		doTypes: map[string]sclDOType{},
		daTypes: map[string]sclDAType{},
	}
	for _, item := range scl.DataTypeTmpls.LNodeTypes {
		index.lnTypes[item.ID] = item
	}
	for _, item := range scl.DataTypeTmpls.DOTypes {
		index.doTypes[item.ID] = item
	}
	for _, item := range scl.DataTypeTmpls.DATypes {
		index.daTypes[item.ID] = item
	}

	seen := map[string]bool{}
	var points []cidPointPreviewItem
	var warnings []string
	summary := cidSummary{}
	for _, ied := range scl.IEDs {
		if summary.IEDName == "" {
			summary.IEDName = ied.Name
		}
		for _, ap := range ied.AccessPoint {
			for _, ld := range ap.Server.LDevices {
				summary.LDCount++
				ldName := cidLogicalDeviceName(scl.Header.NameStructure, ied.Name, ld.Inst)
				if ld.LN0 != nil {
					summary.LNCount++
					points = append(points, cidPointsForLN(index, seen, options, ldName, cidLNName(ld.LN0.Prefix, "LLN0", ld.LN0.Inst), ld.LN0.LNType, ld.LN0.Desc)...)
				}
				for _, ln := range ld.LNs {
					summary.LNCount++
					points = append(points, cidPointsForLN(index, seen, options, ldName, cidLNName(ln.Prefix, ln.Class, ln.Inst), ln.LNType, ln.Desc)...)
				}
			}
		}
	}
	if len(points) == 0 {
		warnings = append(warnings, "no importable IEC61850 points were found in the CID/ICD file")
	}
	sort.SliceStable(points, func(i, j int) bool {
		return points[i].ObjectRef < points[j].ObjectRef
	})
	return points, summary, warnings, nil
}

func cidPointsForLN(index cidTemplateIndex, seen map[string]bool, options cidPointOptions, ldName string, lnName string, lnTypeID string, lnDesc string) []cidPointPreviewItem {
	lnType, ok := index.lnTypes[lnTypeID]
	if !ok {
		return nil
	}
	var points []cidPointPreviewItem
	for _, doRef := range lnType.DOs {
		doType, ok := index.doTypes[doRef.Type]
		if !ok {
			continue
		}
		for _, leaf := range cidLeavesForDO(index, doType) {
			if !cidUsefulLeaf(leaf.Path, leaf.FC) {
				continue
			}
			objectRef := ldName + "/" + lnName + "." + doRef.Name + "." + leaf.Path
			if seen[objectRef] {
				continue
			}
			seen[objectRef] = true
			point := config.PointConfig{
				DeviceKey: options.DeviceKey,
				Name:      cidPointName(lnDesc, doRef.Desc, doRef.Name, leaf.Path),
				Metric:    cidMetric(objectRef),
				Protocol:  options.Protocol,
				Address:   options.Address,
				ObjectRef: objectRef,
				FC:        leaf.FC,
				Quantity:  1,
				DataType:  cidDataType(leaf.Path, leaf.BType),
				Scale:     1,
				Offset:    0,
			}
			point.ApplyDefaults()
			points = append(points, cidPointPreviewItem{
				PointConfig: point,
				Selected:    cidDefaultSelected(leaf.Path, leaf.FC),
				Source:      strings.Trim(strings.Join([]string{lnName, doRef.Name, leaf.Path}, "."), "."),
			})
		}
	}
	return points
}

func cidLeavesForDO(index cidTemplateIndex, doType sclDOType) []cidLeaf {
	var leaves []cidLeaf
	for _, da := range doType.DAs {
		if da.Name == "" || da.FC == "" {
			continue
		}
		if strings.EqualFold(da.BType, "Struct") && da.Type != "" {
			leaves = append(leaves, cidLeavesForDAType(index, da.Type, da.Name, da.FC, nil)...)
			continue
		}
		leaves = append(leaves, cidLeaf{Path: da.Name, FC: da.FC, BType: da.BType})
	}
	return leaves
}

func cidLeavesForDAType(index cidTemplateIndex, daTypeID string, prefix string, fc string, visiting map[string]bool) []cidLeaf {
	if visiting == nil {
		visiting = map[string]bool{}
	}
	if visiting[daTypeID] {
		return nil
	}
	visiting[daTypeID] = true
	defer delete(visiting, daTypeID)

	daType, ok := index.daTypes[daTypeID]
	if !ok {
		return nil
	}
	var leaves []cidLeaf
	for _, bda := range daType.BDAs {
		path := prefix + "." + bda.Name
		if strings.EqualFold(bda.BType, "Struct") && bda.Type != "" {
			leaves = append(leaves, cidLeavesForDAType(index, bda.Type, path, fc, visiting)...)
			continue
		}
		leaves = append(leaves, cidLeaf{Path: path, FC: fc, BType: bda.BType})
	}
	return leaves
}

func cidLogicalDeviceName(nameStructure string, iedName string, ldInst string) string {
	if strings.EqualFold(strings.TrimSpace(nameStructure), "IEDName") && strings.TrimSpace(iedName) != "" {
		return strings.TrimSpace(iedName) + strings.TrimSpace(ldInst)
	}
	return strings.TrimSpace(ldInst)
}

func cidLNName(prefix string, class string, inst string) string {
	if class == "" {
		class = "LLN0"
	}
	if class == "LLN0" {
		return strings.TrimSpace(prefix) + class
	}
	return strings.TrimSpace(prefix) + class + strings.TrimSpace(inst)
}

func cidUsefulLeaf(path string, fc string) bool {
	switch strings.ToUpper(strings.TrimSpace(fc)) {
	case "MX", "ST", "SP", "SE", "CO":
	default:
		return false
	}
	lower := strings.ToLower(path)
	switch {
	case lower == "stval", strings.HasSuffix(lower, ".stval"):
		return true
	case lower == "q", strings.HasSuffix(lower, ".q"):
		return true
	case lower == "t", strings.HasSuffix(lower, ".t"):
		return true
	case lower == "mag.f", strings.HasSuffix(lower, ".mag.f"):
		return true
	case lower == "setmag.f", strings.HasSuffix(lower, ".setmag.f"):
		return true
	case strings.HasSuffix(lower, ".ctlval"):
		return true
	default:
		return false
	}
}

func cidDefaultSelected(path string, fc string) bool {
	lower := strings.ToLower(path)
	if lower == "q" || lower == "t" || strings.HasSuffix(lower, ".q") || strings.HasSuffix(lower, ".t") {
		return false
	}
	return strings.EqualFold(fc, "MX") || strings.EqualFold(fc, "ST") || strings.EqualFold(fc, "SP") || strings.EqualFold(fc, "SE") || strings.EqualFold(fc, "CO")
}

func cidDataType(path string, bType string) string {
	lowerPath := strings.ToLower(path)
	lowerType := strings.ToLower(bType)
	switch {
	case lowerPath == "q" || strings.HasSuffix(lowerPath, ".q"):
		return "quality"
	case lowerPath == "t" || strings.HasSuffix(lowerPath, ".t"):
		return "timestamp"
	case lowerPath == "stval" || strings.HasSuffix(lowerPath, ".stval"):
		if strings.Contains(lowerType, "bool") {
			return "bool"
		}
		return "auto"
	case lowerPath == "mag.f" || strings.HasSuffix(lowerPath, ".mag.f") || strings.HasSuffix(lowerPath, ".setmag.f"):
		return "float32"
	case strings.Contains(lowerType, "bool"):
		return "bool"
	case strings.Contains(lowerType, "float"):
		return "float32"
	case strings.Contains(lowerType, "int64"):
		return "int64"
	case strings.Contains(lowerType, "int32"):
		return "int32"
	default:
		return "auto"
	}
}

func cidPointName(lnDesc string, doDesc string, doName string, leafPath string) string {
	parts := []string{}
	if strings.TrimSpace(doDesc) != "" {
		parts = append(parts, strings.TrimSpace(doDesc))
	} else if strings.TrimSpace(doName) != "" {
		parts = append(parts, strings.TrimSpace(doName))
	}
	if strings.TrimSpace(leafPath) != "" {
		parts = append(parts, strings.TrimSpace(leafPath))
	}
	name := strings.Join(parts, " / ")
	if name == "" {
		name = strings.TrimSpace(lnDesc)
	}
	if name == "" {
		name = "IEC61850 point"
	}
	return name
}

func cidMetric(objectRef string) string {
	metric := strings.ToLower(objectRef)
	metric = strings.ReplaceAll(metric, "/", "_")
	metric = strings.ReplaceAll(metric, ".", "_")
	metric = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(metric, "_")
	metric = strings.Trim(metric, "_")
	if metric == "" {
		return "iec61850_point"
	}
	if metric[0] >= '0' && metric[0] <= '9' {
		return "iec61850_" + metric
	}
	return metric
}
