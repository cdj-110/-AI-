package collector

import (
	"strings"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/model"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
)

func TestRecordOPCDAOnDataChangeExposesCallbackEvidence(t *testing.T) {
	const deviceKey = "opcda-async-monitor-test"
	sourceTime := time.Date(2026, 8, 28, 14, 1, 2, 345000000, time.Local)
	receiveTime := sourceTime.Add(123 * time.Millisecond)
	recordOPCDAOnDataChange(
		deviceKey, "192.168.1.162", "Bucket Brigade.Real8", int64(888),
		model.QualityGood, sourceTime, receiveTime, false, nil,
	)

	frames := packetmonitor.Default.Snapshot(deviceKey, "opcda", 10)
	if len(frames) != 1 {
		t.Fatalf("OPC DA monitor frames = %d, want 1", len(frames))
	}
	frame := frames[0]
	for _, expected := range []string{
		"[OPCDA][ASYNC] OnDataChange",
		"ItemID=Bucket Brigade.Real8",
		"Value=888",
		"Quality=GOOD",
		"SourceTime=2026-08-28 14:01:02.345",
		"ReceiveTime=2026-08-28 14:01:02.468",
	} {
		if !strings.Contains(frame.Summary, expected) {
			t.Fatalf("summary %q does not contain %q", frame.Summary, expected)
		}
	}
	if frame.Direction != "rx" || frame.Hex != "" || frame.Length != 0 {
		t.Fatalf("unexpected semantic callback frame: %#v", frame)
	}
	if frame.Event != "OnDataChange" || frame.Mode != "Async" || frame.Initial ||
		frame.ItemID != "Bucket Brigade.Real8" || frame.ValueText != "888" ||
		frame.Quality != "GOOD" || frame.SourceTimestamp == nil ||
		frame.ReceiveTimestamp == nil || frame.CallbackLatencyMilliseconds == nil ||
		*frame.CallbackLatencyMilliseconds != 123 {
		t.Fatalf("unexpected structured callback detail: %#v", frame)
	}
}

func TestRecordOPCDASyncReadIsExplicitlyVisible(t *testing.T) {
	const deviceKey = "opcda-sync-monitor-test"
	recordOPCDASyncRead(deviceKey, "192.168.1.162", "DEVICE", 7, "tx", nil)
	frames := packetmonitor.Default.Snapshot(deviceKey, "opcda", 10)
	if len(frames) != 1 || frames[0].Summary != "[OPCDA][SYNC] IOPCSyncIO::Read Source=DEVICE Items=7" ||
		frames[0].Event != "IOPCSyncIO::Read" || frames[0].Mode != "Sync" {
		t.Fatalf("unexpected SyncIO monitor frame: %#v", frames)
	}
}
