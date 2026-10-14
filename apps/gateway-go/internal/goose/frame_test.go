package goose

import (
	"math"
	"net"
	"testing"
	"time"
)

func TestEncodeDecodeFrameRoundTrip(t *testing.T) {
	source, _ := net.ParseMAC("02:00:00:00:00:01")
	destination, _ := net.ParseMAC("01:0c:cd:01:00:01")
	now := time.Unix(1785384000, 250_000_000)
	message := Message{
		AppID:             0x1001,
		GoCBRef:           "WEIKONGLD1/LLN0$GO$gcb01",
		TimeAllowedToLive: 2000,
		DataSetRef:        "WEIKONGLD1/LLN0$Dataset01",
		GoID:              "goose-device-1",
		Timestamp:         now,
		StateNumber:       7,
		SequenceNumber:    13,
		ConfRev:           2,
		Values: []Value{
			{Kind: "bool", Value: true},
			{Kind: "int16", Value: int16(-12)},
			{Kind: "uint32", Value: uint32(65539)},
			{Kind: "float32", Value: float32(12.5)},
			{Kind: "float64", Value: 98.25},
			{Kind: "string", Value: "running"},
		},
	}
	frame, err := EncodeFrame(message, source, destination, 100, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) < 18 || frame[12] != 0x81 || frame[13] != 0x00 {
		t.Fatalf("expected an IEEE 802.1Q frame, got %x", frame[:14])
	}
	decoded, err := DecodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.AppID != message.AppID || decoded.GoCBRef != message.GoCBRef || decoded.DataSetRef != message.DataSetRef {
		t.Fatalf("metadata mismatch: %#v", decoded)
	}
	if decoded.StateNumber != 7 || decoded.SequenceNumber != 13 || decoded.ConfRev != 2 {
		t.Fatalf("state mismatch: %#v", decoded)
	}
	if len(decoded.Values) != len(message.Values) {
		t.Fatalf("decoded %d values, want %d", len(decoded.Values), len(message.Values))
	}
	if value, ok := decoded.Values[0].Value.(bool); !ok || !value {
		t.Fatalf("bool value = %#v", decoded.Values[0])
	}
	if value, ok := decoded.Values[1].Value.(int64); !ok || value != -12 {
		t.Fatalf("int value = %#v", decoded.Values[1])
	}
	if value, ok := decoded.Values[2].Value.(uint64); !ok || value != 65539 {
		t.Fatalf("uint value = %#v", decoded.Values[2])
	}
	if value, ok := decoded.Values[3].Value.(float64); !ok || math.Abs(value-12.5) > 0.0001 {
		t.Fatalf("float32 value = %#v", decoded.Values[3])
	}
	if value, ok := decoded.Values[4].Value.(float64); !ok || math.Abs(value-98.25) > 0.0001 {
		t.Fatalf("float64 value = %#v", decoded.Values[4])
	}
	if value, ok := decoded.Values[5].Value.(string); !ok || value != "running" {
		t.Fatalf("string value = %#v", decoded.Values[5])
	}
	if delta := decoded.Timestamp.Sub(now); delta < -time.Microsecond || delta > time.Microsecond {
		t.Fatalf("timestamp delta = %v", delta)
	}
}

func TestDecodeFrameRejectsNonGOOSEFrame(t *testing.T) {
	frame := make([]byte, 60)
	frame[12], frame[13] = 0x08, 0x00
	if _, err := DecodeFrame(frame); err == nil {
		t.Fatal("expected non-GOOSE EtherType to be rejected")
	}
}
