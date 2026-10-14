package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHistoryStoragePointOptionIncludesUnit(t *testing.T) {
	payload, err := json.Marshal(historyStoragePointOption{
		DeviceKey: "noise-meter-1",
		Metric:    "NOISE_DB",
		Name:      "噪声值",
		DataType:  "float64",
		Unit:      "dB",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"unit":"dB"`) {
		t.Fatalf("point option JSON = %s, want unit", payload)
	}
}
