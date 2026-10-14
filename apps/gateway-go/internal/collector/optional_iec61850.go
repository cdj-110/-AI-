//go:build iec61850_mms && cgo

package collector

func newOptionalCollector(protocol string) (Collector, bool) {
	if protocol == "iec61850" {
		return IEC61850{}, true
	}
	return nil, false
}
