package state

import (
	"encoding/json"

	"weikong-iot-platform/apps/gateway-go/internal/valueformat"
)

// MarshalJSON applies the same transport formatting to REST snapshots and
// realtime diffs. The store and its calculation/forwarding consumers keep the
// original values; Decimals affects presentation only.
func (point PointStatus) MarshalJSON() ([]byte, error) {
	type wirePoint PointStatus
	copy := wirePoint(point)
	copy.Value = valueformat.Value(point.Value)
	return json.Marshal(copy)
}
