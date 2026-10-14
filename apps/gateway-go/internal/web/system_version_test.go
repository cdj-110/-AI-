package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSystemVersionIncludesApplicationAndOTARelease(t *testing.T) {
	originalPath := otaReleaseStatePath
	t.Cleanup(func() { otaReleaseStatePath = originalPath })
	otaReleaseStatePath = filepath.Join(t.TempDir(), "ota-release.json")
	if err := os.WriteFile(otaReleaseStatePath, []byte(`{"release":2026092101}`), 0600); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/system/version", nil)
	recorder := httptest.NewRecorder()
	new(Server).systemVersion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var body systemVersionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "V0.6" || body.Release != 2026092101 {
		t.Fatalf("response = %#v", body)
	}
}

func TestSystemVersionRejectsNonGET(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/system/version", nil)
	recorder := httptest.NewRecorder()
	new(Server).systemVersion(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", recorder.Code)
	}
}
