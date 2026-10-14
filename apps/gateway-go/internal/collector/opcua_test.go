package collector

import (
	"testing"

	"github.com/gopcua/opcua/ua"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestOPCUAEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "opc.tcp://127.0.0.1:4840"},
		{"192.168.1.10", "opc.tcp://192.168.1.10:4840"},
		{"192.168.1.10:49320", "opc.tcp://192.168.1.10:49320"},
		{"opc.tcp://plc.local:4840/server", "opc.tcp://plc.local:4840/server"},
	}
	for _, test := range tests {
		got, err := opcuaEndpoint(test.input)
		if err != nil {
			t.Fatalf("opcuaEndpoint(%q) error = %v", test.input, err)
		}
		if got != test.want {
			t.Errorf("opcuaEndpoint(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestOPCUAEndpointRejectsInvalidScheme(t *testing.T) {
	if _, err := opcuaEndpoint("http://plc.local:4840"); err == nil {
		t.Fatal("opcuaEndpoint() accepted an HTTP URL")
	}
}

func TestApplyOPCUAScale(t *testing.T) {
	point := config.PointConfig{Scale: 0.1, Offset: 2}
	if got := applyOPCUAScale(point, int16(120)); got != float64(14) {
		t.Fatalf("applyOPCUAScale() = %#v, want 14", got)
	}
	if got := applyOPCUAScale(point, true); got != true {
		t.Fatalf("applyOPCUAScale() changed a boolean: %#v", got)
	}
}

func TestSelectOPCUAEndpointByAuthentication(t *testing.T) {
	anonymous := &ua.EndpointDescription{
		EndpointURL:       "opc.tcp://plc:4840",
		SecurityMode:      ua.MessageSecurityModeNone,
		SecurityPolicyURI: ua.SecurityPolicyURINone,
		UserIdentityTokens: []*ua.UserTokenPolicy{{
			PolicyID:  "anonymous",
			TokenType: ua.UserTokenTypeAnonymous,
		}},
	}
	username := &ua.EndpointDescription{
		EndpointURL:       "opc.tcp://plc:4840",
		SecurityMode:      ua.MessageSecurityModeNone,
		SecurityPolicyURI: ua.SecurityPolicyURINone,
		UserIdentityTokens: []*ua.UserTokenPolicy{{
			PolicyID:  "username",
			TokenType: ua.UserTokenTypeUserName,
		}},
	}
	endpoints := []*ua.EndpointDescription{anonymous, username}
	if got := selectOPCUAEndpoint(endpoints, ua.UserTokenTypeUserName); got != username {
		t.Fatalf("selected endpoint = %#v, want username endpoint", got)
	}
}

func TestOPCUADataTypeName(t *testing.T) {
	if got := opcuaDataTypeName(ua.NewNumericNodeID(0, 11)); got != "float64" {
		t.Fatalf("opcuaDataTypeName(Double) = %q, want float64", got)
	}
	if got := opcuaDataTypeName(ua.NewNumericNodeID(2, 1001)); got != "auto" {
		t.Fatalf("opcuaDataTypeName(custom) = %q, want auto", got)
	}
}
