package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/collector"
)

func main() {
	address := flag.String("address", "127.0.0.1:102", "IEC61850 server address")
	parentRef := flag.String("parent", "", "optional parent object reference")
	recursive := flag.Bool("recursive", true, "scan recursively")
	timeout := flag.Duration("timeout", 30*time.Second, "scan timeout")
	limit := flag.Int("limit", 50, "maximum nodes to print; 0 prints all")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	nodes, err := collector.BrowseIEC61850Nodes(ctx, *address, *parentRef, *recursive)
	if err != nil {
		log.Fatalf("browse failed: %v", err)
	}

	if *limit > 0 && len(nodes) > *limit {
		nodes = nodes[:*limit]
	}
	payload, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		log.Fatalf("encode result failed: %v", err)
	}
	fmt.Fprintf(os.Stdout, "nodes=%d\n%s\n", len(nodes), payload)
}
