package networking

import (
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

func TestValidateRejectsUnsafeInterfaceName(t *testing.T) {
	err := Validate(config.NetworkPort{Interface: "eth0; reboot", Mode: "dhcp", Enabled: true})
	if err == nil {
		t.Fatal("Validate() accepted unsafe interface name")
	}
}
