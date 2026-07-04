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

type WiFiStatus struct {
	Config     config.WiFiConfig `json:"config"`
	Interface  string            `json:"interface"`
	Available  bool              `json:"available"`
	Connected  bool              `json:"connected"`
	SSID       string            `json:"ssid,omitempty"`
	Signal     int               `json:"signal,omitempty"`
	IPv4       string            `json:"ipv4,omitempty"`
	Tool       string            `json:"tool,omitempty"`
	Message    string            `json:"message,omitempty"`
	Apply      *ApplyStatus      `json:"apply,omitempty"`
	Interfaces []string          `json:"interfaces,omitempty"`
}

type WiFiNetwork struct {
	SSID     string `json:"ssid"`
	Signal   int    `json:"signal,omitempty"`
	Security string `json:"security,omitempty"`
}

type WiFiScanResult struct {
	Interface string        `json:"interface"`
	Networks  []WiFiNetwork `json:"networks"`
	Tool      string        `json:"tool,omitempty"`
	Message   string        `json:"message,omitempty"`
}

type CellularStatus struct {
	Enabled    bool              `json:"enabled"`
	Available  bool              `json:"available"`
	CurrentIP  string            `json:"currentIp,omitempty"`
	DNS        []string          `json:"dns,omitempty"`
	Provider   string            `json:"provider,omitempty"`
	CardNumber string            `json:"cardNumber,omitempty"`
	Interfaces []InterfaceStatus `json:"interfaces,omitempty"`
	Modems     []string          `json:"modems,omitempty"`
	USBDevices []string          `json:"usbDevices,omitempty"`
	Serial     []string          `json:"serial,omitempty"`
	SIM        CellularSIMInfo   `json:"sim,omitempty"`
	Tool       string            `json:"tool,omitempty"`
	Raw        string            `json:"raw,omitempty"`
	DialLog    string            `json:"dialLog,omitempty"`
	Message    string            `json:"message,omitempty"`
}

type CellularSIMInfo struct {
	Available          bool   `json:"available"`
	Port               string `json:"port,omitempty"`
	Manufacturer       string `json:"manufacturer,omitempty"`
	Model              string `json:"model,omitempty"`
	Firmware           string `json:"firmware,omitempty"`
	IMEI               string `json:"imei,omitempty"`
	ICCID              string `json:"iccid,omitempty"`
	IMSI               string `json:"imsi,omitempty"`
	Operator           string `json:"operator,omitempty"`
	Signal             int    `json:"signal,omitempty"`
	SignalText         string `json:"signalText,omitempty"`
	PINStatus          string `json:"pinStatus,omitempty"`
	Registration       string `json:"registration,omitempty"`
	PacketRegistration string `json:"packetRegistration,omitempty"`
	AccessTechnology   string `json:"accessTechnology,omitempty"`
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

func WiFi(configured config.WiFiConfig) WiFiStatus {
	configured.ApplyDefaults()
	status := WiFiStatus{
		Config:     configured,
		Interface:  configured.Interface,
		Apply:      currentApplyStatus("wifi:" + configured.Interface),
		Interfaces: wirelessInterfaces(),
	}
	status.Available = containsString(status.Interfaces, configured.Interface)
	var current InterfaceStatus
	if current, err := interfaceStatusByName(configured.Interface); err == nil {
		status.IPv4 = current.IPv4Address
		status.Available = status.Available || current.Name != ""
	}
	if ssid, signal, ok := wifiLink(configured.Interface); ok {
		status.SSID = ssid
		status.Signal = signal
		status.Connected = true
		status.Tool = "iw"
	}
	if !status.Connected {
		if ssid, err := commandOutput("iwgetid", configured.Interface, "-r"); err == nil && strings.TrimSpace(ssid) != "" {
			status.SSID = strings.TrimSpace(ssid)
			status.Connected = true
			status.Tool = "iwgetid"
		}
	}
	if !status.Connected && configured.Enabled && status.IPv4 != "" && current.Carrier {
		status.SSID = configured.SSID
		status.Connected = true
		status.Tool = "interface"
	}
	if status.Signal == 0 {
		if signal := wifiSignal(configured.Interface); signal != 0 {
			status.Signal = signal
		}
	}
	if !status.Available {
		status.Message = "未检测到 WiFi 网卡 " + configured.Interface
	} else if !status.Connected {
		status.Message = "WiFi 网卡已检测到，但当前未连接"
	}
	return status
}

func ScanWiFi(name string) WiFiScanResult {
	if name == "" {
		name = "wlan0"
	}
	_ = exec.Command("ip", "link", "set", "dev", name, "up").Run()
	result := WiFiScanResult{Interface: name}
	if raw, err := commandOutput("iw", "dev", name, "scan"); err == nil {
		result.Tool = "iw"
		result.Networks = parseIWScan(raw)
		result = result.withMessage()
		if len(result.Networks) > 0 {
			return result
		}
	}
	if raw, err := commandOutput("iwlist", name, "scan"); err == nil {
		result.Tool = "iwlist"
		result.Networks = parseIWListScan(raw)
		result = result.withMessage()
		if len(result.Networks) > 0 {
			return result
		}
	}
	if raw, err := commandOutput("nmcli", "-t", "-f", "SSID,SIGNAL,SECURITY", "dev", "wifi", "list", "ifname", name); err == nil {
		result.Tool = "nmcli"
		result.Networks = parseNMCLIScan(raw)
		return result.withMessage()
	}
	result.Message = "无法扫描 WiFi 网络，请检查无线工具是否可用"
	return result
}

func (r WiFiScanResult) withMessage() WiFiScanResult {
	r.Networks = dedupeWiFiNetworks(r.Networks)
	if len(r.Networks) == 0 {
		r.Message = "未扫描到 WiFi 网络"
	}
	return r
}

func Cellular(configured config.CellularConfig) CellularStatus {
	configured.ApplyDefaults()
	status := CellularStatus{Enabled: configured.IsEnabled(), DialLog: cellularDialLog()}
	if !status.Enabled {
		status.Message = "移动网络未启用"
		return status
	}
	if raw, err := commandOutput("mmcli", "-L"); err == nil {
		status.Tool = "mmcli"
		status.Raw = strings.TrimSpace(raw)
		status.Modems = parseMMCLIModems(raw)
		status.Available = len(status.Modems) > 0 || strings.Contains(strings.ToLower(raw), "modem")
		if status.Available && len(status.Modems) > 0 {
			if detail, err := commandOutput("mmcli", "-m", status.Modems[0]); err == nil {
				status.Raw = strings.TrimSpace(detail)
			}
		}
	}
	if raw, err := commandOutput("lsusb"); err == nil {
		status.USBDevices = parseCellularUSBDevices(raw)
	}
	status.Serial = cellularSerialDevices()
	if sim, raw, ok := detectCellularSIM(status.Serial); ok {
		status.SIM = sim
		if status.Raw == "" {
			status.Raw = strings.TrimSpace(raw)
		}
		status.Available = true
		if status.Tool == "" {
			status.Tool = "AT"
		}
	}
	interfaces, _ := Interfaces()
	for _, item := range interfaces {
		if isCellularInterfaceName(strings.ToLower(item.Name)) {
			status.Interfaces = append(status.Interfaces, item)
			if status.CurrentIP == "" {
				status.CurrentIP = item.IPv4Address
			}
			if len(status.DNS) == 0 {
				status.DNS = item.DNS
			}
		}
	}
	status.Provider = status.SIM.Operator
	status.CardNumber = status.SIM.ICCID
	if len(status.DNS) == 0 {
		status.DNS = readDNS()
	}
	if len(status.Interfaces) > 0 || len(status.USBDevices) > 0 || len(status.Serial) > 0 {
		status.Available = true
	}
	if !status.Available {
		status.Message = "未检测到移动网络模组、SIM 卡或网卡"
	} else if status.Tool == "" {
		status.Message = "已检测到移动网络硬件，但未检测到 mmcli 或 AT 识别信息"
	}
	return status
}

func ApplyCellular(configured config.CellularConfig) CellularStatus {
	configured.ApplyDefaults()
	logPath := cellularDialLogPath()
	if configured.IsEnabled() {
		appendCellularDialLog(logPath, "cellular enabled")
		return RedialCellular(configured)
	}
	iface := configured.Interface
	appendCellularDialLog(logPath, "cellular disabled")
	output, err := commandCombinedOutput("sh", "-c", fmt.Sprintf("ip link set %s down 2>&1 || true\nip addr show %s 2>&1 || true", shellWord(iface), shellWord(iface)))
	if err != nil {
		appendCellularDialLog(logPath, "disable command error: "+err.Error())
	}
	appendCellularDialLog(logPath, strings.TrimSpace(output))
	return Cellular(configured)
}

func RedialCellular(configured config.CellularConfig) CellularStatus {
	configured.ApplyDefaults()
	logPath := cellularDialLogPath()
	if !configured.IsEnabled() {
		appendCellularDialLog(logPath, "redial skipped: cellular disabled")
		return Cellular(configured)
	}
	appendCellularDialLog(logPath, "redial requested")
	iface := configured.Interface
	if iface == "" {
		iface = firstCellularInterfaceName()
	}
	if iface == "" {
		iface = "usbeth0"
	}
	script := fmt.Sprintf(`set -x
date
ip link set %s down 2>/dev/null || true
sleep 1
ip link set %s up 2>/dev/null || true
if command -v ifdown >/dev/null 2>&1; then ifdown %s 2>&1 || true; fi
if command -v ifup >/dev/null 2>&1; then ifup %s 2>&1 || true; fi
if command -v udhcpc >/dev/null 2>&1; then udhcpc -i %s -n -q -t 3 -T 5 2>&1 || true; fi
ip addr show %s 2>&1 || ifconfig %s 2>&1 || true
`, shellWord(iface), shellWord(iface), shellWord(iface), shellWord(iface), shellWord(iface), shellWord(iface), shellWord(iface))
	output, err := commandCombinedOutput("sh", "-c", script)
	if err != nil {
		appendCellularDialLog(logPath, "redial command error: "+err.Error())
	}
	appendCellularDialLog(logPath, strings.TrimSpace(output))
	status := Cellular(configured)
	status.DialLog = cellularDialLog()
	return status
}
func ScheduleApplyWiFi(wifi config.WiFiConfig) error {
	wifi.ApplyDefaults()
	if err := ValidateWiFi(wifi); err != nil {
		return err
	}
	key := "wifi:" + wifi.Interface
	setApplyStatus(key, ApplyStatus{State: "pending", Message: "wifi configuration is scheduled", UpdatedAt: time.Now()})
	go func() {
		time.Sleep(time.Second)
		err := applyWiFi(wifi)
		status := ApplyStatus{State: "applied", Message: "wifi configuration applied", UpdatedAt: time.Now()}
		if err != nil {
			status.State = "error"
			status.Message = err.Error()
		}
		setApplyStatus(key, status)
	}()
	return nil
}

func ValidateWiFi(wifi config.WiFiConfig) error {
	if !interfaceNamePattern.MatchString(wifi.Interface) {
		return fmt.Errorf("invalid wifi interface %q", wifi.Interface)
	}
	if wifi.Mode != "static" && wifi.Mode != "dhcp" {
		return fmt.Errorf("wifi mode must be static or dhcp")
	}
	if wifi.Enabled && strings.TrimSpace(wifi.SSID) == "" {
		return fmt.Errorf("wifi ssid is required")
	}
	if wifi.Mode == "static" && wifi.Enabled {
		if net.ParseIP(wifi.IPAddress) == nil || strings.Contains(wifi.IPAddress, ":") {
			return fmt.Errorf("valid IPv4 address is required for static wifi mode")
		}
		if wifi.PrefixLength < 1 || wifi.PrefixLength > 32 {
			return fmt.Errorf("IPv4 prefix length must be between 1 and 32")
		}
		if wifi.Gateway != "" && net.ParseIP(wifi.Gateway) == nil {
			return fmt.Errorf("invalid IPv4 gateway")
		}
	}
	for _, server := range wifi.DNS {
		if net.ParseIP(server) == nil {
			return fmt.Errorf("invalid DNS server %q", server)
		}
	}
	return nil
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
	if port.Enabled && len(port.DNS) > 0 {
		if err := writeDNS(port.DNS); err != nil {
			return err
		}
	}
	return nil
}

func applyWiFi(wifi config.WiFiConfig) error {
	wpaPath := filepath.Join("/etc/wpa_supplicant", "wpa_supplicant-"+wifi.Interface+".conf")
	wpaWritePath := resolveWritePath(wpaPath)
	if wifi.Enabled {
		if err := writeAtomic(wpaWritePath, []byte(renderWPAConfig(wifi)), 0600); err != nil {
			return err
		}
	}
	port := config.NetworkPort{
		Name:         wifi.Interface,
		Interface:    wifi.Interface,
		Mode:         wifi.Mode,
		IPAddress:    wifi.IPAddress,
		PrefixLength: wifi.PrefixLength,
		Gateway:      wifi.Gateway,
		DNS:          wifi.DNS,
		Enabled:      wifi.Enabled,
	}
	path := filepath.Join("/etc/network/interfaces.d", wifi.Interface)
	old, oldErr := os.ReadFile(path)
	_ = exec.Command("/sbin/ifdown", wifi.Interface).Run()
	_ = exec.Command("ip", "addr", "flush", "dev", wifi.Interface).Run()
	_ = exec.Command("ip", "route", "flush", "dev", wifi.Interface).Run()
	if err := writeAtomic(path, []byte(renderWiFiInterfacesFile(port, wpaPath)), 0644); err != nil {
		return err
	}
	if !wifi.Enabled {
		return exec.Command("ip", "link", "set", "dev", wifi.Interface, "down").Run()
	}
	if err := exec.Command("/sbin/ifup", wifi.Interface).Run(); err != nil {
		if oldErr == nil {
			_ = writeAtomic(path, old, 0644)
			_ = exec.Command("/sbin/ifup", wifi.Interface).Run()
		}
		return fmt.Errorf("apply wifi %s failed: %w", wifi.Interface, err)
	}
	if wifi.Mode == "static" && len(wifi.DNS) > 0 {
		if err := writeDNS(wifi.DNS); err != nil {
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

func renderWiFiInterfacesFile(port config.NetworkPort, wpaPath string) string {
	if !port.Enabled {
		return fmt.Sprintf("iface %s inet manual\n", port.Interface)
	}
	lines := []string{"auto " + port.Interface}
	if port.Mode == "dhcp" {
		lines = append(lines, "iface "+port.Interface+" inet dhcp")
	} else {
		mask := net.CIDRMask(port.PrefixLength, 32)
		lines = append(lines,
			"iface "+port.Interface+" inet static",
			"address "+port.IPAddress,
			"netmask "+net.IP(mask).String(),
		)
		if port.Gateway != "" {
			lines = append(lines, "gateway "+port.Gateway)
		}
	}
	lines = append(lines, "wpa-conf "+wpaPath)
	return strings.Join(lines, "\n") + "\n"
}

func renderWPAConfig(wifi config.WiFiConfig) string {
	if wifi.Password == "" {
		return fmt.Sprintf("ctrl_interface=/var/run/wpa_supplicant\nap_scan=1\nupdate_config=1\n\nnetwork={\n    ssid=%q\n    key_mgmt=NONE\n}\n", wifi.SSID)
	}
	return fmt.Sprintf("ctrl_interface=/var/run/wpa_supplicant\nap_scan=1\nupdate_config=1\n\nnetwork={\n    ssid=%q\n    psk=%q\n    key_mgmt=WPA-PSK\n}\n", wifi.SSID, wifi.Password)
}

func resolveWritePath(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return path
	}
	if filepath.IsAbs(target) {
		return target
	}
	return filepath.Join(filepath.Dir(path), target)
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

func wirelessInterfaces() []string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}
	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if _, err := os.Stat(filepath.Join("/sys/class/net", name, "wireless")); err == nil {
			result = append(result, name)
			continue
		}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "wlan") || strings.HasPrefix(lower, "wl") {
			result = append(result, name)
		}
	}
	return result
}

func interfaceStatusByName(name string) (InterfaceStatus, error) {
	items, err := Interfaces()
	if err != nil {
		return InterfaceStatus{}, err
	}
	for _, item := range items {
		if item.Name == name {
			return item, nil
		}
	}
	return InterfaceStatus{}, fmt.Errorf("interface %s not found", name)
}

func commandOutput(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	raw, err := command.Output()
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func commandCombinedOutput(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	raw, err := command.CombinedOutput()
	return string(raw), err
}

func firstCellularInterfaceName() string {
	interfaces, _ := Interfaces()
	for _, item := range interfaces {
		if isCellularInterfaceName(strings.ToLower(item.Name)) {
			return item.Name
		}
	}
	return ""
}

func cellularDialLogPath() string {
	if _, err := os.Stat("/userdata/weikong/data"); err == nil {
		return "/userdata/weikong/data/cellular-dial.log"
	}
	return filepath.Join(os.TempDir(), "cellular-dial.log")
}

func appendCellularDialLog(path string, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "\n[%s]\n%s\n", time.Now().Format("2006-01-02 15:04:05"), text)
}

func cellularDialLog() string {
	raw, err := os.ReadFile(cellularDialLogPath())
	if err != nil {
		return ""
	}
	text := string(raw)
	if len(text) <= 8192 {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(text[len(text)-8192:])
}

func wifiLink(name string) (string, int, bool) {
	raw, err := commandOutput("iw", "dev", name, "link")
	if err != nil {
		return "", 0, false
	}
	return parseWiFiLink(raw)
}

func parseWiFiLink(raw string) (string, int, bool) {
	if !strings.Contains(raw, "Connected to") {
		return "", 0, false
	}
	var ssid string
	var signal int
	for _, line := range strings.Split(raw, "\n") {
		text := strings.TrimSpace(line)
		if strings.HasPrefix(text, "SSID:") {
			ssid = normalizeSSID(strings.TrimSpace(strings.TrimPrefix(text, "SSID:")))
		} else if strings.HasPrefix(text, "signal:") {
			fields := strings.Fields(strings.TrimPrefix(text, "signal:"))
			if len(fields) > 0 {
				value, _ := strconv.ParseFloat(fields[0], 64)
				signal = signalDBMToPercent(value)
			}
		}
	}
	return ssid, signal, true
}

func wifiSignal(name string) int {
	file, err := os.Open("/proc/net/wireless")
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, name+":") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, name+":"))
		if len(fields) < 3 {
			return 0
		}
		value, _ := strconv.ParseFloat(strings.Trim(fields[2], "."), 64)
		if value <= 0 {
			return signalDBMToPercent(value)
		}
		return int(value)
	}
	return 0
}

func detectCellularSIM(ports []string) (CellularSIMInfo, string, bool) {
	for _, port := range ports {
		if !strings.Contains(filepath.Base(port), "tty") {
			continue
		}
		raw, err := cellularATInfo(port)
		if err != nil || strings.TrimSpace(raw) == "" || !strings.Contains(raw, "OK") {
			continue
		}
		info := parseCellularATInfo(raw)
		info.Port = port
		info.Available = info.ICCID != "" || info.IMSI != "" || strings.Contains(strings.ToUpper(raw), "READY")
		if info.Available || info.IMEI != "" || info.Manufacturer != "" || info.Model != "" {
			return info, raw, true
		}
	}
	return CellularSIMInfo{}, "", false
}

func cellularATInfo(port string) (string, error) {
	if !interfaceNamePattern.MatchString(filepath.Base(port)) {
		return "", fmt.Errorf("invalid modem port %s", port)
	}
	commands := strings.Join([]string{
		"ATE1",
		"AT",
		"ATI",
		"AT+CGMI",
		"AT+CGMM",
		"AT+CGMR",
		"AT+CGSN",
		"AT+GSN",
		"AT+CPIN?",
		"AT+CCID",
		"AT+ICCID",
		"AT+QCCID",
		"AT+MCCID",
		"AT+CIMI",
		"AT+COPS?",
		"AT+CSQ",
		"AT+CREG?",
		"AT+CGREG?",
		"AT+CEREG?",
		"AT+QNWINFO",
	}, `\r\n`) + `\r\n`
	script := fmt.Sprintf(
		`out=$(mktemp); stty -F %[1]s 115200 raw -echo -echoe -echok 2>/dev/null || true; (cat %[1]s > "$out" & pid=$!; sleep 0.15; printf '%[2]s' > %[1]s; sleep 2; kill $pid 2>/dev/null || true; cat "$out"; rm -f "$out")`,
		shellQuote(port),
		commands,
	)
	return commandOutput("sh", "-c", script)
}

func parseCellularATInfo(raw string) CellularSIMInfo {
	info := CellularSIMInfo{}
	lines := cleanATLines(raw)
	currentCommand := ""
	atiValues := []string{}
	for index, line := range lines {
		upper := strings.ToUpper(line)
		if upper == "AT" || upper == "ATI" || strings.HasPrefix(upper, "ATE") || strings.HasPrefix(upper, "AT+") || strings.HasPrefix(upper, "AT^") || strings.HasPrefix(upper, "AT*") {
			currentCommand = upper
			continue
		}
		if strings.Contains(upper, "FIBOCOM") && info.Manufacturer == "" {
			info.Manufacturer = line
		}
		if info.Model == "" && strings.HasPrefix(upper, "LE") {
			info.Model = line
		}
		if info.Firmware == "" && strings.Count(line, ".") >= 2 && firstDigits(line) != "" {
			info.Firmware = line
		}
		if digits := firstDigits(line); len(digits) == 15 && strings.HasPrefix(digits, "86") && info.IMEI == "" {
			info.IMEI = digits
		}
		switch currentCommand {
		case "ATI":
			if !strings.Contains(upper, "FIBOCOM") && !strings.HasPrefix(upper, "V") {
				atiValues = append(atiValues, line)
			}
			if strings.Contains(upper, "FIBOCOM") && info.Manufacturer == "" {
				info.Manufacturer = line
			}
		case "AT+CGMI":
			if info.Manufacturer == "" {
				info.Manufacturer = line
			}
		case "AT+CGMM":
			if info.Model == "" {
				info.Model = line
			}
		case "AT+CGMR":
			if info.Firmware == "" {
				info.Firmware = line
			}
		case "AT+CGSN", "AT+GSN":
			if digits := firstDigits(line); len(digits) >= 10 && info.IMEI == "" {
				info.IMEI = digits
			}
		}
		switch {
		case strings.HasPrefix(upper, "+CPIN:"):
			info.PINStatus = strings.TrimSpace(strings.TrimPrefix(line, line[:strings.Index(line, ":")+1]))
		case strings.HasPrefix(upper, "+CCID:"):
			info.ICCID = firstDigits(line)
		case strings.HasPrefix(upper, "+ICCID:"):
			info.ICCID = firstDigits(line)
		case strings.HasPrefix(upper, "+QCCID:"):
			info.ICCID = firstDigits(line)
		case upper == "AT+CIMI" && index+1 < len(lines):
			if digits := firstDigits(lines[index+1]); len(digits) >= 10 {
				info.IMSI = digits
			}
		case currentCommand == "AT+CIMI" && isIMSILine(line):
			info.IMSI = firstDigits(line)
		case strings.HasPrefix(upper, "+COPS:"):
			info.Operator = parseQuotedField(line)
			info.AccessTechnology = parseAccessTechnology(line)
		case strings.HasPrefix(upper, "+CSQ:"):
			info.Signal, info.SignalText = parseCSQ(line)
		case strings.HasPrefix(upper, "+CREG:"):
			info.Registration = parseRegistration(line)
		case strings.HasPrefix(upper, "+CGREG:"), strings.HasPrefix(upper, "+CEREG:"):
			info.PacketRegistration = parseRegistration(line)
		case strings.HasPrefix(upper, "+QNWINFO:"):
			if value := parseQuotedField(line); value != "" {
				info.AccessTechnology = value
			}
		}
	}
	if info.Model == "" && len(atiValues) > 0 {
		info.Model = atiValues[0]
	}
	if info.Firmware == "" && len(atiValues) > 1 {
		info.Firmware = atiValues[1]
	}
	info.Available = info.ICCID != "" || info.IMSI != "" || strings.EqualFold(info.PINStatus, "READY")
	return info
}

func cleanATLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r", "\n")
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		text := strings.TrimSpace(strings.Trim(line, "\x00"))
		if text == "" || text == "OK" || strings.HasPrefix(text, "ERROR") {
			continue
		}
		lines = append(lines, text)
	}
	return lines
}

func isIMSILine(line string) bool {
	digits := firstDigits(line)
	return len(digits) >= 14 && len(digits) <= 16 && digits == strings.TrimSpace(line)
}

func firstDigits(value string) string {
	var out strings.Builder
	for _, item := range value {
		if item >= '0' && item <= '9' {
			out.WriteRune(item)
		} else if out.Len() > 0 {
			break
		}
	}
	return out.String()
}

func parseQuotedField(value string) string {
	start := strings.Index(value, `"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(value[start+1:], `"`)
	if end < 0 {
		return ""
	}
	return value[start+1 : start+1+end]
}

func parseAccessTechnology(value string) string {
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	code, err := strconv.Atoi(last)
	if err != nil {
		return ""
	}
	switch code {
	case 0:
		return "GSM"
	case 2:
		return "UTRAN"
	case 7:
		return "LTE"
	case 9:
		return "NB-IoT"
	default:
		return last
	}
}

func parseCSQ(value string) (int, string) {
	fields := strings.Split(strings.TrimSpace(strings.TrimPrefix(value[strings.Index(value, ":")+1:], " ")), ",")
	if len(fields) == 0 {
		return 0, ""
	}
	rssi, err := strconv.Atoi(strings.TrimSpace(fields[0]))
	if err != nil || rssi == 99 {
		return 0, "鏈煡"
	}
	dbm := -113 + 2*rssi
	percent := (rssi * 100) / 31
	if percent > 100 {
		percent = 100
	}
	return percent, fmt.Sprintf("%d%% (%d dBm)", percent, dbm)
}

func parseRegistration(value string) string {
	fields := strings.Split(strings.TrimSpace(strings.TrimPrefix(value[strings.Index(value, ":")+1:], " ")), ",")
	if len(fields) == 0 {
		return ""
	}
	codeText := strings.TrimSpace(fields[len(fields)-1])
	if len(fields) > 1 {
		codeText = strings.TrimSpace(fields[1])
	}
	code, err := strconv.Atoi(codeText)
	if err != nil {
		return codeText
	}
	switch code {
	case 0:
		return "\u672a\u6ce8\u518c"
	case 1:
		return "\u5df2\u6ce8\u518c\uff0c\u672c\u5730\u7f51\u7edc"
	case 2:
		return "\u6b63\u5728\u641c\u7d22"
	case 3:
		return "\u6ce8\u518c\u88ab\u62d2\u7edd"
	case 4:
		return "\u672a\u77e5"
	case 5:
		return "\u5df2\u6ce8\u518c\uff0c\u6f2b\u6e38\u7f51\u7edc"
	default:
		return codeText
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func shellWord(value string) string {
	return strings.ReplaceAll(shellQuote(value), "\n", "")
}

func parseMMCLIModems(raw string) []string {
	var result []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "/Modem/") {
			continue
		}
		parts := strings.Split(line, "/Modem/")
		if len(parts) < 2 {
			continue
		}
		id := strings.Fields(parts[1])
		if len(id) > 0 {
			result = append(result, strings.Trim(id[0], "[]"))
		}
	}
	return result
}

func parseCellularUSBDevices(raw string) []string {
	var result []string
	for _, line := range strings.Split(raw, "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.Contains(text, "1d6b:") {
			continue
		}
		lower := strings.ToLower(text)
		if strings.Contains(lower, "2cb7:") || strings.Contains(lower, "2c7c:") || strings.Contains(lower, "05c6:") || strings.Contains(lower, "12d1:") || strings.Contains(lower, "19d2:") || strings.Contains(lower, "1e0e:") || strings.Contains(lower, "a69c:") {
			result = append(result, text)
		}
	}
	return result
}

func cellularSerialDevices() []string {
	patterns := []string{"/dev/ttyUSB*", "/dev/cdc-wdm*", "/dev/qcqmi*", "/dev/modem*"}
	var result []string
	for _, pattern := range patterns {
		items, _ := filepath.Glob(pattern)
		for _, item := range items {
			result = append(result, item)
		}
	}
	return result
}

func isCellularInterfaceName(name string) bool {
	return strings.HasPrefix(name, "wwan") ||
		strings.HasPrefix(name, "ppp") ||
		strings.HasPrefix(name, "usb") ||
		strings.HasPrefix(name, "rmnet") ||
		strings.HasPrefix(name, "qmi") ||
		strings.HasPrefix(name, "ccmni") ||
		strings.HasPrefix(name, "wwp")
}

func parseIWScan(raw string) []WiFiNetwork {
	var result []WiFiNetwork
	var current WiFiNetwork
	flush := func() {
		if current.SSID != "" {
			result = append(result, current)
		}
		current = WiFiNetwork{}
	}
	for _, line := range strings.Split(raw, "\n") {
		text := strings.TrimSpace(line)
		if strings.HasPrefix(text, "BSS ") {
			flush()
			continue
		}
		if strings.HasPrefix(text, "SSID:") {
			current.SSID = normalizeSSID(strings.TrimSpace(strings.TrimPrefix(text, "SSID:")))
		} else if strings.HasPrefix(text, "signal:") {
			fields := strings.Fields(strings.TrimPrefix(text, "signal:"))
			if len(fields) > 0 {
				value, _ := strconv.ParseFloat(fields[0], 64)
				current.Signal = signalDBMToPercent(value)
			}
		} else if strings.Contains(text, "WPA:") || strings.Contains(text, "RSN:") {
			current.Security = "WPA"
		}
	}
	flush()
	return result
}

func parseIWListScan(raw string) []WiFiNetwork {
	var result []WiFiNetwork
	var current WiFiNetwork
	flush := func() {
		if current.SSID != "" {
			result = append(result, current)
		}
		current = WiFiNetwork{}
	}
	for _, line := range strings.Split(raw, "\n") {
		text := strings.TrimSpace(line)
		if strings.HasPrefix(text, "Cell ") {
			flush()
			continue
		}
		if strings.Contains(text, "ESSID:") {
			current.SSID = normalizeSSID(strings.Trim(strings.TrimPrefix(text[strings.Index(text, "ESSID:"):], "ESSID:"), `"`))
		} else if strings.Contains(text, "Quality=") {
			current.Signal = parseQualityPercent(text)
		} else if strings.Contains(text, "Encryption key:on") {
			current.Security = "WPA"
		}
	}
	flush()
	return result
}

func parseNMCLIScan(raw string) []WiFiNetwork {
	var result []WiFiNetwork
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		network := WiFiNetwork{SSID: normalizeSSID(strings.TrimSpace(parts[0]))}
		if len(parts) > 1 {
			network.Signal, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
		if len(parts) > 2 {
			network.Security = strings.TrimSpace(parts[2])
		}
		result = append(result, network)
	}
	return result
}

func dedupeWiFiNetworks(items []WiFiNetwork) []WiFiNetwork {
	bySSID := map[string]WiFiNetwork{}
	for _, item := range items {
		if item.SSID == "" || strings.Trim(item.SSID, "\x00") == "" {
			continue
		}
		current, ok := bySSID[item.SSID]
		if !ok || item.Signal > current.Signal {
			bySSID[item.SSID] = item
		}
	}
	result := make([]WiFiNetwork, 0, len(bySSID))
	for _, item := range bySSID {
		result = append(result, item)
	}
	return result
}

func normalizeSSID(value string) string {
	if value == "" {
		return ""
	}
	decoded := decodeHexEscapes(value)
	if strings.Trim(decoded, "\x00 ") == "" {
		return ""
	}
	return decoded
}

func decodeHexEscapes(value string) string {
	if !strings.Contains(value, `\x`) {
		return value
	}
	out := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if i+3 < len(value) && value[i] == '\\' && value[i+1] == 'x' {
			if b, ok := parseHexByte(value[i+2], value[i+3]); ok {
				out = append(out, b)
				i += 3
				continue
			}
		}
		out = append(out, value[i])
	}
	return string(out)
}

func parseHexByte(a, b byte) (byte, bool) {
	high, ok := hexNibble(a)
	if !ok {
		return 0, false
	}
	low, ok := hexNibble(b)
	if !ok {
		return 0, false
	}
	return high<<4 | low, true
}

func hexNibble(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func parseQualityPercent(text string) int {
	index := strings.Index(text, "Quality=")
	if index < 0 {
		return 0
	}
	value := strings.Fields(strings.TrimPrefix(text[index:], "Quality="))
	if len(value) == 0 {
		return 0
	}
	parts := strings.Split(value[0], "/")
	if len(parts) != 2 {
		return 0
	}
	left, _ := strconv.Atoi(parts[0])
	right, _ := strconv.Atoi(parts[1])
	if right <= 0 {
		return 0
	}
	return left * 100 / right
}

func signalDBMToPercent(dbm float64) int {
	if dbm <= -100 {
		return 0
	}
	if dbm >= -50 {
		return 100
	}
	return int(2 * (dbm + 100))
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
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
