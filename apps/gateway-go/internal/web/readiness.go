package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type readinessResponse struct {
	Ready  bool   `json:"ready"`
	BootID string `json:"bootId"`
	Phase  string `json:"phase"`
}

func newBootID() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err == nil {
		return hex.EncodeToString(raw)
	}
	return fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
}

// readiness deliberately exposes only a random process generation and a coarse
// phase. It is safe to poll after a restart when the previous login session has
// disappeared, without disclosing configuration or point details.
func (s *Server) readiness(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result := readinessResponse{BootID: strings.TrimSpace(s.bootID), Phase: "starting"}
	if result.BootID == "" {
		result.BootID = newBootID()
		s.bootID = result.BootID
	}
	if s.store != nil {
		health := s.store.Health(20 * time.Second)
		// Readiness follows the application's progress heartbeat rather than the
		// availability of field devices. A disconnected PLC or OPC server must
		// not leave the maintenance page waiting forever after a healthy restart.
		result.Ready = health.Healthy
		if result.Ready {
			result.Phase = "ready"
		} else if !health.Healthy {
			result.Phase = "unhealthy"
		} else {
			result.Phase = "initializing_collection"
		}
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if !result.Ready {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(writer).Encode(result)
}
