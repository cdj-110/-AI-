//go:build !iec61850_mms || !cgo

package web

import (
	"encoding/json"
	"net/http"
)

func (s *Server) registerOptionalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/iec61850/browse", s.requirePermission("collection.run", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": false, "message": "当前网关程序未包含 IEC 61850 MMS 功能，请升级支持 MMS 的固件后再扫描设备模型。"})
	}))
}
