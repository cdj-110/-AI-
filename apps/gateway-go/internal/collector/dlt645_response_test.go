package collector

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goburrow/serial"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
)

type responseDLTPort struct {
	chunks  [][]byte
	writes  int
	onWrite func([]byte)
	onRead  func() []byte
}

func (*responseDLTPort) Open(*serial.Config) error { return nil }
func (*responseDLTPort) Close() error              { return nil }
func (p *responseDLTPort) Write(b []byte) (int, error) {
	p.writes++
	if p.onWrite != nil {
		p.onWrite(b)
	}
	return len(b), nil
}
func (p *responseDLTPort) Read(b []byte) (int, error) {
	if len(p.chunks) == 0 && p.onRead != nil {
		if raw := p.onRead(); len(raw) > 0 {
			p.chunks = append(p.chunks, raw)
		}
	}
	if len(p.chunks) > 0 {
		n := copy(b, p.chunks[0])
		p.chunks[0] = p.chunks[0][n:]
		if len(p.chunks[0]) == 0 {
			p.chunks = p.chunks[1:]
		}
		return n, nil
	}
	time.Sleep(time.Millisecond)
	return 0, serial.ErrTimeout
}
func responseDLTFrame(control byte, data []byte) []byte {
	raw, _ := buildDLT645Frame("000000000001", control, data, 0)
	return raw
}

func TestDLT645IgnoresOtherDIUntilMatchingReply(t *testing.T) {
	for _, version := range []string{"2007", "1997"} {
		for _, split := range []bool{false, true} {
			t.Run(version+map[bool]string{true: "/split", false: "/sticky"}[split], func(t *testing.T) {
				control := byte(0x11)
				di := []byte{0, 0, 0, 0}
				if version == "1997" {
					control = 1
					di = []byte{0x10, 0x90}
				}
				wrong := append([]byte(nil), di...)
				wrong[0]++
				bad := responseDLTFrame(control|0x80, append(wrong, 0x99))
				good := responseDLTFrame(control|0x80, append(append([]byte(nil), di...), 0x12))
				request := responseDLTFrame(control, di)
				stream := append(append(append([]byte{0, 0xfe}, request...), bad...), good...)
				p := &responseDLTPort{chunks: [][]byte{stream}}
				if split {
					p.chunks = [][]byte{stream[:13], stream[13 : len(stream)-4], stream[len(stream)-4:]}
				}
				point := scheduledDLTPoint()
				frame, err := exchangeDLT645(context.Background(), &dlt645Bus{port: p}, point, version, request, point.DLT645.MeterAddress, control)
				if err != nil || !bytes.Equal(frame.Raw, good) || p.writes != 1 {
					t.Fatalf("frame=%X writes=%d err=%v", frame.Raw, p.writes, err)
				}
			})
		}
	}
}

func TestDLT645WrongDIDoesNotExtendDeadline(t *testing.T) {
	p := &responseDLTPort{onRead: func() []byte { time.Sleep(time.Millisecond); return responseDLTFrame(0x91, []byte{1, 0, 0, 0, 0}) }}
	point := scheduledDLTPoint()
	point.DLT645.ResponseTimeoutMilliseconds = 25
	start := time.Now()
	bus := &dlt645Bus{port: p}
	_, err := exchangeDLT645(context.Background(), bus, point, "2007", responseDLTFrame(0x11, []byte{0, 0, 0, 0}), point.DLT645.MeterAddress, 0x11)
	if err == nil || !strings.Contains(err.Error(), "请求 DI=00000000，收到 DI=00000001") || !strings.Contains(err.Error(), "响应超时") {
		t.Fatal(err)
	}
	if time.Since(start) > 200*time.Millisecond || bus.resyncWindow == 0 {
		t.Fatal("mismatches extended deadline or failed to quarantine bus")
	}
}

func TestDLT645ResidualSurvivesReopenAndIsDrainedBeforeRetry(t *testing.T) {
	CloseDLT645Connections()
	old := dlt645OpenPort
	t.Cleanup(func() { CloseDLT645Connections(); dlt645OpenPort = old })
	p := &responseDLTPort{}
	var sent time.Time
	discarded := false
	p.onWrite = func(raw []byte) {
		if p.writes == 1 {
			sent = time.Now()
			return
		}
		if !discarded {
			t.Error("retry sent before late reply drained")
		}
		request, _, _, _ := parseDLT645Stream(raw)
		p.chunks = append(p.chunks, responseDLTFrame(0x91, append(request.Data, 0x23, 0x01, 0, 0)))
	}
	p.onRead = func() []byte {
		if p.writes == 1 && !discarded && time.Since(sent) >= 40*time.Millisecond {
			discarded = true
			// Same DI as the retry: DI matching alone cannot protect freshness.
			return responseDLTFrame(0x91, []byte{0, 0, 0, 0, 0x99, 0x99, 0, 0})
		}
		return nil
	}
	opens := 0
	dlt645OpenPort = func(*serial.Config) (serial.Port, error) { opens++; return p, nil }
	point := scheduledDLTPoint()
	point.DLT645.ResponseTimeoutMilliseconds = 20
	value, err := (DLT645{}).ReadPoint(context.Background(), point)
	if err != nil || value.Value != float64(123) || p.writes != 2 || opens != 2 || !discarded {
		t.Fatalf("value=%v writes=%d opens=%d drained=%v err=%v", value.Value, p.writes, opens, discarded, err)
	}
}

func TestDLT645ResidualClearedBeforeDifferentPoint(t *testing.T) {
	p := &responseDLTPort{}
	point := scheduledDLTPoint()
	point.DLT645.ResponseTimeoutMilliseconds = 10
	bus := &dlt645Bus{port: p}
	_, err := exchangeDLT645(context.Background(), bus, point, "2007", responseDLTFrame(0x11, []byte{0, 0, 0, 0}), point.DLT645.MeterAddress, 0x11)
	if err == nil {
		t.Fatal("expected timeout")
	}
	p.chunks = [][]byte{responseDLTFrame(0x91, []byte{0, 0, 0, 0, 0x99})}
	di := []byte{0, 4, 0, 0}
	p.onWrite = func([]byte) {
		if len(p.chunks) > 0 {
			t.Error("old reply not drained")
		}
		p.chunks = append(p.chunks, responseDLTFrame(0x91, append(di, 0x23)))
	}
	r, err := exchangeDLT645(context.Background(), bus, point, "2007", responseDLTFrame(0x11, di), point.DLT645.MeterAddress, 0x11)
	if err != nil || !bytes.Equal(r.Data[:4], di) {
		t.Fatalf("%v %v", r, err)
	}
}

func TestDLT645ResidualCancellationSendsNothing(t *testing.T) {
	p := &responseDLTPort{}
	bus := &dlt645Bus{port: p, resyncWindow: time.Second}
	point := scheduledDLTPoint()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	_, err := exchangeDLT645(ctx, bus, point, "2007", responseDLTFrame(0x11, []byte{0, 0, 0, 0}), point.DLT645.MeterAddress, 0x11)
	if !errors.Is(err, context.DeadlineExceeded) || p.writes != 0 || bus.resyncWindow == 0 {
		t.Fatalf("writes=%d err=%v", p.writes, err)
	}
}

func TestDLT645ContinuousResidualIsBoundedAndNeverSends(t *testing.T) {
	p := &responseDLTPort{onRead: func() []byte { time.Sleep(5 * time.Millisecond); return []byte{0xfe} }}
	bus := &dlt645Bus{port: p, resyncWindow: 10 * time.Millisecond}
	point := scheduledDLTPoint()
	start := time.Now()
	_, err := exchangeDLT645(context.Background(), bus, point, "2007", responseDLTFrame(0x11, []byte{0, 0, 0, 0}), point.DLT645.MeterAddress, 0x11)
	if err == nil || !strings.Contains(err.Error(), "未达到静默") || p.writes != 0 || time.Since(start) > 2*time.Second {
		t.Fatalf("writes=%d err=%v", p.writes, err)
	}
}

func TestDLT645MatchingPreservesExceptionsAndWriteAck(t *testing.T) {
	point := scheduledDLTPoint()
	for _, control := range []byte{0x01, 0x11, 0x04, 0x14, 0x13} {
		p := &responseDLTPort{chunks: [][]byte{responseDLTFrame(control|0xc0, []byte{0x02})}}
		_, err := exchangeDLT645(context.Background(), &dlt645Bus{port: p}, point, "2007", responseDLTFrame(control, []byte{0, 0, 0, 0}), point.DLT645.MeterAddress, control)
		if err == nil || !strings.Contains(err.Error(), "无请求数据") {
			t.Fatalf("control=%X err=%v", control, err)
		}
	}
	for _, control := range []byte{0x04, 0x14, 0x13} {
		p := &responseDLTPort{chunks: [][]byte{responseDLTFrame(control|0x80, nil)}}
		_, err := exchangeDLT645(context.Background(), &dlt645Bus{port: p}, point, "2007", responseDLTFrame(control, []byte{0, 0, 0, 0}), point.DLT645.MeterAddress, control)
		if err != nil {
			t.Fatalf("control=%X err=%v", control, err)
		}
	}
}

func TestDLT645TraceEchoCredentialsRemainMasked(t *testing.T) {
	point := scheduledDLTPoint()
	point.DeviceKey = "write-echo-redaction-test"
	raw := responseDLTFrame(0x14, []byte{0, 0, 0, 0, 0x12, 0x34, 0x56, 0x78, 1, 2, 3, 4})
	recordDLT645Frame(point, "2007", "rx", raw, 0x14, errors.New("ignored echo"))
	frames := packetmonitor.Default.Snapshot(point.DeviceKey, "dlt645", 1)
	if len(frames) != 1 || frames[0].Hex != "" || frames[0].ValueText != "***" {
		t.Fatal(frames)
	}
}

func TestDLT645IgnoredReplyTraceSeparatesChecksumFromMatching(t *testing.T) {
	point := scheduledDLTPoint()
	point.DeviceKey = "ignored-reply-trace-test"
	raw := responseDLTFrame(0x91, []byte{1, 0, 0, 0, 0x23})
	recordDLT645Frame(point, "2007", "rx", raw, 0x91, errors.New("已忽略：响应数据标识不匹配"))
	frames := packetmonitor.Default.Snapshot(point.DeviceKey, "dlt645", 1)
	if len(frames) != 1 || !strings.Contains(frames[0].Summary, "校验=通过") || !strings.Contains(frames[0].Summary, "收到DI=00000001") || frames[0].ItemID != "00000000" || frames[0].Hex == "" {
		t.Fatal(frames)
	}
}
