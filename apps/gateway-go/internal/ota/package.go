package ota

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const MaxArtifactSize int64 = 64 << 20
const MaxManifestSize = 4096

// TrustedPublicKeys is injected into the release binary at build time. No
// private signing key is ever stored on the gateway.
var TrustedPublicKeys = ""

type Manifest struct {
	FormatVersion int    `json:"formatVersion"`
	Model         string `json:"model"`
	Release       int64  `json:"release"`
	Version       string `json:"version"`
	ArtifactSHA   string `json:"artifactSha256"`
	ArtifactSize  int64  `json:"artifactSize"`
	KeyID         string `json:"keyId"`
	Signature     string `json:"signature"`
}

func (m Manifest) SignedPayload() []byte {
	return []byte(fmt.Sprintf("WKU1\n%d\n%s\n%d\n%s\n%s\n%d\n%s\n", m.FormatVersion, m.Model, m.Release, m.Version, m.ArtifactSHA, m.ArtifactSize, m.KeyID))
}

func (m Manifest) VerifySignature(keys map[string]string) error {
	if m.FormatVersion != 1 || m.Model != "EG100" || m.Release < 1 || m.Release > 999999999999 || len(m.Version) == 0 || len(m.Version) > 64 || m.ArtifactSize < 1 || m.ArtifactSize > MaxArtifactSize {
		return errors.New("unsupported OTA package metadata")
	}
	if len(m.ArtifactSHA) != 64 || strings.ToLower(m.ArtifactSHA) != m.ArtifactSHA {
		return errors.New("invalid OTA artifact digest")
	}
	if _, err := hex.DecodeString(m.ArtifactSHA); err != nil {
		return err
	}
	encoded := keys[m.KeyID]
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("OTA signing key is not trusted")
	}
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(key, m.SignedPayload(), signature) {
		return errors.New("OTA signature verification failed")
	}
	return nil
}

func ReadManifest(reader io.Reader) (Manifest, error) {
	var header [8]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return Manifest{}, err
	}
	if string(header[:4]) != "WKU1" {
		return Manifest{}, errors.New("invalid OTA package header")
	}
	length := binary.BigEndian.Uint32(header[4:])
	if length == 0 || length > MaxManifestSize {
		return Manifest{}, errors.New("invalid OTA manifest length")
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func VerifyFile(path string, keys map[string]string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	manifest, err := ReadManifest(file)
	if err != nil {
		return Manifest{}, err
	}
	if err := manifest.VerifySignature(keys); err != nil {
		return Manifest{}, err
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(file, MaxArtifactSize+1))
	if err != nil {
		return Manifest{}, err
	}
	if count != manifest.ArtifactSize || hex.EncodeToString(hash.Sum(nil)) != manifest.ArtifactSHA {
		return Manifest{}, errors.New("OTA artifact size or digest mismatch")
	}
	return manifest, nil
}

func TrustedKeys() (map[string]string, error) {
	var keys map[string]string
	if err := json.Unmarshal([]byte(TrustedPublicKeys), &keys); err != nil || len(keys) == 0 {
		return nil, errors.New("OTA trusted key ring is not configured")
	}
	return keys, nil
}
