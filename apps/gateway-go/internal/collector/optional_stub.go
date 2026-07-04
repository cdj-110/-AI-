//go:build !iec61850_mms || !cgo

package collector

func newOptionalCollector(protocol string) (Collector, bool) {
	return nil, false
}
