package goose

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultSniffDuration = 60 * time.Second
	maxSniffDuration     = 5 * time.Minute
	maxSniffSources      = 256
)

type SniffExpected struct {
	DeviceKey      string `json:"deviceKey,omitempty"`
	InterfaceName  string `json:"interfaceName,omitempty"`
	DestinationMAC string `json:"destinationMac,omitempty"`
	AppID          uint16 `json:"appId,omitempty"`
	GoCBRef        string `json:"goCbRef,omitempty"`
	DataSetRef     string `json:"dataSetRef,omitempty"`
	VLANID         uint16 `json:"vlanId,omitempty"`
}

type SniffSource struct {
	Key               string    `json:"key"`
	SourceMAC         string    `json:"sourceMac"`
	DestinationMAC    string    `json:"destinationMac"`
	Tagged            bool      `json:"tagged"`
	VLANID            uint16    `json:"vlanId"`
	VLANPriority      uint8     `json:"vlanPriority"`
	AppID             uint16    `json:"appId"`
	GoCBRef           string    `json:"goCbRef"`
	DataSetRef        string    `json:"dataSetRef"`
	GoID              string    `json:"goId,omitempty"`
	StateNumber       uint32    `json:"stateNumber"`
	SequenceNumber    uint32    `json:"sequenceNumber"`
	ConfRev           uint32    `json:"confRev"`
	TimeAllowedToLive uint32    `json:"timeAllowedToLiveMilliseconds"`
	Test              bool      `json:"test"`
	NeedsCommission   bool      `json:"needsCommission"`
	Values            []Value   `json:"values"`
	FrameCount        uint64    `json:"frameCount"`
	SequenceGaps      uint64    `json:"sequenceGaps"`
	FramesPerSecond   float64   `json:"framesPerSecond"`
	FirstSeen         time.Time `json:"firstSeen"`
	LastSeen          time.Time `json:"lastSeen"`
	Matched           bool      `json:"matched"`
	Warnings          []string  `json:"warnings,omitempty"`
}

type SniffSnapshot struct {
	Active        bool          `json:"active"`
	InterfaceName string        `json:"interfaceName,omitempty"`
	StartedAt     time.Time     `json:"startedAt,omitempty"`
	ExpiresAt     time.Time     `json:"expiresAt,omitempty"`
	FrameCount    uint64        `json:"frameCount"`
	Expected      SniffExpected `json:"expected"`
	Sources       []SniffSource `json:"sources"`
	Error         string        `json:"error,omitempty"`
}

// Sniffer owns at most one short-lived, in-memory diagnostic session. It does
// not persist frames and therefore cannot consume gateway storage.
type Sniffer struct {
	mu            sync.Mutex
	active        bool
	interfaceName string
	startedAt     time.Time
	expiresAt     time.Time
	frameCount    uint64
	expected      SniffExpected
	sources       map[string]*SniffSource
	cancel        context.CancelFunc
	lastError     string
	sessionID     uint64
}

func NewSniffer() *Sniffer {
	return &Sniffer{sources: make(map[string]*SniffSource)}
}

func (s *Sniffer) Start(interfaceName string, duration time.Duration, expected SniffExpected) error {
	interfaceName = strings.TrimSpace(interfaceName)
	if interfaceName == "" {
		return fmt.Errorf("GOOSE sniff interface is required")
	}
	if duration <= 0 {
		duration = defaultSniffDuration
	}
	if duration > maxSniffDuration {
		duration = maxSniffDuration
	}
	socket, err := Open(interfaceName)
	if err != nil {
		return err
	}
	if err := socket.JoinAllMulticast(); err != nil {
		_ = socket.Close()
		return err
	}

	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		_ = socket.Close()
		return fmt.Errorf("a GOOSE sniff session is already running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	now := time.Now()
	s.active = true
	s.interfaceName = interfaceName
	s.startedAt = now
	s.expiresAt = now.Add(duration)
	s.frameCount = 0
	s.expected = expected
	s.sources = make(map[string]*SniffSource)
	s.cancel = cancel
	s.lastError = ""
	s.sessionID++
	sessionID := s.sessionID
	s.mu.Unlock()

	go s.receive(ctx, socket, sessionID)
	return nil
}

func (s *Sniffer) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.active = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Sniffer) Snapshot() SniffSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	result := SniffSnapshot{
		Active:        s.active,
		InterfaceName: s.interfaceName,
		StartedAt:     s.startedAt,
		ExpiresAt:     s.expiresAt,
		FrameCount:    s.frameCount,
		Expected:      s.expected,
		Error:         s.lastError,
		Sources:       make([]SniffSource, 0, len(s.sources)),
	}
	for _, stored := range s.sources {
		source := *stored
		source.Values = append([]Value(nil), stored.Values...)
		source.Warnings, source.Matched = diagnoseSniffSource(source, s.expected, now)
		result.Sources = append(result.Sources, source)
	}
	sort.Slice(result.Sources, func(left, right int) bool {
		if result.Sources[left].Matched != result.Sources[right].Matched {
			return result.Sources[left].Matched
		}
		return result.Sources[left].LastSeen.After(result.Sources[right].LastSeen)
	})
	return result
}

func (s *Sniffer) receive(ctx context.Context, socket *Socket, sessionID uint64) {
	defer socket.Close()
	defer func() {
		s.mu.Lock()
		if s.sessionID == sessionID {
			s.active = false
			s.cancel = nil
		}
		s.mu.Unlock()
	}()
	for {
		frame, err := socket.Receive(ctx)
		if err != nil {
			if ctx.Err() == nil {
				s.mu.Lock()
				s.lastError = err.Error()
				s.mu.Unlock()
			}
			return
		}
		message, err := DecodeFrame(frame)
		if err != nil {
			continue
		}
		sourceMAC, destinationMAC, tagged, vlanID, vlanPriority, ok := ethernetMetadata(frame)
		if !ok {
			continue
		}
		now := time.Now()
		key := strings.Join([]string{sourceMAC, destinationMAC, fmt.Sprintf("%04x", message.AppID), message.GoCBRef, message.DataSetRef}, "|")
		s.mu.Lock()
		s.frameCount++
		source := s.sources[key]
		if source == nil {
			if len(s.sources) >= maxSniffSources {
				s.mu.Unlock()
				continue
			}
			source = &SniffSource{Key: key, SourceMAC: sourceMAC, DestinationMAC: destinationMAC, FirstSeen: now}
			s.sources[key] = source
		} else if source.StateNumber == message.StateNumber && message.SequenceNumber > source.SequenceNumber+1 {
			source.SequenceGaps += uint64(message.SequenceNumber - source.SequenceNumber - 1)
		}
		source.Tagged = tagged
		source.VLANID = vlanID
		source.VLANPriority = vlanPriority
		source.AppID = message.AppID
		source.GoCBRef = message.GoCBRef
		source.DataSetRef = message.DataSetRef
		source.GoID = message.GoID
		source.StateNumber = message.StateNumber
		source.SequenceNumber = message.SequenceNumber
		source.ConfRev = message.ConfRev
		source.TimeAllowedToLive = message.TimeAllowedToLive
		source.Test = message.Test
		source.NeedsCommission = message.NeedsCommission
		source.Values = append(source.Values[:0], message.Values...)
		source.FrameCount++
		source.LastSeen = now
		elapsed := now.Sub(source.FirstSeen).Seconds()
		if elapsed > 0 {
			source.FramesPerSecond = float64(source.FrameCount) / elapsed
		}
		s.mu.Unlock()
	}
}

func ethernetMetadata(frame []byte) (source, destination string, tagged bool, vlanID uint16, priority uint8, ok bool) {
	if len(frame) < 14 {
		return "", "", false, 0, 0, false
	}
	destination = net.HardwareAddr(frame[:6]).String()
	source = net.HardwareAddr(frame[6:12]).String()
	if binary.BigEndian.Uint16(frame[12:14]) == 0x8100 {
		if len(frame) < 18 {
			return "", "", false, 0, 0, false
		}
		tci := binary.BigEndian.Uint16(frame[14:16])
		tagged = true
		vlanID = tci & 0x0fff
		priority = uint8((tci >> 13) & 7)
	}
	return source, destination, tagged, vlanID, priority, true
}

func diagnoseSniffSource(source SniffSource, expected SniffExpected, now time.Time) ([]string, bool) {
	var warnings []string
	if expected.DestinationMAC != "" && !strings.EqualFold(expected.DestinationMAC, source.DestinationMAC) {
		warnings = append(warnings, "目标 MAC 与设备配置不一致")
	}
	if expected.AppID != 0 && expected.AppID != source.AppID {
		warnings = append(warnings, "APPID 与设备配置不一致")
	}
	if expected.GoCBRef != "" && expected.GoCBRef != source.GoCBRef {
		warnings = append(warnings, "GoCBRef 与设备配置不一致")
	}
	if expected.DataSetRef != "" && expected.DataSetRef != source.DataSetRef {
		warnings = append(warnings, "DataSetRef 与设备配置不一致")
	}
	if expected.DeviceKey != "" && expected.VLANID != source.VLANID {
		warnings = append(warnings, "VLAN ID 与设备配置不一致")
	}
	if source.Test {
		warnings = append(warnings, "报文处于 Test 状态")
	}
	if source.NeedsCommission {
		warnings = append(warnings, "报文标记 NeedsCommission")
	}
	if source.SequenceGaps > 0 {
		warnings = append(warnings, fmt.Sprintf("检测到 %d 个序号缺口", source.SequenceGaps))
	}
	ttl := time.Duration(source.TimeAllowedToLive) * time.Millisecond
	if ttl > 0 && now.Sub(source.LastSeen) > ttl {
		warnings = append(warnings, "报文已超过 TTL，发布可能中断")
	}
	matched := expected.DeviceKey == "" || (expected.DestinationMAC == "" || strings.EqualFold(expected.DestinationMAC, source.DestinationMAC)) &&
		(expected.AppID == 0 || expected.AppID == source.AppID) &&
		(expected.GoCBRef == "" || expected.GoCBRef == source.GoCBRef) &&
		(expected.DataSetRef == "" || expected.DataSetRef == source.DataSetRef) &&
		expected.VLANID == source.VLANID
	return warnings, matched
}
