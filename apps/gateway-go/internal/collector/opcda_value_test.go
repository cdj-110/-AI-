package collector

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestIsOPCDAVariantDecodeError(t *testing.T) {
	if !isOPCDAVariantDecodeError(errors.New(
		"OPC DA read failed: code=162 detail=[OPC] decode IOPCSyncIO::Read response failed: SyncRead item 4: invalid SAFEARRAY",
	)) {
		t.Fatal("VARIANT decode failure must be isolated to the offending item")
	}
	if isOPCDAVariantDecodeError(errors.New("OPC DA IOPCSyncIO::Read exchange failed: connection reset")) {
		t.Fatal("transport failure must rebuild the session instead of being treated as an item codec failure")
	}
}

func TestOPCDABrowseSessionContextIsUnique(t *testing.T) {
	firstContext, firstID := withOPCDABrowseSession(context.Background())
	secondContext, secondID := withOPCDABrowseSession(context.Background())
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatalf("browse session IDs must be non-empty and unique: %q %q", firstID, secondID)
	}
	if got := opcdaBrowseSessionID(firstContext); got != firstID {
		t.Fatalf("first context session ID = %q, want %q", got, firstID)
	}
	if got := opcdaBrowseSessionID(secondContext); got != secondID {
		t.Fatalf("second context session ID = %q, want %q", got, secondID)
	}
}

func TestNormalizeOPCDAWriteValueUsesCanonicalVariantType(t *testing.T) {
	tests := []struct {
		name, raw, target string
		want              interface{}
	}{
		{name: "bool", raw: "true", target: "bool", want: true},
		{name: "int16 minimum", raw: "-32768", target: "int16", want: int64(-32768)},
		{name: "uint16 maximum", raw: "65535", target: "uint16", want: uint64(65535)},
		{name: "uint32 maximum", raw: "4294967295", target: "uint32", want: uint64(4294967295)},
		{name: "uint64 exact", raw: "18446744073709551615", target: "uint64", want: uint64(18446744073709551615)},
		{name: "float64", raw: "123.25", target: "float64", want: float64(123.25)},
		{name: "unicode", raw: "中文测试", target: "string", want: "中文测试"},
		{name: "empty string", raw: "", target: "string", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeOPCDAWriteValue(test.raw, test.target)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("value = %#v (%T), want %#v (%T)", got, got, test.want, test.want)
			}
		})
	}
}

func TestNormalizeOPCDAWriteValueAcceptsJSONNumberWithoutPrecisionLoss(t *testing.T) {
	got, err := normalizeOPCDAWriteValue(json.Number("4294967295"), "uint32")
	if err != nil {
		t.Fatal(err)
	}
	if got != uint64(4294967295) {
		t.Fatalf("value = %#v (%T), want uint64(4294967295)", got, got)
	}
}

func TestNormalizeOPCDAWriteValueRejectsOverflow(t *testing.T) {
	if _, err := normalizeOPCDAWriteValue("65536", "uint16"); err == nil {
		t.Fatal("uint16 overflow must be rejected")
	}
}

func TestApplyOPCDAScale(t *testing.T) {
	point := config.PointConfig{Scale: 0.1, Offset: 2, Decimals: 2}
	if got := applyOPCDAScale(point, int64(123)); got != float64(14.3) {
		t.Fatalf("scaled value = %#v, want 14.3", got)
	}
	if got := applyOPCDAScale(point, "text"); got != "text" {
		t.Fatalf("non numeric value = %#v, want text", got)
	}
	if got := applyOPCDAScale(point, []float64{10, -5}); !reflect.DeepEqual(got, []float64{3, 1.5}) {
		t.Fatalf("scaled SAFEARRAY = %#v, want [3 1.5]", got)
	}
}

func TestOPCDASubscriptionRetryDelayIsBounded(t *testing.T) {
	want := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, time.Minute, time.Minute}
	for index, expected := range want {
		if got := opcdaSubscriptionRetryDelay(index + 1); got != expected {
			t.Fatalf("failure %d delay = %s, want %s", index+1, got, expected)
		}
	}
}

func TestOPCDACanonicalDataType(t *testing.T) {
	cases := map[uint16]string{5: "float64", 11: "bool", 19: "uint32", 0x2000 | 18: "uint16[]"}
	for value, want := range cases {
		if got := opcdaCanonicalDataType(value); got != want {
			t.Fatalf("type %#x = %q, want %q", value, got, want)
		}
	}
}

func TestOPCDANativeArrayTypeNames(t *testing.T) {
	if got := opcdaNativeValueTypeName(7); got != "float64[]" {
		t.Fatalf("native type 7 = %q, want float64[]", got)
	}
	if got := opcdaNativeValueTypeName(8); got != "string[]" {
		t.Fatalf("native type 8 = %q, want string[]", got)
	}
}

func TestOPCDAAccessNameUsesConfigEnumAndSafeDefault(t *testing.T) {
	cases := map[uint32]string{
		0: "read",
		1: "read",
		2: "write",
		3: "read_write",
	}
	for rights, want := range cases {
		if got := opcdaAccessName(rights); got != want {
			t.Fatalf("access rights %d = %q, want %q", rights, got, want)
		}
	}
}

func TestOPCDASessionLayoutsIgnoreReadMode(t *testing.T) {
	previous := []config.PointConfig{{
		DeviceKey: "opc", Protocol: "opcda", Address: "192.0.2.1",
		CLSID: "clsid", GroupName: "group", RequestedUpdateRateMS: 1000,
		ReadMode: "sync", ItemID: "Random.Real8", RequestedDataType: "auto",
	}}
	next := append([]config.PointConfig(nil), previous...)
	next[0].ReadMode = "subscription"
	if !OPCDASessionLayoutsEqual(previous, next) {
		t.Fatal("read mode-only change must preserve the OPC DA session")
	}
}

func TestOPCDASessionResetDevicesTargetsReadModeChange(t *testing.T) {
	previous := []config.PointConfig{
		{DeviceKey: "opc-a", Protocol: "opcda", Address: "192.0.2.1", CLSID: "a", GroupName: "group-a", ReadMode: "sync", ItemID: "Random.Real8", RequestedDataType: "auto", Scale: 1},
		{DeviceKey: "opc-b", Protocol: "opcda", Address: "192.0.2.2", CLSID: "b", GroupName: "group-b", ReadMode: "sync", ItemID: "Random.Int1", RequestedDataType: "auto", Scale: 1},
	}
	next := append([]config.PointConfig(nil), previous...)
	next[0].ReadMode = "subscription"
	next[1].Scale = 100
	if got := OPCDASessionResetDevices(previous, next); !reflect.DeepEqual(got, []string{"opc-a"}) {
		t.Fatalf("reset devices = %v, want [opc-a]", got)
	}
}

func TestOPCDASessionResetDevicesTargetsOnlyNativeLayoutChange(t *testing.T) {
	previous := []config.PointConfig{
		{DeviceKey: "opc-a", Protocol: "opcda", Address: "192.0.2.1", CLSID: "a", GroupName: "group-a", ReadMode: "subscription", ItemID: "Random.Real8", RequestedDataType: "auto"},
		{DeviceKey: "opc-b", Protocol: "opcda", Address: "192.0.2.2", CLSID: "b", GroupName: "group-b", ReadMode: "subscription", ItemID: "Random.Int1", RequestedDataType: "auto"},
	}
	next := append([]config.PointConfig(nil), previous...)
	next[1].GroupName = "group-b-new"
	if got := OPCDASessionResetDevices(previous, next); !reflect.DeepEqual(got, []string{"opc-b"}) {
		t.Fatalf("reset devices = %v, want [opc-b]", got)
	}
}

func TestOPCDASessionLayoutsResetForNativeChanges(t *testing.T) {
	base := []config.PointConfig{{
		DeviceKey: "opc", Protocol: "opcda", Address: "192.0.2.1",
		CLSID: "clsid", GroupName: "group", RequestedUpdateRateMS: 1000,
		ReadMode: "sync", ItemID: "Random.Real8", RequestedDataType: "auto",
	}}
	cases := map[string]func(*config.PointConfig){
		"update rate": func(point *config.PointConfig) { point.RequestedUpdateRateMS = 10000 },
		"group":       func(point *config.PointConfig) { point.GroupName = "other" },
		"item":        func(point *config.PointConfig) { point.ItemID = "Random.Int1" },
	}
	for name, change := range cases {
		next := append([]config.PointConfig(nil), base...)
		change(&next[0])
		if OPCDASessionLayoutsEqual(base, next) {
			t.Fatalf("%s change must reset the OPC DA session", name)
		}
	}
}
