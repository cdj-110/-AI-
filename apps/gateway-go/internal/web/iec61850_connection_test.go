//go:build iec61850_mms && cgo

package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMMSTestConnectionRejectsTCPOnlyServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_ = connection.Close()
		}
	}()
	server := newAuthTestServer()
	request := httptest.NewRequest(http.MethodPost, "/api/connection/test", strings.NewReader(`{"protocol":"iec61850","address":"`+listener.Addr().String()+`"}`))
	authenticateTestRequest(server, request)
	result := httptest.NewRecorder()
	server.routes().ServeHTTP(result, request)
	if result.Code != http.StatusBadRequest || !strings.Contains(result.Body.String(), "MMS association") {
		t.Fatalf("TCP reachability was incorrectly accepted as MMS success: %d %s", result.Code, result.Body.String())
	}
}
