package goose

import (
	"net"
	"testing"
	"time"
)

func TestEthernetMetadataAndSniffDiagnosis(t *testing.T) {
	frame, err := EncodeFrame(Message{
		AppID:             0x1000,
		GoCBRef:           "IEDLD1/LLN0$GO$gcb01",
		DataSetRef:        "IEDLD1/LLN0$Dataset01",
		TimeAllowedToLive: 2000,
		Values:            []Value{{Kind: "bool", Value: true}},
	}, mustMAC(t, "02:00:00:00:00:01"), mustMAC(t, "01:0c:cd:01:00:01"), 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	source, destination, tagged, vlanID, priority, ok := ethernetMetadata(frame)
	if !ok || source != "02:00:00:00:00:01" || destination != "01:0c:cd:01:00:01" || !tagged || vlanID != 12 || priority != 4 {
		t.Fatalf("unexpected metadata: %s %s %v %d %d", source, destination, tagged, vlanID, priority)
	}
	now := time.Now()
	item := SniffSource{
		DestinationMAC: destination,
		VLANID:         vlanID,
		AppID:          0x1000,
		GoCBRef:        "IEDLD1/LLN0$GO$gcb01",
		DataSetRef:     "IEDLD1/LLN0$Dataset01",
		LastSeen:       now,
	}
	warnings, matched := diagnoseSniffSource(item, SniffExpected{
		DeviceKey:      "device-001",
		DestinationMAC: destination,
		VLANID:         vlanID,
		AppID:          item.AppID,
		GoCBRef:        item.GoCBRef,
		DataSetRef:     item.DataSetRef,
	}, now)
	if !matched || len(warnings) != 0 {
		t.Fatalf("expected matching source, got matched=%v warnings=%v", matched, warnings)
	}
}

func mustMAC(t *testing.T, raw string) net.HardwareAddr {
	t.Helper()
	value, err := net.ParseMAC(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
