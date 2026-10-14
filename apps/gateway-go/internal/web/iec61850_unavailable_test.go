//go:build !iec61850_mms || !cgo

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnavailableMMSReportsCapabilityInsteadOf404OrTCPSuccess(t *testing.T) {
	for _, path := range []string{"/api/iec61850/browse", "/api/connection/test"} {
		t.Run(path, func(t *testing.T) {
			server := newAuthTestServer()
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"protocol":"iec61850","address":"127.0.0.1:102"}`))
			authenticateTestRequest(server, request)
			response := httptest.NewRecorder()
			server.routes().ServeHTTP(response, request)
			var body struct {
				OK      bool   `json:"ok"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusNotImplemented || body.OK || !strings.Contains(body.Message, "未包含 IEC 61850 MMS") {
				t.Fatalf("unexpected capability response: %d %s", response.Code, response.Body.String())
			}
			unauthenticated := httptest.NewRecorder()
			server.routes().ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, path, nil))
			if unauthenticated.Code != http.StatusUnauthorized {
				t.Fatal("MMS endpoint bypassed authentication")
			}
		})
	}
}
