//go:build linux

package collector

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/goose"
	"weikong-iot-platform/apps/gateway-go/internal/packetmonitor"
)

type gooseSubscription struct {
	key            string
	deviceKey      string
	interfaceName  string
	goCBRef        string
	appID          uint16
	destinationMAC net.HardwareAddr
	socket         *goose.Socket
	cancel         context.CancelFunc
	done           chan struct{}
	ready          chan struct{}
	readyOnce      sync.Once

	mu         sync.RWMutex
	message    goose.Message
	receivedAt time.Time
	lastError  error
}

var gooseSubscriptions = struct {
	sync.Mutex
	items map[string]*gooseSubscription
}{items: map[string]*gooseSubscription{}}

func readIEC61850GOOSEPoint(ctx context.Context, point config.PointConfig) (interface{}, error) {
	subscription, err := getGOOSESubscription(point)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	subscription.mu.RLock()
	message := subscription.message
	receivedAt := subscription.receivedAt
	lastError := subscription.lastError
	subscription.mu.RUnlock()
	if receivedAt.IsZero() {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-subscription.ready:
			subscription.mu.RLock()
			message = subscription.message
			receivedAt = subscription.receivedAt
			lastError = subscription.lastError
			subscription.mu.RUnlock()
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			if lastError != nil {
				return nil, lastError
			}
			return nil, fmt.Errorf("waiting for IEC61850 GOOSE message %s", point.GoCBRef)
		}
	}
	ttl := time.Duration(message.TimeAllowedToLive) * time.Millisecond
	if ttl <= 0 {
		ttl = 2 * time.Second
	}
	if time.Since(receivedAt) > ttl {
		return nil, fmt.Errorf("IEC61850 GOOSE message expired %s ago", time.Since(receivedAt).Round(time.Millisecond))
	}
	if point.GooseIndex < 0 || point.GooseIndex >= len(message.Values) {
		return nil, fmt.Errorf("IEC61850 GOOSE data set index %d is outside 0..%d", point.GooseIndex, len(message.Values)-1)
	}
	return coerceGOOSEValue(message.Values[point.GooseIndex].Value, point.DataType), nil
}

func getGOOSESubscription(point config.PointConfig) (*gooseSubscription, error) {
	mac, err := net.ParseMAC(strings.TrimSpace(point.DestinationMAC))
	if err != nil || len(mac) != 6 {
		return nil, fmt.Errorf("invalid IEC61850 GOOSE destination MAC %q", point.DestinationMAC)
	}
	key := strings.Join([]string{
		strings.TrimSpace(point.Address),
		strings.TrimSpace(point.GoCBRef),
		fmt.Sprintf("%04x", point.AppID),
		strings.ToLower(mac.String()),
	}, "|")

	gooseSubscriptions.Lock()
	defer gooseSubscriptions.Unlock()
	if existing := gooseSubscriptions.items[key]; existing != nil {
		return existing, nil
	}
	socket, err := goose.Open(strings.TrimSpace(point.Address))
	if err != nil {
		return nil, err
	}
	if err := socket.JoinMulticast(mac); err != nil {
		_ = socket.Close()
		return nil, err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	subscription := &gooseSubscription{
		key:            key,
		deviceKey:      point.DeviceKey,
		interfaceName:  strings.TrimSpace(point.Address),
		goCBRef:        strings.TrimSpace(point.GoCBRef),
		appID:          point.AppID,
		destinationMAC: append(net.HardwareAddr(nil), mac...),
		socket:         socket,
		cancel:         cancel,
		done:           make(chan struct{}),
		ready:          make(chan struct{}),
	}
	gooseSubscriptions.items[key] = subscription
	go subscription.receive(runCtx)
	return subscription, nil
}

func (s *gooseSubscription) receive(ctx context.Context) {
	defer close(s.done)
	for {
		frame, err := s.socket.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.setError(fmt.Errorf("receive IEC61850 GOOSE on %s: %w", s.interfaceName, err))
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if len(frame) < 14 || !equalMAC(frame[:6], s.destinationMAC) {
			continue
		}
		message, err := goose.DecodeFrame(frame)
		if err != nil || message.AppID != s.appID || message.GoCBRef != s.goCBRef {
			continue
		}
		if message.Test || message.NeedsCommission {
			continue
		}
		s.mu.Lock()
		s.message = message
		s.receivedAt = time.Now()
		s.lastError = nil
		s.mu.Unlock()
		s.readyOnce.Do(func() { close(s.ready) })
		packetmonitor.Record(packetmonitor.Frame{
			Protocol:  "iec61850-goose",
			Direction: "rx",
			DeviceKey: s.deviceKey,
			Address:   s.interfaceName,
			Summary: fmt.Sprintf(
				"GOOSE 接收 APPID=0x%04X stNum=%d sqNum=%d 数据集成员=%d",
				message.AppID, message.StateNumber, message.SequenceNumber, len(message.Values),
			),
		}, frame)
	}
}

func (s *gooseSubscription) setError(err error) {
	s.mu.Lock()
	s.lastError = err
	s.mu.Unlock()
}

func CloseIEC61850GOOSEConnections() {
	gooseSubscriptions.Lock()
	items := gooseSubscriptions.items
	gooseSubscriptions.items = map[string]*gooseSubscription{}
	gooseSubscriptions.Unlock()
	for _, subscription := range items {
		subscription.cancel()
		_ = subscription.socket.Close()
		<-subscription.done
	}
}

func equalMAC(left []byte, right net.HardwareAddr) bool {
	if len(left) < 6 || len(right) != 6 {
		return false
	}
	for index := 0; index < 6; index++ {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func coerceGOOSEValue(value interface{}, dataType string) interface{} {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "bool", "boolean":
		switch typed := value.(type) {
		case bool:
			return typed
		case int64:
			return typed != 0
		case uint64:
			return typed != 0
		case float64:
			return typed != 0
		}
	case "int8":
		return int8(gooseInt64(value))
	case "int16":
		return int16(gooseInt64(value))
	case "int32":
		return int32(gooseInt64(value))
	case "int64":
		return gooseInt64(value)
	case "uint8":
		return uint8(gooseUint64(value))
	case "uint16":
		return uint16(gooseUint64(value))
	case "uint32":
		return uint32(gooseUint64(value))
	case "uint64":
		return gooseUint64(value)
	case "float32":
		return float32(gooseFloat64(value))
	case "float64", "float":
		return gooseFloat64(value)
	case "string":
		return fmt.Sprint(value)
	}
	return value
}

func gooseInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	}
	return 0
}

func gooseUint64(value interface{}) uint64 {
	if signed := gooseInt64(value); signed > 0 {
		return uint64(signed)
	}
	switch typed := value.(type) {
	case uint:
		return uint64(typed)
	case uint8:
		return uint64(typed)
	case uint16:
		return uint64(typed)
	case uint32:
		return uint64(typed)
	case uint64:
		return typed
	}
	return 0
}

func gooseFloat64(value interface{}) float64 {
	switch typed := value.(type) {
	case float32:
		return float64(typed)
	case float64:
		return typed
	}
	return float64(gooseInt64(value))
}
