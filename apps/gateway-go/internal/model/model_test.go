package model

import (
	"testing"
	"time"
)

func TestPointValueDefaultsAreProtocolNeutral(t *testing.T) {
	now := time.Now()
	value := PointValue{Value: []uint64{1, 2}, ActualDataType: "auto"}
	value.ApplyDefaults(now)
	if value.Quality != QualityGood || value.ActualDataType != "uint64[]" || !value.ReceiveTimestamp.Equal(now) {
		t.Fatalf("unexpected defaults: %#v", value)
	}

	native := PointValue{Value: float64(1), ActualDataType: "VT_R8"}
	native.ApplyDefaults(now)
	if native.ActualDataType != "VT_R8" {
		t.Fatalf("native protocol type was overwritten: %#v", native)
	}
}
