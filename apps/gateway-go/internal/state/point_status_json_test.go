package state

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPointStatusJSONPreservesTypesAndRawValues(t *testing.T) {
	for _, test := range []struct {
		name  string
		value interface{}
		want  string
	}{
		{"tail", 43.800000000000004, `"value":43.8`},
		{"precision", 1.2345678901234567, `"value":1.2345678901234567`},
		{"uint64", uint64(18446744073709551615), `"value":18446744073709551615`},
		{"bool", true, `"value":true`},
		{"string", "43.800000000000004", `"value":"43.800000000000004"`},
		{"array", []float64{43.800000000000004}, `"value":[43.8]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			point := PointStatus{DeviceKey: "source", Metric: "a", Value: test.value, Decimals: 2}
			encoded, err := json.Marshal(point)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), test.want) || !strings.Contains(string(encoded), `"decimals":2`) {
				t.Fatal(string(encoded))
			}
			if values, ok := point.Value.([]float64); ok && values[0] != 43.800000000000004 {
				t.Fatal("original array changed")
			}
		})
	}
}
