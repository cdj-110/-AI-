package cloud

import (
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/model"
)

func TestNormalizeTelemetryNumbers(t *testing.T) {
	for _, test := range []struct{ input, want float64 }{
		{42.900000000000006, 42.9}, {44.300000000000004, 44.3},
		{45.150000000000006, 45.15}, {0.30000000000000004, 0.3},
		{-42.900000000000006, -42.9}, {0.9999999999999999, 1},
		{1.2345678901234567, 1.2345678901234567},
		{0.3333333333333333, 0.3333333333333333}, {1.23456789, 1.23456789},
		{1e-20, 1e-20}, {math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64},
		{9007199254740992, 9007199254740992}, {0, 0},
	} {
		if got := normalizeTelemetryNumber(test.input); got != test.want {
			t.Fatalf("normalize(%0.17g)=%0.17g, want %0.17g", test.input, got, test.want)
		}
	}
	if math.Signbit(normalizeTelemetryNumber(math.Copysign(0, -1))) {
		t.Fatal("negative zero")
	}
	if !math.IsNaN(normalizeTelemetryNumber(math.NaN())) {
		t.Fatal("NaN changed")
	}
	if !math.IsInf(normalizeTelemetryNumber(math.Inf(1)), 1) {
		t.Fatal("infinity changed")
	}
}

func TestMQTTReportNumbersAcrossAllPayloadModes(t *testing.T) {
	for _, mode := range []string{"cloud", "attributes", "grouped", "custom"} {
		t.Run(mode, func(t *testing.T) {
			transport := &commandTestMQTT{published: make(chan commandTestPublish, 1)}
			client := &Client{gatewayKey: "GW-A", mqtt: transport, payloadMode: mode, payloadTemplate: `{"metrics":{attributes},"devices":{devices},"selected":{attribute.child.mbrtu_P01}}`}
			nested := map[string]interface{}{"array": []float64{42.900000000000006, 1.2345678901234567}}
			metrics := map[string]interface{}{"mbrtu_P01": 42.900000000000006, "mbtcp_P01": false, "zero": 0, "one": 1, "precise": 1.2345678901234567, "large": uint64(18446744073709551615), "text": "42.900000000000006", "bool": true, "nested": nested}
			grouped := map[string]map[string]interface{}{"child": metrics}
			var err error
			if mode == "cloud" {
				err = client.PublishTelemetry(model.Reading{DeviceKey: "child", Time: time.Now(), Metrics: metrics})
			} else if mode == "attributes" {
				err = client.PublishAttributes(metrics)
			} else {
				err = client.PublishManual(grouped)
			}
			if err != nil {
				t.Fatal(err)
			}
			publication := <-transport.published
			raw := string(publication.payload.([]byte))
			if !strings.Contains(raw, `"mbrtu_P01":42.9`) || !strings.Contains(raw, `"mbtcp_P01":false`) || !strings.Contains(raw, `"zero":0`) || !strings.Contains(raw, `"one":1`) || !strings.Contains(raw, `"precise":1.2345678901234567`) {
				t.Fatal(raw)
			}
			if !strings.Contains(raw, `"text":"42.900000000000006"`) || !strings.Contains(raw, `"bool":true`) || strings.Contains(raw, `"mbrtu_P01":"`) {
				t.Fatal("value types changed: ", raw)
			}
			if mode != "custom" && !strings.Contains(raw, `"large":18446744073709551615`) {
				t.Fatal("integer precision lost: ", raw)
			}
			if mode == "custom" && !strings.Contains(raw, `"selected":42.9`) {
				t.Fatal("point template not normalized: ", raw)
			}
			if !strings.Contains(raw, `"array":[42.9,1.2345678901234567]`) {
				t.Fatal("nested readings not normalized: ", raw)
			}
			if metrics["mbrtu_P01"] != 42.900000000000006 || nested["array"].([]float64)[0] != 42.900000000000006 {
				t.Fatal("original reading mutated")
			}
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				t.Fatal(err)
			}
			if mode == "cloud" && (publication.qos != 1 || publication.topic != "weikong/gateways/GW-A/children/child/telemetry") {
				t.Fatal(publication)
			}
			if mode != "cloud" && publication.qos != 0 {
				t.Fatal("manual QoS changed")
			}
			if publication.retained {
				t.Fatal("report unexpectedly retained")
			}
		})
	}
}

func TestTelemetryNormalizationBoundsAndIdempotency(t *testing.T) {
	random := rand.New(rand.NewSource(20260918))
	for index := 0; index < 20000; index++ {
		value := math.Float64frombits(random.Uint64())
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		got := normalizeTelemetryNumber(value)
		step := math.Max(math.Abs(math.Nextafter(value, math.Inf(1))-value), math.Abs(math.Nextafter(value, math.Inf(-1))-value))
		if math.Abs(got-value) > 2*step || normalizeTelemetryNumber(got) != got {
			t.Fatalf("unbounded or non-idempotent normalization: %.17g -> %.17g", value, got)
		}
	}
}

func TestTelemetryNestedTypesAndNilPreserved(t *testing.T) {
	var nilMetrics map[string]interface{}
	var nilValues []interface{}
	input := map[string]interface{}{"": true, "items": []interface{}{44.300000000000004, uint64(18446744073709551615), false, "44.300000000000004"}, "nilMap": nilMetrics, "nilSlice": nilValues, "float32": float32(1.2)}
	output := normalizeTelemetryValue(input).(map[string]interface{})
	items := output["items"].([]interface{})
	if items[0] != float64(44.3) || items[1] != uint64(18446744073709551615) || items[2] != false || items[3] != "44.300000000000004" || output[""] != true || output["float32"] != float32(1.2) {
		t.Fatal(output)
	}
	if output["nilMap"].(map[string]interface{}) != nil || output["nilSlice"].([]interface{}) != nil || normalizeTelemetryGroups(nil) != nil {
		t.Fatal("nil values changed")
	}
	items[1] = 0
	if input["items"].([]interface{})[1] != uint64(18446744073709551615) {
		t.Fatal("shared nested slice mutated")
	}
}
