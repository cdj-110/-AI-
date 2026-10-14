package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

type httpsSettingsResponse struct {
	Enabled         bool   `json:"enabled"`
	RequiresRestart bool   `json:"requiresRestart,omitempty"`
	URL             string `json:"url,omitempty"`
	CertificateFile string `json:"certificateFile,omitempty"`
	ExpiresAt       string `json:"expiresAt,omitempty"`
	Fingerprint     string `json:"fingerprint,omitempty"`
	CertificateOK   bool   `json:"certificateOK"`
}

func (s *Server) httpsSettings(writer http.ResponseWriter, request *http.Request) {
	if !s.hasPermission(request, "users.manage") {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	switch request.Method {
	case http.MethodGet:
		s.writeHTTPSSettings(writer, request, false)
	case http.MethodPut:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16*1024)).Decode(&body); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		s.configMu.Lock()
		cfg, err := config.Load(s.configPath)
		if err == nil {
			if body.Enabled {
				cfg.Web.TLSCertFile, cfg.Web.TLSKeyFile, err = s.ensureSelfSignedCertificate(request)
			} else {
				cfg.Web.TLSCertFile = ""
				cfg.Web.TLSKeyFile = ""
			}
		}
		if err == nil {
			err = config.Save(s.configPath, cfg)
		}
		s.configMu.Unlock()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		s.appendAudit(auditEvent{Username: currentUsername(s, request), RemoteIP: requestRemoteIP(request), Method: request.Method, Path: request.URL.Path, Status: http.StatusOK, Result: "success", Detail: "https enabled=" + strings.ToLower(strings.TrimSpace(boolText(body.Enabled)))})
		s.writeHTTPSSettings(writer, request, true)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func currentUsername(s *Server, request *http.Request) string {
	user, _ := s.currentUser(request)
	return user.Username
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (s *Server) writeHTTPSSettings(writer http.ResponseWriter, request *http.Request, requiresRestart bool) {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	response := httpsSettingsResponse{
		Enabled:         cfg.Web.TLSCertFile != "" && cfg.Web.TLSKeyFile != "",
		RequiresRestart: requiresRestart,
		URL:             listenerURL(request, cfg.Web.Listen, cfg.Web.TLSCertFile != "" && cfg.Web.TLSKeyFile != ""),
		CertificateFile: cfg.Web.TLSCertFile,
	}
	if response.Enabled {
		if raw, readErr := os.ReadFile(cfg.Web.TLSCertFile); readErr == nil {
			if block, _ := pem.Decode(raw); block != nil {
				if certificate, parseErr := x509.ParseCertificate(block.Bytes); parseErr == nil {
					fingerprint := sha256.Sum256(certificate.Raw)
					response.CertificateOK = time.Now().Before(certificate.NotAfter)
					response.ExpiresAt = certificate.NotAfter.Format(time.RFC3339)
					response.Fingerprint = strings.ToUpper(hex.EncodeToString(fingerprint[:]))
				}
			}
		}
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(response)
}

func (s *Server) ensureSelfSignedCertificate(request *http.Request) (string, string, error) {
	directory := filepath.Join(filepath.Dir(s.configPath), ".runtime", "tls")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", "", err
	}
	certPath := filepath.Join(directory, "gateway-web.crt")
	keyPath := filepath.Join(directory, "gateway-web.key")
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return "", "", err
	}
	hostnames, addresses := certificateHosts(request)
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Weikong Edge Gateway", Organization: []string{"Weikong Edge Gateway"}},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.AddDate(5, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     hostnames,
		IPAddresses:  addresses,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", err
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}
	if err := writePrivateFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}), 0600); err != nil {
		return "", "", err
	}
	if err := writePrivateFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		return "", "", err
	}
	return certPath, keyPath, nil
}

func certificateHosts(request *http.Request) ([]string, []net.IP) {
	dnsSet := map[string]bool{"localhost": true}
	ipSet := map[string]net.IP{"127.0.0.1": net.ParseIP("127.0.0.1"), "::1": net.ParseIP("::1")}
	addHost := func(host string) {
		host = strings.TrimSpace(strings.Trim(host, "[]"))
		if host == "" {
			return
		}
		if ip := net.ParseIP(host); ip != nil {
			ipSet[ip.String()] = ip
		} else {
			dnsSet[host] = true
		}
	}
	host := request.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	addHost(host)
	if hostname, err := os.Hostname(); err == nil {
		addHost(hostname)
	}
	if interfaces, err := net.InterfaceAddrs(); err == nil {
		for _, address := range interfaces {
			if ip, _, parseErr := net.ParseCIDR(address.String()); parseErr == nil {
				addHost(ip.String())
			}
		}
	}
	dns := make([]string, 0, len(dnsSet))
	for name := range dnsSet {
		dns = append(dns, name)
	}
	addresses := make([]net.IP, 0, len(ipSet))
	for _, address := range ipSet {
		addresses = append(addresses, address)
	}
	return dns, addresses
}

func writePrivateFile(path string, content []byte, mode os.FileMode) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, content, mode); err != nil {
		return err
	}
	if err := os.Chmod(temporary, mode); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func listenerURL(request *http.Request, listen string, secure bool) string {
	host := request.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	port := "8088"
	if _, parsedPort, err := net.SplitHostPort(listen); err == nil && parsedPort != "" {
		port = parsedPort
	}
	scheme := "http"
	if secure {
		scheme = "https"
	}
	return scheme + "://" + net.JoinHostPort(strings.Trim(host, "[]"), port)
}
