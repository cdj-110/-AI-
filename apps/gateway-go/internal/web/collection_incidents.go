package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collectdiag"
)

func (s *Server) collectionIncidentHistory(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.collectionIncidents == nil {
		http.Error(writer, "collection incident database is unavailable", http.StatusServiceUnavailable)
		return
	}
	query := collectdiag.Query{
		Page:     boundedIncidentQueryInt(request, "page", 1, 1, 1_000_000),
		PageSize: boundedIncidentQueryInt(request, "pageSize", 30, 1, 200),
		State:    strings.TrimSpace(request.URL.Query().Get("state")),
		Protocol: strings.TrimSpace(request.URL.Query().Get("protocol")),
		Keyword:  strings.TrimSpace(request.URL.Query().Get("keyword")),
	}
	flushCtx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := s.collectionIncidents.Flush(flushCtx); err != nil {
		http.Error(writer, err.Error(), http.StatusServiceUnavailable)
		return
	}
	page, err := s.collectionIncidents.Query(query)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(page)
}

func (s *Server) clearCollectionIncidents(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodDelete {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.collectionIncidents == nil {
		http.Error(writer, "collection incident database is unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	if err := s.collectionIncidents.Clear(ctx); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]interface{}{"ok": true, "message": "采集异常记录已清空"})
}

func boundedIncidentQueryInt(request *http.Request, key string, fallback, min, max int) int {
	value, err := strconv.Atoi(request.URL.Query().Get(key))
	if err != nil || value < min {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}
