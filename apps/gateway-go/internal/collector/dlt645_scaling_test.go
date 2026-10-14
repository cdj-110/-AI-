package collector

import (
	"context"
	"math"
	"testing"
)

func TestDLT645ScaleOffsetAfterBCDDecimals(t *testing.T) {
	for _, item := range []string{"", "combined-active-total"} {
		t.Run("item="+item, func(t *testing.T) {
			installDelayedDLT(t, 0)
			p := scheduledDLTPoint()
			p.DLT645.ItemKey = item
			p.Decimals = 2
			p.Scale = 0.5
			p.Offset = -2
			value, err := (DLT645{}).ReadPoint(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			// Raw BCD 123 -> 1.23 -> 1.23*0.5-2 = -1.385.
			got, ok := value.Value.(float64)
			if !ok || math.Abs(got-(-1.385)) > 1e-10 {
				t.Fatalf("scaling should run once after decimal decoding: %#v", value)
			}
		})
	}
}
