package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type opcuaBrowseRequest struct {
	Address   string `json:"address"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	NodeID    string `json:"nodeId"`
	Recursive bool   `json:"recursive"`
}

func (s *Server) browseOPCUA(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 64*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var body opcuaBrowseRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer cancel()
	point := config.PointConfig{
		Protocol: "opcua",
		Address:  body.Address,
		Username: body.Username,
		Password: body.Password,
	}
	var nodes []collector.OPCUABrowseNode
	if body.Recursive {
		nodes, err = collector.BrowseOPCUAVariableNodes(ctx, point, body.NodeID, 1000)
	} else {
		nodes, err = collector.BrowseOPCUANodes(ctx, point, body.NodeID)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "nodes": nodes})
}
