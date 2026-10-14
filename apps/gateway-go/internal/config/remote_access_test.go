package config

import "testing"

func TestRemoteAccessDefaultsAndValidation(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gw","remoteAccess":{"enabled":true,"endpoint":"wss://cloud.example.com/api/remote-access/tunnel"},"mqtt":{"enabled":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RemoteAccess.LocalURL != "http://127.0.0.1:8088" {
		t.Fatalf("localUrl = %q", cfg.RemoteAccess.LocalURL)
	}
}

func TestRemoteAccessRejectsUnsafeLocalTarget(t *testing.T) {
	_, err := Parse([]byte(`{"gatewayKey":"gw","remoteAccess":{"enabled":true,"endpoint":"wss://cloud.example.com/api/remote-access/tunnel","localUrl":"http://192.168.1.20:8088"},"mqtt":{"enabled":false}}`))
	if err == nil {
		t.Fatal("expected unsafe local target to be rejected")
	}
}

func TestRemoteAccessKeepsCloudAndLoopbackTLSPoliciesSeparate(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gw","remoteAccess":{"enabled":true,"endpoint":"wss://cloud.example.com/api/remote-access/tunnel","localUrl":"https://127.0.0.1:8088","insecureSkipVerify":false,"localInsecureSkipVerify":true},"mqtt":{"enabled":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RemoteAccess.InsecureSkipVerify {
		t.Fatal("public cloud certificate verification must remain enabled")
	}
	if !cfg.RemoteAccess.LocalInsecureSkipVerify {
		t.Fatal("loopback self-signed certificate option was not preserved")
	}
}
