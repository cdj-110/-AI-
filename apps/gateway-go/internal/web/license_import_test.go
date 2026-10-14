package web

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/hardware"
)

func TestSignedActivationPreflightAndFailedImportPreserveWorkingFiles(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	oldRing := config.ActivationTrustedPublicKeys
	ring, _ := json.Marshal(map[string]string{"test": base64.StdEncoding.EncodeToString(public)})
	config.ActivationTrustedPublicKeys = string(ring)
	t.Cleanup(func() { config.ActivationTrustedPublicKeys = oldRing })
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	oldFile := filepath.Join(dir, "old-license.json")
	original := []byte(`{"gatewayKey":"test","productModel":"INSTALLED","hardwareProfile":{"ethernetPortCount":1,"rs232PortCount":0,"rs485PortCount":0,"canPortCount":0,"supports4G":false,"supportsWifi":false},"resources":[{"resourceKey":"网口1","name":"网口1","type":"network","enabled":true,"network":{"name":"网口1","interface":"eth0","mode":"static","ipAddress":"10.0.0.9","prefixLength":24,"enabled":true}}],"cloudConnectionEnabled":false,"activation":{"file":"old-license.json"},"mqtt":{"enabled":false}}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFile, []byte(`{"old":"working-license"}`), 0600); err != nil {
		t.Fatal(err)
	}
	id := hardware.ReadIdentity().ID
	if id == "" {
		id = "TEST-HARDWARE"
	}
	payload := fmt.Sprintf(`{"enabled":true,"licenseVersion":2,"keyId":"test","licenseSequence":1,"hardwareId":%q,"chipUsid":"","sn":"WK-TEST","deviceSecret":"test-secret","broker":"tcp://127.0.0.1:1883","cloudApi":"","remoteAccessEndpoint":"","issuedAt":%q,"expiresAt":%q,"features":["collection"],"productModel":"EG100","hardwareProfile":{"ethernetPortCount":2,"rs232PortCount":2,"rs485PortCount":2,"canPortCount":0,"supports4G":true,"supportsWifi":true},"nonce":"test-nonce"}`, id, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	var document map[string]interface{}
	_ = json.Unmarshal([]byte(payload), &document)
	document["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(payload)))
	content, _ := json.Marshal(document)
	server := &Server{configPath: path, onConfig: func(config.Config) error { return nil }}
	call := func(method string, content []byte) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"file": oldFile, "content": string(content)})
		request := httptest.NewRequest(method, "/api/activation", bytes.NewReader(body))
		response := httptest.NewRecorder()
		server.activationFile(response, request)
		return response
	}
	preview := call(http.MethodPost, content)
	if preview.Code != 200 {
		t.Fatalf("preflight: %d %s", preview.Code, preview.Body.String())
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, original) {
		t.Fatal("preflight changed config")
	}
	if _, err := os.Stat(filepath.Join(dir, ".license-state.json")); !os.IsNotExist(err) {
		t.Fatal("preflight committed sequence")
	}
	installed := call(http.MethodPut, content)
	if installed.Code != 200 {
		t.Fatalf("install: %d %s", installed.Code, installed.Body.String())
	}
	workingConfig, _ := os.ReadFile(path)
	working, err := server.readRawConfig()
	if err != nil || working.CloudConnectionEnabled == nil || *working.CloudConnectionEnabled {
		t.Fatalf("replacement overrode the administrator's disabled cloud switch: %v", err)
	}
	if working.ProductModel != "INSTALLED" || len(working.Resources) != 1 || working.Resources[0].Network == nil || working.Resources[0].Network.IPAddress != "10.0.0.9" {
		t.Fatalf("replacement license rebuilt the installed hardware config: %#v", working)
	}
	oldRaw, _ := os.ReadFile(oldFile)
	if string(oldRaw) != `{"old":"working-license"}` {
		t.Fatal("old file overwritten")
	}
	document["nonce"] = "tampered"
	tampered, _ := json.Marshal(document)
	failed := call(http.MethodPut, tampered)
	if failed.Code == 200 {
		t.Fatal("tampered file accepted")
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(after, workingConfig) {
		t.Fatal("failed import changed working pointer")
	}
	replay := call(http.MethodPut, content)
	if replay.Code != 200 {
		t.Fatalf("idempotent reimport: %s", replay.Body.String())
	}
}
