//go:build !linux

package goose

import (
	"context"
	"fmt"
	"net"
)

type Socket struct{}

func Open(string) (*Socket, error) {
	return nil, fmt.Errorf("IEC61850 GOOSE raw Ethernet is supported on Linux gateways")
}
func (s *Socket) SourceMAC() net.HardwareAddr { return nil }
func (s *Socket) JoinMulticast(net.HardwareAddr) error {
	return fmt.Errorf("IEC61850 GOOSE raw Ethernet is supported on Linux gateways")
}
func (s *Socket) JoinAllMulticast() error {
	return fmt.Errorf("IEC61850 GOOSE raw Ethernet is supported on Linux gateways")
}
func (s *Socket) Receive(context.Context) ([]byte, error) {
	return nil, fmt.Errorf("IEC61850 GOOSE raw Ethernet is supported on Linux gateways")
}
func (s *Socket) Send([]byte) error {
	return fmt.Errorf("IEC61850 GOOSE raw Ethernet is supported on Linux gateways")
}
func (s *Socket) Close() error { return nil }
