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

type opcdaBrowseRequest struct {
	connectionTestRequest
	BranchItemID string `json:"branchItemId"`
	Recursive    bool   `json:"recursive"`
	Limit        int    `json:"limit"`
}

func (s *Server) browseOPCDA(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 128*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var body opcdaBrowseRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Password == "***" {
		if s.runtime == nil || !resolveMaskedOPCDAPassword(&body.connectionTestRequest, s.runtime.Config()) {
			http.Error(writer, "无法从当前设备配置解析已隐藏的 OPC DA 密码", http.StatusBadRequest)
			return
		}
	}
	limit := body.Limit
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	point := config.PointConfig{
		DeviceKey: body.DeviceKey, Protocol: "opcda", Address: body.Address,
		ProgID: body.ProgID, CLSID: body.CLSID, Domain: body.Domain,
		Workstation: body.Workstation, Username: body.Username, Password: body.Password,
		SecurityMode: body.SecurityMode, Authentication: body.Authentication,
		AuthenticationLevel:        body.AuthenticationLevel,
		ConnectTimeoutMilliseconds: body.ConnectTimeoutMilliseconds,
		RequestTimeoutMilliseconds: body.RequestTimeoutMilliseconds,
		GroupName:                  body.GroupName, RequestedUpdateRateMS: body.RequestedUpdateRateMS,
		Deadband: body.Deadband, LocaleID: body.LocaleID, LocalBindIP: body.LocalBindIP,
	}
	point.ApplyDefaults()
	timeout := time.Duration(point.ConnectTimeoutMilliseconds+point.RequestTimeoutMilliseconds)*time.Millisecond + 10*time.Second
	if body.Recursive {
		timeout += 30 * time.Second
	}
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	if timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	defer cancel()
	var nodes []collector.OPCDABrowseNode
	if body.Recursive {
		nodes, err = collector.BrowseOPCDALeaves(ctx, point, body.BranchItemID, limit)
	} else {
		nodes, err = collector.BrowseOPCDANodes(ctx, point, body.BranchItemID, limit)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok": false, "message": err.Error(),
		})
		return
	}
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok": true, "nodes": nodes, "count": len(nodes),
	})
}
