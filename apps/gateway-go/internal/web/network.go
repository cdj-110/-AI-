package web

import (
	"encoding/json"
	"net/http"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/networking"
)

func (s *Server) networkInterfaces(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	interfaces, err := networking.Interfaces()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"interfaces": interfaces})
}

func (s *Server) applyNetworkInterface(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut && request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var port config.NetworkPort
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&port); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	port.ApplyDefaults()
	if err := networking.ScheduleApply(port); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok":           true,
		"interface":    port.Interface,
		"mode":         port.Mode,
		"ipAddress":    port.IPAddress,
		"applyingInMs": 1000,
	})
}
