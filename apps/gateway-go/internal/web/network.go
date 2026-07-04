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

func (s *Server) wifiNetwork(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		cfg := s.runtime.Config()
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(networking.WiFi(cfg.WiFi))
	case http.MethodPut, http.MethodPost:
		var wifi config.WiFiConfig
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&wifi); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		wifi.ApplyDefaults()
		current := s.runtime.Config()
		if wifi.Password == "***" {
			wifi.Password = current.WiFi.Password
		}
		if err := networking.ValidateWiFi(wifi); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		current.WiFi = wifi
		if err := config.Save(s.configPath, current); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if s.onConfig != nil {
			s.onConfig(current)
		} else {
			s.runtime.UpdateConfig(current)
		}
		if err := networking.ScheduleApplyWiFi(wifi); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "interface": wifi.Interface, "applyingInMs": 1000})
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) scanWiFiNetwork(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	iface := request.URL.Query().Get("interface")
	if iface == "" {
		iface = s.runtime.Config().WiFi.Interface
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(networking.ScanWiFi(iface))
}

func (s *Server) cellularNetwork(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodPut {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if request.Method == http.MethodPost {
		_ = json.NewEncoder(writer).Encode(networking.RedialCellular(s.runtime.Config().Cellular))
		return
	}
	if request.Method == http.MethodPut {
		var cellular config.CellularConfig
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cellular); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		cellular.ApplyDefaults()
		current := s.runtime.Config()
		current.Cellular = cellular
		if err := config.Save(s.configPath, current); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		if s.onConfig != nil {
			s.onConfig(current)
		} else {
			s.runtime.UpdateConfig(current)
		}
		_ = json.NewEncoder(writer).Encode(networking.ApplyCellular(cellular))
		return
	}
	_ = json.NewEncoder(writer).Encode(networking.Cellular(s.runtime.Config().Cellular))
}
