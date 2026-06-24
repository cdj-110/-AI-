package collector

import (
	"testing"
	"time"
)

func TestIEC104ValueAfterRejectsCachedValue(t *testing.T) {
	ioa := uint32(16385)
	oldUpdate := time.Now().Add(-time.Second)
	client := &iec104Client{
		values:       map[uint32]interface{}{ioa: float32(61)},
		valueUpdated: map[uint32]time.Time{ioa: oldUpdate},
	}

	if _, ok := client.valueAfter(ioa, oldUpdate); ok {
		t.Fatal("cached value was accepted as a fresh interrogation result")
	}
	client.valueUpdated[ioa] = oldUpdate.Add(time.Millisecond)
	if value, ok := client.valueAfter(ioa, oldUpdate); !ok || value != float32(61) {
		t.Fatalf("fresh value = %v, %v", value, ok)
	}
}
