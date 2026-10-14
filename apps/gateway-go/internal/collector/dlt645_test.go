package collector

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"sync"
	"testing"

	"github.com/goburrow/serial"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type simulatedDLT645Port struct {
	mu       sync.Mutex
	response []byte
	controls *[]byte
}

func (port *simulatedDLT645Port) Open(*serial.Config) error { return nil }
func (port *simulatedDLT645Port) Close() error              { return nil }
func (port *simulatedDLT645Port) Write(request []byte) (int, error) {
	parsed, _, complete, err := parseDLT645Stream(request)
	if err != nil || !complete {
		return 0, io.ErrUnexpectedEOF
	}
	port.mu.Lock()
	defer port.mu.Unlock()
	*port.controls = append(*port.controls, parsed.Control)
	// Simulate a legacy meter: 2007 requests time out, 1997 requests reply.
	if parsed.Control == 0x01 {
		payload := append(append([]byte{}, parsed.Data...), []byte{0x56, 0x34, 0x12, 0x00}...)
		port.response, _ = buildDLT645Frame(parsed.Address, 0x81, payload, 2)
	}
	return len(request), nil
}
func (port *simulatedDLT645Port) Read(target []byte) (int, error) {
	port.mu.Lock()
	defer port.mu.Unlock()
	if len(port.response) == 0 {
		return 0, serial.ErrTimeout
	}
	n := copy(target, port.response)
	port.response = port.response[n:]
	return n, nil
}

func TestDLT645GoldenFrame2007(t *testing.T) {
	frame, err := buildDLT645Frame("123456789012", 0x11, []byte{0, 0, 0, 0}, 4)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix, _ := hex.DecodeString("FEFEFEFE6812907856341268110433333333")
	if !bytes.Equal(frame[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("frame prefix = % X, want % X", frame[:len(wantPrefix)], wantPrefix)
	}
	parsed, consumed, complete, err := parseDLT645Stream(append([]byte{0x55, 0xFE}, frame...))
	if err != nil || !complete {
		t.Fatalf("parse complete=%v err=%v", complete, err)
	}
	if consumed != len(frame)+2 || parsed.Address != "123456789012" || parsed.Control != 0x11 || !bytes.Equal(parsed.Data, []byte{0, 0, 0, 0}) {
		t.Fatalf("unexpected parsed frame: %#v consumed=%d", parsed, consumed)
	}
}

func TestDLT645StreamFragmentAndChecksum(t *testing.T) {
	frame, err := buildDLT645Frame("000000000001", 0x91, []byte{0, 0, 0, 0, 0x56, 0x34, 0x12, 0x00}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, complete, err := parseDLT645Stream(frame[:8]); err != nil || complete {
		t.Fatalf("fragment complete=%v err=%v", complete, err)
	}
	bad := append([]byte(nil), frame...)
	bad[len(bad)-2]++
	if _, _, _, err := parseDLT645Stream(bad); err == nil {
		t.Fatal("expected checksum error")
	}
	stream := append(append([]byte{0x00, 0xFE}, frame...), frame...)
	first, used, complete, err := parseDLT645Stream(stream)
	if err != nil || !complete || first.Address != "000000000001" {
		t.Fatalf("first frame failed: %#v used=%d complete=%v err=%v", first, used, complete, err)
	}
	if _, _, complete, err = parseDLT645Stream(stream[used:]); err != nil || !complete {
		t.Fatalf("sticky second frame complete=%v err=%v", complete, err)
	}
}

func TestDLT645ValueCodecs(t *testing.T) {
	raw, err := encodeDLT645Value("12345.67", "bcd", 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	value, kind, err := decodeDLT645Value(raw, "bcd", 2)
	if err != nil || kind != "float64" || value != float64(12345.67) {
		t.Fatalf("round trip value=%v type=%s err=%v raw=% X", value, kind, err, raw)
	}
	negative, err := encodeDLT645Value("-12.3456", "signed-bcd", 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	value, _, err = decodeDLT645Value(negative, "signed-bcd", 4)
	if err != nil || value != float64(-12.3456) {
		t.Fatalf("signed value=%v err=%v raw=% X", value, err, negative)
	}
	if _, _, err = decodeDLT645Value([]byte{0xFA}, "bcd", 0); err == nil {
		t.Fatal("expected invalid BCD error")
	}
	if _, err = encodeDLT645Value(-1, "bcd", 2, 0); err == nil {
		t.Fatal("expected unsigned BCD negative error")
	}
	binaryValue, err := encodeDLT645Value("4294967295", "binary", 4, 0)
	if err != nil || !bytes.Equal(binaryValue, []byte{0xff, 0xff, 0xff, 0xff}) {
		t.Fatalf("unexpected binary encoding: % X, %v", binaryValue, err)
	}
	if _, err = encodeDLT645Value("4294967296", "binary", 4, 0); err == nil {
		t.Fatal("expected uint32 overflow to be rejected")
	}
}

func TestDLT645AddressValidation(t *testing.T) {
	raw, err := encodeDLT645Address("000000000001")
	if err != nil || decodeDLT645Address(raw) != "000000000001" {
		t.Fatalf("address raw=% X decoded=%s err=%v", raw, decodeDLT645Address(raw), err)
	}
	for _, invalid := range []string{"1", "00000000000A", "9999999999999"} {
		if _, err := encodeDLT645Address(invalid); err == nil {
			t.Fatalf("expected invalid address %q", invalid)
		}
	}
}

func TestDLT645AutoDetectsAndCaches1997WithSimulatedSerial(t *testing.T) {
	CloseDLT645Connections()
	previousOpen := dlt645OpenPort
	defer func() { dlt645OpenPort = previousOpen; CloseDLT645Connections() }()
	var controls []byte
	dlt645OpenPort = func(*serial.Config) (serial.Port, error) {
		return &simulatedDLT645Port{controls: &controls}, nil
	}
	point := config.PointConfig{DeviceKey: "meter", Metric: "energy", Protocol: "dlt645", Address: "simulated", BaudRate: 2400, DataBits: 8, StopBits: 1, Parity: "even", DLT645: &config.DLT645Config{Version: "auto", MeterAddress: "000000000001", ResponseTimeoutMilliseconds: 5, Retries: 0, WakeupBytes: 2, ItemKey: "forward-active-total", Access: "read"}, Scale: 1}
	value, err := (DLT645{}).ReadPoint(context.Background(), point)
	if err != nil {
		t.Fatal(err)
	}
	if value.Value != float64(1234.56) || value.Diagnostic != "DL/T 645-1997" {
		t.Fatalf("unexpected value: %#v", value)
	}
	before := len(controls)
	if _, err = (DLT645{}).ReadPoint(context.Background(), point); err != nil {
		t.Fatal(err)
	}
	if len(controls) != before+1 || controls[len(controls)-1] != 0x01 {
		t.Fatalf("cached read should issue one 1997 request, controls=% X", controls)
	}
}

func TestDLT645ExceptionChineseExplanation(t *testing.T) {
	err := dlt645Exception([]byte{0x06})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("密码错误或未授权")) || !bytes.Contains([]byte(err.Error()), []byte("无请求数据")) {
		t.Fatalf("unexpected exception: %v", err)
	}
}

func TestExchangeDLT645RejectsNilBusPortWithoutPanic(t *testing.T) {
	point := config.PointConfig{Address: "/dev/ttyS2"}
	_, err := exchangeDLT645(context.Background(), &dlt645Bus{}, point, "2007", []byte{0x68}, "000000000001", 0x11)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("尚未打开或已断开")) {
		t.Fatalf("expected closed-port diagnostic, got %v", err)
	}
}

func TestDLT645ReopensSharedBusAfterPreviousTransactionClosedPort(t *testing.T) {
	CloseDLT645Connections()
	previousOpen := dlt645OpenPort
	defer func() { dlt645OpenPort = previousOpen; CloseDLT645Connections() }()
	var controls []byte
	dlt645OpenPort = func(*serial.Config) (serial.Port, error) {
		return &simulatedDLT645Port{controls: &controls}, nil
	}
	point := config.PointConfig{DeviceKey: "meter", Metric: "energy", Protocol: "dlt645", Address: "simulated-reopen", BaudRate: 2400, DataBits: 8, StopBits: 1, Parity: "even", DLT645: &config.DLT645Config{Version: "1997", MeterAddress: "000000000001", ResponseTimeoutMilliseconds: 5, Retries: 0, WakeupBytes: 2, ItemKey: "forward-active-total", Access: "read"}, Scale: 1}
	settings := dlt645Settings(point)
	dlt645Pool.Lock()
	dlt645Pool.items[point.Address] = &dlt645Bus{settings: settings}
	dlt645Pool.Unlock()

	value, err := (DLT645{}).ReadPoint(context.Background(), point)
	if err != nil {
		t.Fatal(err)
	}
	if value.Value != float64(1234.56) || len(controls) != 1 {
		t.Fatalf("unexpected reopened read value=%v controls=% X", value.Value, controls)
	}
}
