//go:build !iec61850_mms || !cgo

package collector

import (
	"context"
	"fmt"
)

func BrowseIEC61850Nodes(ctx context.Context, address string, parentRef string, recursive bool) ([]IEC61850BrowseNode, error) {
	return nil, fmt.Errorf("IEC61850 MMS browse is not built in; rebuild with CGO_ENABLED=1 and -tags iec61850_mms after installing libiec61850")
}
