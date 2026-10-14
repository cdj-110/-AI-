package ota

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFileRejectsTamperingAndUntrustedKey(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("compressed-gateway-fixture")
	digest := sha256.Sum256(artifact)
	manifest := Manifest{FormatVersion: 1, Model: "EG100", Release: 2026092101, Version: "1.0.0", ArtifactSHA: hex.EncodeToString(digest[:]), ArtifactSize: int64(len(artifact)), KeyID: "test"}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, manifest.SignedPayload()))
	raw, _ := json.Marshal(manifest)
	header := make([]byte, 8)
	copy(header, "WKU1")
	binary.BigEndian.PutUint32(header[4:], uint32(len(raw)))
	path := filepath.Join(t.TempDir(), "test.wku")
	if err := os.WriteFile(path, append(append(header, raw...), artifact...), 0600); err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"test": base64.StdEncoding.EncodeToString(public)}
	if _, err := VerifyFile(path, keys); err != nil {
		t.Fatalf("valid package rejected: %v", err)
	}
	if _, err := VerifyFile(path, nil); err == nil {
		t.Fatal("untrusted key accepted")
	}
	file, _ := os.OpenFile(path, os.O_WRONLY, 0600)
	_, _ = file.WriteAt([]byte("X"), int64(len(header)+len(raw)))
	_ = file.Close()
	if _, err := VerifyFile(path, keys); err == nil {
		t.Fatal("tampered artifact accepted")
	}
}
