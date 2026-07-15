package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/datamanager"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestAuthenticatedWebSocketRouteUpgrades(t *testing.T) {
	server := newAuthTestServer()
	login := httptest.NewRecorder()
	form := strings.NewReader("username=admin&password=123456&next=%2F")
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", form)
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	server.routes().ServeHTTP(login, loginRequest)
	if len(login.Result().Cookies()) != 1 {
		t.Fatalf("login cookies = %#v", login.Result().Cookies())
	}

	httpServer := httptest.NewServer(server.routes())
	defer httpServer.Close()
	header := http.Header{}
	header.Set("Cookie", login.Result().Cookies()[0].String())
	connection, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/api/ws", header)
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("websocket upgrade failed with status %d: %v", status, err)
	}
	defer connection.Close()
	var message datamanager.Message
	if err := connection.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	if message.Type != "snapshot" {
		t.Fatalf("initial message type = %q", message.Type)
	}
}

func TestWebSocketStreamsSnapshotAndPointDiff(t *testing.T) {
	store := state.New(config.Config{Points: []config.PointConfig{{DeviceKey: "d1", Metric: "p1"}}})
	server := New(config.ListenerConfig{}, store, nil, "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.data.Start(ctx)
	httpServer := httptest.NewServer(http.HandlerFunc(server.webSocket))
	defer httpServer.Close()

	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))

	seenSnapshot := false
	for index := 0; index < 4; index++ {
		var message datamanager.Message
		if err := connection.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message.Type == "snapshot" {
			seenSnapshot = true
		}
	}
	if !seenSnapshot {
		t.Fatal("websocket did not send initial snapshot")
	}

	store.SetPointValue("d1", "p1", 7)
	for {
		var message datamanager.Message
		if err := connection.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message.Type != "points.diff" {
			continue
		}
		var diff datamanager.PointDiff
		if err := json.Unmarshal(message.Payload, &diff); err != nil {
			t.Fatal(err)
		}
		if len(diff.Points) != 1 || diff.Points[0].Value != float64(7) {
			t.Fatalf("unexpected websocket point diff: %#v", diff)
		}
		break
	}
}

func TestWebSocketRejectsUnsupportedCommandType(t *testing.T) {
	store := state.New(config.Config{})
	server := New(config.ListenerConfig{}, store, nil, "", nil)
	httpServer := httptest.NewServer(http.HandlerFunc(server.webSocket))
	defer httpServer.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	for index := 0; index < 4; index++ {
		var ignored datamanager.Message
		if err := connection.ReadJSON(&ignored); err != nil {
			t.Fatal(err)
		}
	}
	if err := connection.WriteJSON(webSocketRequest{Type: "unknown", RequestID: "r1"}); err != nil {
		t.Fatal(err)
	}
	var response datamanager.Message
	if err := connection.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if response.Type != "command.result" {
		t.Fatalf("response type = %q", response.Type)
	}
}
