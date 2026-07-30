//go:build linux

package goose

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"
)

const (
	solPacket            = 263
	packetAddMembership  = 1
	packetMRMulticast    = 0
	packetMRAllMulticast = 2
)

type packetMreq struct {
	interfaceIndex int32
	membershipType uint16
	addressLength  uint16
	address        [8]byte
}

type Socket struct {
	fd             int
	interfaceIndex int
	sourceMAC      net.HardwareAddr
}

func Open(interfaceName string) (*Socket, error) {
	networkInterface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("find GOOSE interface %s: %w", interfaceName, err)
	}
	if len(networkInterface.HardwareAddr) != 6 {
		return nil, fmt.Errorf("GOOSE interface %s has no Ethernet MAC", interfaceName)
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(EtherType)))
	if err != nil {
		return nil, fmt.Errorf("open GOOSE raw socket: %w", err)
	}
	address := &syscall.SockaddrLinklayer{Protocol: htons(EtherType), Ifindex: networkInterface.Index}
	if err := syscall.Bind(fd, address); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("bind GOOSE raw socket to %s: %w", interfaceName, err)
	}
	if err := syscall.SetNonblock(fd, true); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	return &Socket{fd: fd, interfaceIndex: networkInterface.Index, sourceMAC: append(net.HardwareAddr(nil), networkInterface.HardwareAddr...)}, nil
}

func (s *Socket) SourceMAC() net.HardwareAddr { return append(net.HardwareAddr(nil), s.sourceMAC...) }

// JoinMulticast enables the interface's hardware filter for a GOOSE
// destination. AF_PACKET sockets do not automatically subscribe to multicast
// MAC addresses.
func (s *Socket) JoinMulticast(address net.HardwareAddr) error {
	if len(address) != 6 || address[0]&1 == 0 {
		return fmt.Errorf("GOOSE destination %q is not a multicast MAC", address)
	}
	request := packetMreq{
		interfaceIndex: int32(s.interfaceIndex),
		membershipType: packetMRMulticast,
		addressLength:  uint16(len(address)),
	}
	copy(request.address[:], address)
	_, _, errno := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(s.fd),
		uintptr(solPacket),
		uintptr(packetAddMembership),
		uintptr(unsafe.Pointer(&request)),
		unsafe.Sizeof(request),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("join GOOSE multicast %s: %w", address, errno)
	}
	return nil
}

// JoinAllMulticast temporarily enables receipt of every multicast frame on
// the bound interface. It is used only by the bounded diagnostic sniffer.
func (s *Socket) JoinAllMulticast() error {
	request := packetMreq{
		interfaceIndex: int32(s.interfaceIndex),
		membershipType: packetMRAllMulticast,
	}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(s.fd),
		uintptr(solPacket),
		uintptr(packetAddMembership),
		uintptr(unsafe.Pointer(&request)),
		unsafe.Sizeof(request),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("enable all-multicast GOOSE sniffing: %w", errno)
	}
	return nil
}

func (s *Socket) Receive(ctx context.Context) ([]byte, error) {
	buffer := make([]byte, 65536)
	for {
		count, _, err := syscall.Recvfrom(s.fd, buffer, 0)
		if err == nil {
			return append([]byte(nil), buffer[:count]...), nil
		}
		if err != syscall.EAGAIN && err != syscall.EWOULDBLOCK {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (s *Socket) Send(frame []byte) error {
	if len(frame) < 6 {
		return fmt.Errorf("GOOSE Ethernet frame is too short")
	}
	address := &syscall.SockaddrLinklayer{
		Protocol: htons(EtherType),
		Ifindex:  s.interfaceIndex,
		Halen:    6,
	}
	copy(address.Addr[:], frame[:6])
	return syscall.Sendto(s.fd, frame, 0, address)
}

func (s *Socket) Close() error { return syscall.Close(s.fd) }

func htons(value uint16) uint16 { return value<<8 | value>>8 }
