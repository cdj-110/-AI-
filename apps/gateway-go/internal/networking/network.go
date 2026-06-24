package networking

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type InterfaceStatus struct {
	Name           string       `json:"name"`
	HardwareAddr   string       `json:"hardwareAddress,omitempty"`
	Up             bool         `json:"up"`
	Carrier        bool         `json:"carrier"`
	SpeedMbps      int          `json:"speedMbps,omitempty"`
	Addresses      []string     `json:"addresses"`
	IPv4Address    string       `json:"ipv4Address,omitempty"`
	PrefixLength   int          `json:"prefixLength,omitempty"`
	Gateway        string       `json:"gateway,omitempty"`
	DNS            []string     `json:"dns,omitempty"`
	ConfiguredMode string       `json:"configuredMode,omitempty"`
	Apply          *ApplyStatus `json:"apply,omitempty"`
}

type ApplyStatus struct {
	State     string    `json:"state,omitempty"`
	Message   string    `json:"message,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

var applyState = struct {
	sync.RWMutex
	items map[string]ApplyStatus
}{items: map[string]ApplyStatus{}}

var interfaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func Interfaces() ([]InterfaceStatus, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	gatewayByInterface := defaultGateways()
	dns := readDNS()
	statuses := make([]InterfaceStatus, 0, len(interfaces))
	for _, item := range interfaces {
		if item.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := item.Addrs()
		status := InterfaceStatus{
			Name:           item.Name,
			HardwareAddr:   item.HardwareAddr.String(),
			Up:             item.Flags&net.FlagUp != 0,
			Carrier:        readInt(filepath.Join("/sys/class/net", item.Name, "carrier")) == 1,
			SpeedMbps:      readInt(filepath.Join("/sys/class/net", item.Name, "speed")),
			Addresses:      make([]string, 0, len(addresses)),
			Gateway:        gatewayByInterface[item.Name],
			DNS:            dns,
			ConfiguredMode: configuredMode(item.Name),
			Apply:          currentApplyStatus(item.Name),
		}
		for _, address := range addresses {
			text := address.String()
			status.Addresses = append(status.Addresses, text)
			ip, network, parseErr := net.ParseCIDR(text)
			if parseErr == nil && ip.To4() != nil && status.IPv4Address == "" {
				status.IPv4Address = ip.String()
				status.PrefixLength, _ = network.Mask.Size()
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func Validate(port config.NetworkPort) error {
	if !interfaceNamePattern.MatchString(port.Interface) {
		return fmt.Errorf("invalid network interface %q", port.Interface)
	}
	if _, err := net.InterfaceByName(port.Interface); err != nil {
		return fmt.Errorf("network interface %s not found", port.Interface)
	}
	if port.Mode != "static" && port.Mode != "dhcp" {
		return fmt.Errorf("network mode must be static or dhcp")
	}
	if port.Mode == "static" && port.Enabled {
		if net.ParseIP(port.IPAddress) == nil || strings.Contains(port.IPAddress, ":") {
			return fmt.Errorf("valid IPv4 address is required for static mode")
		}
		if port.PrefixLength < 1 || port.PrefixLength > 32 {
			return fmt.Errorf("IPv4 prefix length must be between 1 and 32")
		}
		if port.Gateway != "" && net.ParseIP(port.Gateway) == nil {
			return fmt.Errorf("invalid IPv4 gateway")
		}
	}
	for _, server := range port.DNS {
		if net.ParseIP(server) == nil {
			return fmt.Errorf("invalid DNS server %q", server)
		}
	}
	return nil
}

func ScheduleApply(port config.NetworkPort) error {
	if err := Validate(port); err != nil {
		return err
	}
	setApplyStatus(port.Interface, ApplyStatus{State: "pending", Message: "network configuration is scheduled", UpdatedAt: time.Now()})
	go func() {
		time.Sleep(time.Second)
		err := apply(port)
		status := ApplyStatus{State: "applied", Message: "network configuration applied", UpdatedAt: time.Now()}
		if err != nil {
			status.State = "error"
			status.Message = err.Error()
		}
		setApplyStatus(port.Interface, status)
	}()
	return nil
}

func apply(port config.NetworkPort) error {
	path := filepath.Join("/etc/network/interfaces.d", port.Interface)
	old, oldErr := os.ReadFile(path)
	_ = exec.Command("/sbin/ifdown", port.Interface).Run()
	_ = exec.Command("ip", "addr", "flush", "dev", port.Interface).Run()
	_ = exec.Command("ip", "route", "flush", "dev", port.Interface).Run()
	if err := writeAtomic(path, []byte(renderInterfacesFile(port)), 0644); err != nil {
		return err
	}
	if !port.Enabled {
		return exec.Command("ip", "link", "set", "dev", port.Interface, "down").Run()
	}
	if err := exec.Command("/sbin/ifup", port.Interface).Run(); err != nil {
		if oldErr == nil {
			_ = writeAtomic(path, old, 0644)
			_ = exec.Command("/sbin/ifup", port.Interface).Run()
		}
		return fmt.Errorf("apply %s failed: %w", port.Interface, err)
	}
	if port.Mode == "static" && len(port.DNS) > 0 {
		if err := writeDNS(port.DNS); err != nil {
			return err
		}
	}
	return nil
}

func renderInterfacesFile(port config.NetworkPort) string {
	if !port.Enabled {
		return fmt.Sprintf("iface %s inet manual\n", port.Interface)
	}
	if port.Mode == "dhcp" {
		return fmt.Sprintf("auto %s\niface %s inet dhcp\n", port.Interface, port.Interface)
	}
	mask := net.CIDRMask(port.PrefixLength, 32)
	lines := []string{
		"auto " + port.Interface,
		"iface " + port.Interface + " inet static",
		"address " + port.IPAddress,
		"netmask " + net.IP(mask).String(),
	}
	if port.Gateway != "" {
		lines = append(lines, "gateway "+port.Gateway)
	}
	return strings.Join(lines, "\n") + "\n"
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".network-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func writeDNS(servers []string) error {
	var builder strings.Builder
	builder.WriteString("# Managed by weikong-gateway\n")
	for _, server := range servers {
		builder.WriteString("nameserver ")
		builder.WriteString(server)
		builder.WriteByte('\n')
	}
	return writeAtomic("/etc/resolv.conf", []byte(builder.String()), 0644)
}

func configuredMode(name string) string {
	raw, err := os.ReadFile(filepath.Join("/etc/network/interfaces.d", name))
	if err != nil {
		return ""
	}
	text := string(raw)
	if strings.Contains(text, " inet dhcp") {
		return "dhcp"
	}
	if strings.Contains(text, " inet static") {
		return "static"
	}
	return "manual"
}

func defaultGateways() map[string]string {
	result := map[string]string{}
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return result
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		value, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			continue
		}
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, uint32(value))
		result[fields[0]] = net.IP(bytes).String()
	}
	return result
}

func readDNS() []string {
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	defer file.Close()
	var servers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "nameserver" {
			servers = append(servers, fields[1])
		}
	}
	return servers
}

func readInt(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	value, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	return value
}

func currentApplyStatus(name string) *ApplyStatus {
	applyState.RLock()
	defer applyState.RUnlock()
	status, ok := applyState.items[name]
	if !ok {
		return nil
	}
	return &status
}

func setApplyStatus(name string, status ApplyStatus) {
	applyState.Lock()
	defer applyState.Unlock()
	applyState.items[name] = status
}
