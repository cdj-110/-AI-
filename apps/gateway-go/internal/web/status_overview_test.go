package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestCollectionOverviewPermissions(t *testing.T) {
	for _, permission := range []string{"status.view", "config.view", "history.view"} {
		s := &Server{store: state.New(config.Config{})}
		r := httptest.NewRequest(http.MethodGet, "/api/status/overview", nil)
		authorizeConfigTestRequest(s, r, permission)
		w := httptest.NewRecorder()
		s.collectionOverview(w, r)
		want := http.StatusOK
		if permission == "history.view" {
			want = http.StatusForbidden
		}
		if w.Code != want {
			t.Fatalf("%s: %d", permission, w.Code)
		}
		if want == http.StatusOK && !strings.Contains(w.Body.String(), `"issues":[]`) {
			t.Fatal(w.Body.String())
		}
	}
	s := &Server{}
	r := httptest.NewRequest(http.MethodPost, "/api/status/overview", nil)
	w := httptest.NewRecorder()
	s.collectionOverview(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal(w.Code)
	}
}
