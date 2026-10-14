// A small read-only probe to verify the EG100 system CA store and LAN HTTPS.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "expected HTTPS URL")
		os.Exit(2)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer response.Body.Close()
	fmt.Printf("TLS verified; HTTP status: %d\n", response.StatusCode)
}
