package collector

import (
	"context"
	"encoding/binary"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type fakeS7AreaClient struct {
	db map[int][]byte
	mb []byte
	eb []byte
	ab []byte
}

func (f *fakeS7AreaClient) AGReadDB(db, start, size int, buffer []byte) error {
	copy(buffer, f.db[db][start:start+size])
	return nil
}

func (f *fakeS7AreaClient) AGWriteDB(db, start, size int, buffer []byte) error {
	copy(f.db[db][start:start+size], buffer[:size])
	return nil
}

func (f *fakeS7AreaClient) AGReadMB(start, size int, buffer []byte) error {
	copy(buffer, f.mb[start:start+size])
	return nil
}

func (f *fakeS7AreaClient) AGWriteMB(start, size int, buffer []byte) error {
	copy(f.mb[start:start+size], buffer[:size])
	return nil
}

func (f *fakeS7AreaClient) AGReadEB(start, size int, buffer []byte) error {
	copy(buffer, f.eb[start:start+size])
	return nil
}

func (f *fakeS7AreaClient) AGReadAB(start, size int, buffer []byte) error {
	copy(buffer, f.ab[start:start+size])
	return nil
}

func (f *fakeS7AreaClient) AGWriteAB(start, size int, buffer []byte) error {
	copy(f.ab[start:start+size], buffer[:size])
	return nil
}

func TestSiemensS7SupportsWrite(t *testing.T) {
	bit8 := uint8(8)
	cases := []struct {
		name string
		p    config.PointConfig
		want bool
	}{
		{"marker-bool", config.PointConfig{Area: "M", DataType: "bool"}, true},
		{"db-number", config.PointConfig{Area: "DB", DataType: "uint16"}, true},
		{"output-real", config.PointConfig{Area: "Q", DataType: "float32"}, true},
		{"input", config.PointConfig{Area: "I", DataType: "bool"}, false},
		{"unsupported-type", config.PointConfig{Area: "M", DataType: "float64"}, false},
		{"invalid-bit", config.PointConfig{Area: "M", DataType: "bool", BitIndex: &bit8}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SiemensS7SupportsWrite(tc.p); got != tc.want {
				t.Fatalf("SiemensS7SupportsWrite() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWriteAndVerifyGoS7BoolBitPreservesOtherBits(t *testing.T) {
	bit := uint8(1)
	fake := &fakeS7AreaClient{db: map[int][]byte{}, mb: []byte{0xa0}, eb: make([]byte, 1), ab: make([]byte, 1)}
	conn := &goS7Connection{client: fake}
	point := config.PointConfig{Area: "M", Register: 0, DataType: "bool", BitIndex: &bit}
	raw, err := encodeS7Value(point, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAndVerifyGoS7Point(context.Background(), conn, point, raw); err != nil {
		t.Fatal(err)
	}
	if fake.mb[0] != 0xa2 {
		t.Fatalf("marker byte = 0x%02x, want 0xa2", fake.mb[0])
	}
}

func TestWriteAndVerifyGoS7DBNumber(t *testing.T) {
	fake := &fakeS7AreaClient{db: map[int][]byte{1: make([]byte, 8)}, mb: make([]byte, 1), eb: make([]byte, 1), ab: make([]byte, 1)}
	conn := &goS7Connection{client: fake}
	point := config.PointConfig{Area: "DB", DBNumber: 1, Register: 2, DataType: "uint16", Scale: 1}
	raw, err := encodeS7Value(point, 27125)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAndVerifyGoS7Point(context.Background(), conn, point, raw); err != nil {
		t.Fatal(err)
	}
	if got := binary.BigEndian.Uint16(fake.db[1][2:4]); got != 27125 {
		t.Fatalf("DB value = %d, want 27125", got)
	}
}

func TestEncodeS7ValueRejectsUnsafeInteger(t *testing.T) {
	point := config.PointConfig{Area: "DB", DataType: "uint16", Scale: 1}
	for _, value := range []interface{}{-1, 65536, 1.5} {
		if _, err := encodeS7Value(point, value); err == nil {
			t.Fatalf("encodeS7Value(%v) unexpectedly succeeded", value)
		}
	}
}

func TestBuildAndParseS7CustomTSAPWrite(t *testing.T) {
	bit := uint8(2)
	point := config.PointConfig{Area: "M", Register: 3, DataType: "bool", BitIndex: &bit}
	packet, err := buildS7WriteRequest(7, point, []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.BigEndian.Uint16(packet[2:4]); int(got) != len(packet) {
		t.Fatalf("TPKT length = %d, want %d", got, len(packet))
	}
	if packet[17] != 0x05 || packet[22] != 0x01 || packet[27] != 0x83 {
		t.Fatalf("unexpected write header: %x", packet)
	}
	if got := uint32(packet[28])<<16 | uint32(packet[29])<<8 | uint32(packet[30]); got != 26 {
		t.Fatalf("bit address = %d, want 26", got)
	}
	response := []byte{
		0x03, 0x00, 0x00, 0x16, 0x02, 0xf0, 0x80,
		0x32, 0x03, 0x00, 0x00, 0x00, 0x07, 0x00, 0x02, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x01, 0xff,
	}
	if err := parseS7WriteResponse(response); err != nil {
		t.Fatal(err)
	}
	response[len(response)-1] = 0x05
	if err := parseS7WriteResponse(response); err == nil {
		t.Fatal("parseS7WriteResponse unexpectedly accepted a failed write")
	}
}
