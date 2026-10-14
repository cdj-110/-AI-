//go:build !iec61850_mms || !cgo

package collector

import (
	"context"
	"fmt"
)

func BrowseIEC61850Nodes(ctx context.Context, address string, parentRef string, recursive bool) ([]IEC61850BrowseNode, error) {
	return nil, fmt.Errorf("IEC61850 MMS browse is not built in; rebuild with CGO_ENABLED=1 and -tags iec61850_mms after installing libiec61850")
}

func ProbeIEC61850ObjectRefs(ctx context.Context, address string, refs []IEC61850ProbeResult) ([]IEC61850ProbeResult, error) {
	return nil, fmt.Errorf("IEC61850 MMS probe is not built in; rebuild with CGO_ENABLED=1 and -tags iec61850_mms after installing libiec61850")
}

func DiscoverIEC61850TemplateNodes(ctx context.Context, address string, iedName string) ([]IEC61850BrowseNode, error) {
	return nil, fmt.Errorf("IEC61850 MMS template discovery is not built in; rebuild with CGO_ENABLED=1 and -tags iec61850_mms after installing libiec61850")
}

func MergeIEC61850BrowseNodes(nodes []IEC61850BrowseNode, extra []IEC61850BrowseNode) []IEC61850BrowseNode {
	return append(nodes, extra...)
}
