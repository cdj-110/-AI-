package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collectdiag"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestCollectionIncidentHistoryReturnsPersistentFailures(t *testing.T) {
	incidents, err := collectdiag.Open(filepath.Join(t.TempDir(), "config.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer incidents.Close()
	incidents.RecordPointIncident(state.PointIncidentEvent{Active: true, DeviceKey: "d1", PointName: "温度", Metric: "p1", Protocol: "modbus-tcp", Address: "40001", Reason: "i/o timeout", At: time.Now()})

	server := newAuthTestServer()
	server.SetCollectionIncidents(incidents)
	request := httptest.NewRequest(http.MethodGet, "/api/collection-incidents?page=1&pageSize=20&state=ACTIVE", nil)
	authenticateTestRequest(server, request)
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	var page collectdiag.Page
	if err = json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].Metric != "p1" || page.Items[0].Explanation == "" {
		t.Fatalf("unexpected page: %#v", page)
	}

	clearRequest := httptest.NewRequest(http.MethodDelete, "/api/collection-incidents/clear", nil)
	authenticateTestRequest(server, clearRequest)
	clearResponse := httptest.NewRecorder()
	server.routes().ServeHTTP(clearResponse, clearRequest)
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear response = %d: %s", clearResponse.Code, clearResponse.Body.String())
	}
	page, err = incidents.Query(collectdiag.Query{Page: 1, PageSize: 20})
	if err != nil || page.Total != 0 {
		t.Fatalf("incidents not cleared: page=%#v err=%v", page, err)
	}
}
