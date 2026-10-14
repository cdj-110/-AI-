package remoteaccess

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestLocalClientPreservesLoginRedirectAndCookie(t *testing.T) {
	redirectFollowed := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/login" {
			http.SetCookie(writer, &http.Cookie{Name: "weikong_gateway_session_http", Value: "session-token", Path: "/", HttpOnly: true})
			http.Redirect(writer, request, "/", http.StatusSeeOther)
			return
		}
		redirectFollowed = true
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent := New(config.RemoteAccessConfig{LocalURL: server.URL}, "gw", "user", "secret")
	form := url.Values{"username": {"admin"}, "password": {"secret"}}
	response, err := agent.client.Post(server.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusSeeOther)
	}
	if response.Header.Get("Location") != "/" {
		t.Fatalf("location = %q", response.Header.Get("Location"))
	}
	if len(response.Cookies()) != 1 || response.Cookies()[0].Value != "session-token" {
		t.Fatalf("login cookie was not preserved: %#v", response.Cookies())
	}
	if redirectFollowed {
		t.Fatal("gateway followed the browser login redirect")
	}
}

func TestLocalTargetOnlyAllowsLoopback(t *testing.T) {
	agent := New(config.RemoteAccessConfig{LocalURL: "http://127.0.0.1:8088"}, "gw", "user", "secret")
	target, err := agent.localTarget("/api/status?full=true")
	if err != nil {
		t.Fatal(err)
	}
	if target != "http://127.0.0.1:8088/api/status?full=true" {
		t.Fatalf("target = %q", target)
	}

	agent.cfg.LocalURL = "http://192.168.1.10:8088"
	if _, err := agent.localTarget("/"); err == nil {
		t.Fatal("expected non-loopback target to be rejected")
	}
}

func TestResolvedLocalTargetFallsBackFromHTTPSToHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"healthy":true}`))
	}))
	defer server.Close()
	agent := New(config.RemoteAccessConfig{LocalURL: strings.Replace(server.URL, "http://", "https://", 1)}, "gw", "user", "secret")
	target, err := agent.resolvedLocalTarget(context.Background(), "/api/status?full=true")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(target, server.URL+"/api/status") {
		t.Fatalf("target = %q, want HTTP target under %q", target, server.URL)
	}
}

func TestResolvedLocalTargetFallsBackFromHTTPToHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"healthy":true}`))
	}))
	defer server.Close()
	agent := New(config.RemoteAccessConfig{
		LocalURL: strings.Replace(server.URL, "https://", "http://", 1),
	}, "gw", "user", "secret")
	target, err := agent.resolvedLocalTarget(context.Background(), "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(target, server.URL+"/api/status") {
		t.Fatalf("target = %q, want HTTPS target under %q", target, server.URL)
	}
}

func TestLocalTargetRejectsAbsoluteURL(t *testing.T) {
	agent := New(config.RemoteAccessConfig{LocalURL: "http://localhost:8088"}, "gw", "user", "secret")
	if _, err := agent.localTarget("http://example.com/"); err == nil {
		t.Fatal("expected absolute proxy target to be rejected")
	}
}

func TestBoundedTerminalSize(t *testing.T) {
	cols, rows := boundedTerminalSize(160, 48)
	if cols != 160 || rows != 48 {
		t.Fatalf("size = %dx%d, want 160x48", cols, rows)
	}
	cols, rows = boundedTerminalSize(10000, 0)
	if cols != 120 || rows != 32 {
		t.Fatalf("invalid size fallback = %dx%d, want 120x32", cols, rows)
	}
}

func TestCloudPingKeepsTunnelReadDeadlineAlive(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		for range 6 {
			time.Sleep(25 * time.Millisecond)
			if err := connection.WriteControl(websocket.PingMessage, []byte("keepalive"), time.Now().Add(time.Second)); err != nil {
				return
			}
		}
		if err := connection.WriteMessage(websocket.TextMessage, []byte(`{"type":"auth.ok"}`)); err != nil {
			return
		}
		// Keep the peer open while the client finishes processing the final
		// ping/pong control frame; production cloud connections remain open too.
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	endpoint := "ws" + strings.TrimPrefix(server.URL, "http")
	connection, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	refresh := keepCloudTunnelAlive(connection, 60*time.Millisecond)
	if err := refresh(); err != nil {
		t.Fatal(err)
	}
	_, payload, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("active tunnel expired while cloud pings were arriving: %v", err)
	}
	if string(payload) != `{"type":"auth.ok"}` {
		t.Fatalf("payload = %s", payload)
	}
}
