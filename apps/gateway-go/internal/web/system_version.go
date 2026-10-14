package web

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"

	"weikong-iot-platform/apps/gateway-go/internal/buildmeta"
)

var otaReleaseStatePath = "/userdata/weikong/config/ota-release.json"

type systemVersionResponse struct {
	Version   string `json:"version"`
	Release   int64  `json:"release,omitempty"`
	BuildTime string `json:"buildTime,omitempty"`
	Commit    string `json:"commit,omitempty"`
	GoVersion string `json:"goVersion"`
}

func (s *Server) systemVersion(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	response := systemVersionResponse{
		Version:   buildmeta.DisplayVersion(),
		BuildTime: buildmeta.BuildTime,
		Commit:    buildmeta.Commit,
		GoVersion: runtime.Version(),
	}
	var state struct {
		Release int64 `json:"release"`
	}
	if raw, err := os.ReadFile(otaReleaseStatePath); err == nil && json.Unmarshal(raw, &state) == nil && state.Release > 0 {
		response.Release = state.Release
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(response)
}
