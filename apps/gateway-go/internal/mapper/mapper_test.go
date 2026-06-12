package mapper

import (
	"math"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestDecodeFloat32WithNormalWords(t *testing.T) {
	value, err := Decode(config.PointConfig{DataType: "float32", ByteOrder: "big", WordOrder: "normal", Scale: 1}, []byte{0x41, 0xD2, 0x00, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, value, 26.25)
}

func TestDecodeFloat32WithSwappedWords(t *testing.T) {
	value, err := Decode(config.PointConfig{DataType: "float32", ByteOrder: "big", WordOrder: "swap", Scale: 1}, []byte{0x00, 0x00, 0x41, 0xD2})
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, value, 26.25)
}

func TestDecodeLittleEndianUint16(t *testing.T) {
	value, err := Decode(config.PointConfig{DataType: "uint16", ByteOrder: "little", WordOrder: "normal", Scale: 0.1}, []byte{0x10, 0x27})
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, value, 1000)
}

func TestDecodeBitBool(t *testing.T) {
	bit := uint8(3)
	value, err := Decode(config.PointConfig{DataType: "bool", ByteOrder: "big", WordOrder: "normal", BitIndex: &bit}, []byte{0x00, 0x08})
	if err != nil {
		t.Fatal(err)
	}
	if value != true {
		t.Fatalf("expected true, got %v", value)
	}
}

func assertFloat(t *testing.T, value interface{}, expected float64) {
	t.Helper()
	actual, ok := value.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", value)
	}
	if math.Abs(actual-expected) > 0.0001 {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}
