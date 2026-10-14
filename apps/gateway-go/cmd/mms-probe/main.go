// mms-probe performs read-only model discovery and sample reads for field diagnostics.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func main() {
	address := flag.String("address", "", "MMS server host:port (required)")
	limit := flag.Int("samples", 5, "maximum leaf values to read")
	flag.Parse()
	if *address == "" || *limit < 0 || *limit > 20 {
		fmt.Fprintln(os.Stderr, "provide -address host:port and -samples between 0 and 20")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer collector.CloseIEC61850Connections()
	if _, err := collector.TestIEC61850Connection(ctx, *address); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	nodes, err := collector.BrowseIEC61850Nodes(ctx, *address, "", true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	leaves := []collector.IEC61850BrowseNode{}
	var walk func([]collector.IEC61850BrowseNode)
	walk = func(items []collector.IEC61850BrowseNode) {
		for _, node := range items {
			if node.Leaf && node.FC != "" {
				leaves = append(leaves, node)
			}
			walk(node.Children)
		}
	}
	walk(nodes)
	output := map[string]interface{}{"address": *address, "association": "ok", "roots": len(nodes), "leaves": len(leaves)}
	samples := []map[string]interface{}{}
	for i, node := range leaves {
		if i >= *limit {
			break
		}
		point := config.PointConfig{Protocol: "iec61850", Address: *address, ObjectRef: node.ObjectRef, FC: node.FC, Scale: 1}
		value, readErr := (collector.IEC61850{}).ReadPoint(ctx, point)
		sample := map[string]interface{}{"objectRef": node.ObjectRef, "fc": node.FC}
		if readErr != nil {
			sample["error"] = readErr.Error()
		} else {
			sample["value"] = value.Value
		}
		samples = append(samples, sample)
	}
	output["samples"] = samples
	_ = json.NewEncoder(os.Stdout).Encode(output)
}
