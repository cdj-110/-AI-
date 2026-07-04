//go:build iec61850_mms && cgo

package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
)

type iec61850BrowseRequest struct {
	Address   string `json:"address"`
	ParentRef string `json:"parentRef"`
	Recursive bool   `json:"recursive"`
	IEDName   string `json:"iedName"`
}

func (s *Server) browseIEC61850(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 64*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var body iec61850BrowseRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 20*time.Second)
	defer cancel()
	nodes, err := collector.BrowseIEC61850Nodes(ctx, body.Address, body.ParentRef, body.Recursive)
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	iedName := strings.TrimSpace(body.IEDName)
	if iec61850LooksLikeObjectRef(iedName) {
		iedName = strings.TrimSpace(strings.SplitN(iedName, "/", 2)[0])
	}
	if iedName != "" {
		probeCtx, probeCancel := context.WithTimeout(request.Context(), 25*time.Second)
		defer probeCancel()
		if probedNodes, probeErr := collector.DiscoverIEC61850TemplateNodes(probeCtx, body.Address, iedName); probeErr == nil {
			nodes = collector.MergeIEC61850BrowseNodes(nodes, probedNodes)
		}
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "nodes": nodes})
}

func iec61850LooksLikeObjectRef(value string) bool {
	return strings.Contains(value, "/")
}
