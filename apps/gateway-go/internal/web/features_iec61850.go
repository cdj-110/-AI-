//go:build iec61850_mms && cgo

package web

const iec61850Enabled = true

func boolLiteral(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
