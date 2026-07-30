package networking

import (
	"path/filepath"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestRenderStaticInterfaceFile(t *testing.T) {
	content := renderInterfacesFile(config.NetworkPort{
		Interface:    "eth1",
		Mode:         "static",
		IPAddress:    "192.168.2.10",
		PrefixLength: 24,
		Gateway:      "192.168.2.1",
		Enabled:      true,
	})
	for _, expected := range []string{
		"auto eth1",
		"iface eth1 inet static",
		"address 192.168.2.10",
		"netmask 255.255.255.0",
		"gateway 192.168.2.1",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("rendered config missing %q:\n%s", expected, content)
		}
	}
}

func TestRenderDHCPInterfaceFile(t *testing.T) {
	content := renderInterfacesFile(config.NetworkPort{Interface: "eth0", Mode: "dhcp", Enabled: true})
	if content != "auto eth0\niface eth0 inet dhcp\n" {
		t.Fatalf("unexpected DHCP config: %q", content)
	}
}

func TestRenderWiFiInterfaceFile(t *testing.T) {
	content := renderWiFiInterfacesFile(config.NetworkPort{Interface: "wlan0", Mode: "dhcp", Enabled: true}, "/etc/wpa_supplicant/wpa_supplicant-wlan0.conf")
	for _, expected := range []string{
		"auto wlan0",
		"iface wlan0 inet dhcp",
		"wpa-conf /etc/wpa_supplicant/wpa_supplicant-wlan0.conf",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("rendered wifi config missing %q:\n%s", expected, content)
		}
	}
}

func TestWirelessBootMarker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "wireless-enabled")
	t.Setenv("GATEWAY_WIRELESS_MARKER", marker)
	if WirelessBootEnabled() {
		t.Fatal("wireless boot unexpectedly enabled")
	}
	if err := SetWirelessBootEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !WirelessBootEnabled() {
		t.Fatal("wireless boot marker was not enabled")
	}
	if err := SetWirelessBootEnabled(false); err != nil {
		t.Fatal(err)
	}
	if WirelessBootEnabled() {
		t.Fatal("wireless boot marker was not removed")
	}
}

func TestRenderWPAConfigUsesEG100CompatibleFormat(t *testing.T) {
	content := renderWPAConfig(config.WiFiConfig{SSID: "wkjishubu", Password: "wk123456"})
	for _, expected := range []string{
		"ctrl_interface=/var/run/wpa_supplicant",
		"ap_scan=1",
		`ssid="wkjishubu"`,
		`psk="wk123456"`,
		"key_mgmt=WPA-PSK",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("rendered wpa config missing %q:\n%s", expected, content)
		}
	}
}

func TestValidateRejectsUnsafeInterfaceName(t *testing.T) {
	err := Validate(config.NetworkPort{Interface: "eth0; reboot", Mode: "dhcp", Enabled: true})
	if err == nil {
		t.Fatal("Validate() accepted unsafe interface name")
	}
}

func TestParseIWScan(t *testing.T) {
	raw := `BSS 00:11:22:33:44:55(on wlan0)
	signal: -55.00 dBm
	SSID: PlantWiFi
	RSN:	 * Version: 1
BSS 00:11:22:33:44:66(on wlan0)
	signal: -82.00 dBm
	SSID: Guest`
	items := parseIWScan(raw)
	if len(items) != 2 {
		t.Fatalf("expected 2 networks, got %d: %#v", len(items), items)
	}
	if items[0].SSID != "PlantWiFi" || items[0].Security != "WPA" || items[0].Signal != 90 {
		t.Fatalf("unexpected first network: %#v", items[0])
	}
	if items[1].SSID != "Guest" || items[1].Signal != 36 {
		t.Fatalf("unexpected second network: %#v", items[1])
	}
}

func TestParseIWListScan(t *testing.T) {
	raw := `Cell 01 - Address: 00:11:22:33:44:55
                    Quality=42/70  Signal level=-68 dBm
                    Encryption key:on
                    ESSID:"Factory"
          Cell 02 - Address: 00:11:22:33:44:66
                    Quality=55/70  Signal level=-54 dBm
                    Encryption key:off
                    ESSID:"Guest"`
	items := parseIWListScan(raw)
	if len(items) != 2 {
		t.Fatalf("expected 2 networks, got %d: %#v", len(items), items)
	}
	if items[0].SSID != "Factory" || items[0].Security != "WPA" || items[0].Signal != 60 {
		t.Fatalf("unexpected first network: %#v", items[0])
	}
	if items[1].SSID != "Guest" || items[1].Signal != 78 {
		t.Fatalf("unexpected second network: %#v", items[1])
	}
}

func TestNormalizeSSIDDecodesHexEscapedUTF8(t *testing.T) {
	if got := normalizeSSID(`\xe4\xb8\xad\xe6\x96\xb0\xe6\x80\x9d\xe5\xad\xa6-1`); got != "中新思学-1" {
		t.Fatalf("unexpected decoded ssid: %q", got)
	}
	if got := normalizeSSID(`DIRECT-C0-HP\x20`); got != "DIRECT-C0-HP " {
		t.Fatalf("unexpected space decoded ssid: %q", got)
	}
	if got := normalizeSSID(`\x00\x00\x00`); got != "" {
		t.Fatalf("hidden ssid should be empty, got %q", got)
	}
}

func TestParseWiFiLink(t *testing.T) {
	raw := `Connected to d4:35:38:b9:1a:48 (on wlan0)
	SSID: wkjishubu
	freq: 2412
	signal: -39 dBm`
	ssid, signal, ok := parseWiFiLink(raw)
	if !ok {
		t.Fatal("expected connected link")
	}
	if ssid != "wkjishubu" || signal != 100 {
		t.Fatalf("unexpected link parse result: ssid=%q signal=%d", ssid, signal)
	}
}

func TestParseCellularUSBDevices(t *testing.T) {
	raw := `Bus 001 Device 001: ID 1d6b:0002
Bus 001 Device 105: ID 2cb7:0d01
Bus 001 Device 102: ID a69c:88dc`
	items := parseCellularUSBDevices(raw)
	if len(items) != 2 {
		t.Fatalf("expected 2 cellular usb hints, got %d: %#v", len(items), items)
	}
}

func TestParseCellularATInfo(t *testing.T) {
	raw := `ATI
Fibocom Wireless Inc.
LE370-CN
12007.6002.00.02.09.03
V1.2

OK
AT+CGSN
869762089344837

OK
AT+CPIN?
+CPIN: READY

OK
AT+CCID
+CCID: 89860412345678901234

OK
AT+CIMI
460041234567890

OK
+COPS: 0,0,"CHN-UNICOM",7
+CSQ: 25,99
+CREG: 0,1
+CGREG: 0,5`
	info := parseCellularATInfo(raw)
	if !info.Available {
		t.Fatal("expected SIM to be available")
	}
	if info.Manufacturer != "Fibocom Wireless Inc." || info.Model != "LE370-CN" || info.IMEI != "869762089344837" {
		t.Fatalf("module info = %#v", info)
	}
	if info.ICCID != "89860412345678901234" {
		t.Fatalf("ICCID = %q", info.ICCID)
	}
	if info.IMSI != "460041234567890" {
		t.Fatalf("IMSI = %q", info.IMSI)
	}
	if info.Operator != "CHN-UNICOM" || info.AccessTechnology != "LTE" {
		t.Fatalf("operator/access = %q/%q", info.Operator, info.AccessTechnology)
	}
	if info.Signal == 0 || info.SignalText == "" {
		t.Fatalf("signal not parsed: %#v", info)
	}
	if info.Registration != "\u5df2\u6ce8\u518c\uff0c\u672c\u5730\u7f51\u7edc" || info.PacketRegistration != "\u5df2\u6ce8\u518c\uff0c\u6f2b\u6e38\u7f51\u7edc" {
		t.Fatalf("registration = %q/%q", info.Registration, info.PacketRegistration)
	}
}
