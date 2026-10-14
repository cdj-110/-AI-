package collector

import (
	"testing"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestDLT645SignedBCDBounds(t *testing.T) {
	for _, value := range []string{"900001", "800000", "-800000", "799999.9", "NaN", "+Inf", "1e9999999"} {
		if raw, err := encodeDLT645Value(value, "signed-bcd", 3, 0); err == nil {
			t.Fatalf("accepted %s: %X", value, raw)
		}
	}
	for _, value := range []float64{799999, -799999, 0, 123456} {
		raw, err := encodeDLT645Value(value, "signed-bcd", 3, 0)
		if err != nil {
			t.Fatal(err)
		}
		decoded, _, err := decodeDLT645Value(raw, "signed-bcd", 0)
		if err != nil || decoded != value {
			t.Fatalf("roundtrip %v: %v %v", value, decoded, err)
		}
	}
}

func TestDLT645DateTimeValidation(t *testing.T) {
	for _, item := range []struct {
		format string
		raw    []byte
		want   string
	}{
		{"datetime", []byte{0x56, 0x34, 0x12, 0x09, 0x09, 0x26}, "2026-09-09 12:34:56"},
		{"date", []byte{0x03, 0x09, 0x09, 0x26}, "2026-09-09"},
		{"date", []byte{0x29, 0x02, 0x24}, "2024-02-29"},
	} {
		got, _, err := decodeDLT645Value(item.raw, item.format, 0)
		if err != nil || got != item.want {
			t.Fatalf("%s: %v %v", item.format, got, err)
		}
	}
	for _, item := range []struct {
		format string
		raw    []byte
	}{
		{"date", []byte{0xFA, 0x99, 0x26}}, {"date", []byte{0x29, 0x02, 0x25}},
		{"time", []byte{0x60, 0, 0}}, {"datetime", []byte{0, 0, 0}}, {"time", []byte{0, 0, 0x24}},
	} {
		if got, _, err := decodeDLT645Value(item.raw, item.format, 0); err == nil {
			t.Fatalf("accepted invalid %s: %v", item.format, got)
		}
	}
}

func TestDLT645EnergyDictionaryMapping(t *testing.T) {
	for key, want := range map[string]string{"forward-active-total": "00010000", "reverse-active-total": "00020000", "forward-active-rate-4": "00010400", "reverse-active-rate-4": "00020400", "forward-reactive-total": "00030000", "reverse-reactive-total": "00040000"} {
		item, ok := findDLT645Item(key)
		if !ok || item.DI2007 != want {
			t.Fatalf("%s: %s want %s", key, item.DI2007, want)
		}
	}
}

func TestDLT645SavedDIIsNotSilentlyRetargeted(t *testing.T) {
	point := config.PointConfig{DLT645: &config.DLT645Config{ItemKey: "forward-active-total", DataIdentifier: "00000000"}}
	if _, _, err := resolveDLT645Descriptor(point, "2007"); err == nil {
		t.Fatal("stale explicit DI was silently replaced")
	}
	point.DLT645.ItemKey = ""
	if _, _, err := resolveDLT645Descriptor(point, "2007"); err != nil {
		t.Fatal("manual DI must be preserved", err)
	}
}
