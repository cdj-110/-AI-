package web

import (
	"encoding/json"
	"net/http"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/networking"
)

type wifiBootRequest struct {
	Enabled bool `json:"enabled"`
}

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
		status := networking.WiFi(cfg.WiFi)
		if status.Config.Password != "" {
			status.Config.Password = "***"
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(status)
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
		previousWiFi := current.WiFi
		previousBootEnabled := networking.WirelessBootEnabled()
		runtimeAvailable := networking.WiFi(wifi).Available
		if !runtimeAvailable {
			if err := networking.PrepareWiFiConfiguration(wifi); err != nil {
				http.Error(writer, "prepare WiFi configuration failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if err := networking.SetWirelessBootEnabled(wifi.Enabled); err != nil {
			if !runtimeAvailable {
				_ = networking.PrepareWiFiConfiguration(previousWiFi)
			}
			http.Error(writer, "save WiFi boot state failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		current.WiFi = wifi
		if err := s.saveAndApplyConfig(current); err != nil {
			if !runtimeAvailable {
				_ = networking.PrepareWiFiConfiguration(previousWiFi)
			}
			_ = networking.SetWirelessBootEnabled(previousBootEnabled)
			http.Error(writer, "apply WiFi configuration failed; previous configuration was restored: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		if !runtimeAvailable {
			message := "WiFi 配置已保存，重启网关后启用并生效。"
			if !wifi.Enabled {
				message = "WiFi 已设置为停用，重启网关后生效。"
			}
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{
				"ok": true, "connected": false, "interface": wifi.Interface,
				"restartRequired": true, "message": message,
			})
			return
		}
		if err := networking.ApplyWiFiNow(wifi); err != nil {
			rollback := current
			rollback.WiFi = previousWiFi
			_ = s.saveAndApplyConfig(rollback)
			_ = networking.SetWirelessBootEnabled(previousBootEnabled)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{
				"ok": false, "connected": false, "interface": wifi.Interface,
				"restartRequired": false, "message": "WiFi 连接失败：" + err.Error(),
			})
			return
		}
		applied := networking.WiFi(wifi)
		message := "WiFi 已停用。"
		if wifi.Enabled {
			message = "WiFi 连接成功，当前 IP：" + applied.IPv4
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{
			"ok": true, "connected": applied.Connected, "interface": wifi.Interface,
			"ipv4": applied.IPv4, "restartRequired": false, "message": message,
		})
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

func (s *Server) wifiBoot(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body wifiBootRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	current := s.runtime.Config()
	previousBootEnabled := networking.WirelessBootEnabled()
	if err := networking.SetWirelessBootEnabled(body.Enabled); err != nil {
		http.Error(writer, "save WiFi boot state failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	current.WiFi.Enabled = body.Enabled
	if err := s.saveAndApplyConfig(current); err != nil {
		_ = networking.SetWirelessBootEnabled(previousBootEnabled)
		http.Error(writer, "save WiFi state failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	message := "WiFi 已设置为停用，重启网关后生效。"
	if body.Enabled {
		message = "WiFi 已设置为启用，重启网关后即可扫描和连接。"
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{
		"ok": true, "restartRequired": true,
		"enabled": body.Enabled, "message": message,
	})
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
		if err := s.saveAndApplyConfig(current); err != nil {
			http.Error(writer, "apply cellular configuration failed; previous configuration was restored: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(writer).Encode(networking.ApplyCellular(cellular))
		return
	}
	_ = json.NewEncoder(writer).Encode(networking.Cellular(s.runtime.Config().Cellular))
}
