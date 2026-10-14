package remoteaccess

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const (
	maxBodyBytes      = 16 << 20
	maxMessageBytes   = 32 << 20
	tunnelReadTimeout = 75 * time.Second
)

type Agent struct {
	cfg                            config.RemoteAccessConfig
	gatewayKey, username, password string
	client                         *http.Client
	localMu                        sync.Mutex
	localScheme                    string
	writeMu                        sync.Mutex
	cloud                          *websocket.Conn
	streamsMu                      sync.Mutex
	streams                        map[string]*websocket.Conn
	sshStreamsMu                   sync.Mutex
	sshStreams                     map[string]*sshStream
	sshOpening                     map[string]*sshOpening
}

type sshOpening struct{ cancel context.CancelFunc }

type sshStream struct {
	client    *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	closeOnce sync.Once
}

func (s *sshStream) close() {
	s.closeOnce.Do(func() {
		_ = s.stdin.Close()
		_ = s.session.Close()
		_ = s.client.Close()
	})
}

type message struct {
	Type       string              `json:"type"`
	RequestID  string              `json:"requestId,omitempty"`
	StreamID   string              `json:"streamId,omitempty"`
	Method     string              `json:"method,omitempty"`
	Path       string              `json:"path,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	BodyBase64 string              `json:"bodyBase64,omitempty"`
	DataBase64 string              `json:"dataBase64,omitempty"`
	Status     int                 `json:"status,omitempty"`
	Error      string              `json:"error,omitempty"`
	Binary     bool                `json:"binary,omitempty"`
	Username   string              `json:"username,omitempty"`
	Password   string              `json:"password,omitempty"`
	Cols       int                 `json:"cols,omitempty"`
	Rows       int                 `json:"rows,omitempty"`
}

func New(cfg config.RemoteAccessConfig, gatewayKey, username, password string) *Agent {
	// Cloud and loopback TLS policies are deliberately independent. A field
	// gateway commonly uses a self-signed certificate on 127.0.0.1. The local
	// target is separately restricted to loopback, so accepting that certificate
	// never weakens verification for the public WSS endpoint.
	tlsConfig := loopbackTLSConfig()
	return &Agent{
		cfg: cfg, gatewayKey: gatewayKey, username: username, password: password,
		client: &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: tlsConfig},
			Timeout:   30 * time.Second,
			// Redirects and their Set-Cookie headers belong to the remote browser.
			// Following them inside the gateway would discard the intermediate
			// session cookie and leave a successful login stuck on the login page.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		streams: map[string]*websocket.Conn{}, sshStreams: map[string]*sshStream{}, sshOpening: map[string]*sshOpening{},
	}
}

func (a *Agent) Run(ctx context.Context) {
	if !a.cfg.Enabled {
		return
	}
	backoff := time.Second
	for ctx.Err() == nil {
		if err := a.runOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[REMOTE] tunnel disconnected: %v; retry in %s", err, backoff)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (a *Agent) runOnce(ctx context.Context) error {
	endpoint, err := url.Parse(a.cfg.Endpoint)
	if err != nil {
		return err
	}
	query := endpoint.Query()
	query.Set("gatewayKey", a.gatewayKey)
	endpoint.RawQuery = query.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: a.cfg.InsecureSkipVerify}} // #nosec G402 -- explicit commissioning option
	headers := http.Header{"X-Gateway-Username": []string{a.username}, "X-Gateway-Password": []string{a.password}}
	cloud, response, err := dialer.DialContext(ctx, endpoint.String(), headers)
	if err != nil {
		if response != nil {
			return fmt.Errorf("handshake status %s", response.Status)
		}
		return err
	}
	a.writeMu.Lock()
	a.cloud = cloud
	a.writeMu.Unlock()
	defer func() {
		a.writeMu.Lock()
		if a.cloud == cloud {
			a.cloud = nil
		}
		a.writeMu.Unlock()
		_ = cloud.Close()
		a.closeStreams()
	}()
	cloud.SetReadLimit(maxMessageBytes)
	refreshReadDeadline := keepCloudTunnelAlive(cloud, tunnelReadTimeout)
	_ = refreshReadDeadline()
	log.Printf("[REMOTE] outbound tunnel connected endpoint=%s", endpoint.Host)
	for {
		_, raw, err := cloud.ReadMessage()
		if err != nil {
			return err
		}
		// Any successfully received control or business message proves that the
		// cloud tunnel is alive. Refreshing here prevents an active SSH terminal
		// from being closed by the fixed read deadline.
		_ = refreshReadDeadline()
		var input message
		if json.Unmarshal(raw, &input) != nil {
			continue
		}
		switch input.Type {
		case "auth.ok":
		case "http.request":
			go a.handleHTTP(ctx, input)
		case "ws.open":
			go a.openWebSocket(ctx, input)
		case "ws.data":
			a.writeWebSocket(input)
		case "ws.close":
			a.closeStream(input.StreamID)
		case "ssh.open":
			a.startSSH(ctx, input)
		case "ssh.input":
			a.writeSSH(input)
		case "ssh.resize":
			a.resizeSSH(input)
		case "ssh.close":
			a.closeSSHStream(input.StreamID)
		}
	}
}

func keepCloudTunnelAlive(cloud *websocket.Conn, timeout time.Duration) func() error {
	refresh := func() error {
		return cloud.SetReadDeadline(time.Now().Add(timeout))
	}
	cloud.SetPongHandler(func(string) error { return refresh() })
	cloud.SetPingHandler(func(payload string) error {
		if err := refresh(); err != nil {
			return err
		}
		return cloud.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(5*time.Second))
	})
	return refresh
}

func (a *Agent) startSSH(parent context.Context, input message) {
	ctx, cancel := context.WithCancel(parent)
	opening := &sshOpening{cancel: cancel}
	a.sshStreamsMu.Lock()
	previous := a.sshOpening[input.StreamID]
	a.sshOpening[input.StreamID] = opening
	a.sshStreamsMu.Unlock()
	if previous != nil {
		previous.cancel()
	}
	go func() {
		defer func() {
			a.sshStreamsMu.Lock()
			if a.sshOpening[input.StreamID] == opening {
				delete(a.sshOpening, input.StreamID)
			}
			a.sshStreamsMu.Unlock()
			cancel()
		}()
		a.openSSH(ctx, input)
	}()
}

func (a *Agent) openSSH(ctx context.Context, input message) {
	username := strings.TrimSpace(input.Username)
	if input.StreamID == "" || username == "" || input.Password == "" {
		_ = a.send(message{Type: "ssh.error", StreamID: input.StreamID, Error: "请输入 SSH 账号和密码"})
		return
	}
	cols, rows := boundedTerminalSize(input.Cols, input.Rows)
	dialer := net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:22")
	if err != nil {
		_ = a.send(message{Type: "ssh.error", StreamID: input.StreamID, Error: "网关 SSH 服务未开启或暂不可用"})
		return
	}
	_ = connection.SetDeadline(time.Now().Add(12 * time.Second))
	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(input.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec G106 -- target is hard-coded to this gateway's loopback SSH service
		Timeout:         10 * time.Second,
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, "127.0.0.1:22", sshConfig)
	if err != nil {
		_ = connection.Close()
		_ = a.send(message{Type: "ssh.error", StreamID: input.StreamID, Error: "SSH 账号或密码错误"})
		return
	}
	_ = connection.SetDeadline(time.Time{})
	client := ssh.NewClient(clientConnection, channels, requests)
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		_ = a.send(message{Type: "ssh.error", StreamID: input.StreamID, Error: "无法创建 SSH 终端会话"})
		return
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		_ = a.send(message{Type: "ssh.error", StreamID: input.StreamID, Error: "无法连接 SSH 终端输入"})
		return
	}
	stdout, stdoutErr := session.StdoutPipe()
	stderr, stderrErr := session.StderrPipe()
	if stdoutErr != nil || stderrErr != nil {
		_ = session.Close()
		_ = client.Close()
		_ = a.send(message{Type: "ssh.error", StreamID: input.StreamID, Error: "无法连接 SSH 终端输出"})
		return
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err = session.RequestPty("xterm-256color", rows, cols, modes); err == nil {
		err = session.Shell()
	}
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		_ = a.send(message{Type: "ssh.error", StreamID: input.StreamID, Error: "网关拒绝创建交互式 SSH 终端"})
		return
	}
	if ctx.Err() != nil {
		_ = session.Close()
		_ = client.Close()
		return
	}
	stream := &sshStream{client: client, session: session, stdin: stdin}
	a.sshStreamsMu.Lock()
	if ctx.Err() != nil {
		a.sshStreamsMu.Unlock()
		stream.close()
		return
	}
	previous := a.sshStreams[input.StreamID]
	a.sshStreams[input.StreamID] = stream
	a.sshStreamsMu.Unlock()
	if previous != nil {
		previous.close()
	}
	if err = a.send(message{Type: "ssh.opened", StreamID: input.StreamID}); err != nil {
		a.closeSSHStream(input.StreamID)
		return
	}
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); a.pipeSSHOutput(input.StreamID, stdout) }()
	go func() { defer readers.Done(); a.pipeSSHOutput(input.StreamID, stderr) }()
	_ = session.Wait()
	readers.Wait()
	a.removeSSHStream(input.StreamID, stream)
	stream.close()
	_ = a.send(message{Type: "ssh.close", StreamID: input.StreamID})
}

func boundedTerminalSize(cols, rows int) (int, int) {
	if cols < 20 || cols > 500 {
		cols = 120
	}
	if rows < 5 || rows > 200 {
		rows = 32
	}
	return cols, rows
}

func (a *Agent) pipeSSHOutput(streamID string, reader io.Reader) {
	buffer := make([]byte, 32*1024)
	for {
		count, err := reader.Read(buffer)
		if count > 0 && a.send(message{Type: "ssh.data", StreamID: streamID, DataBase64: base64.StdEncoding.EncodeToString(buffer[:count])}) != nil {
			return
		}
		if err != nil {
			return
		}
	}
}

func (a *Agent) writeSSH(input message) {
	data, err := base64.StdEncoding.DecodeString(input.DataBase64)
	if err != nil || len(data) > 256*1024 {
		return
	}
	a.sshStreamsMu.Lock()
	stream := a.sshStreams[input.StreamID]
	a.sshStreamsMu.Unlock()
	if stream != nil {
		_, _ = stream.stdin.Write(data)
	}
}

func (a *Agent) resizeSSH(input message) {
	cols, rows := boundedTerminalSize(input.Cols, input.Rows)
	a.sshStreamsMu.Lock()
	stream := a.sshStreams[input.StreamID]
	a.sshStreamsMu.Unlock()
	if stream != nil {
		_ = stream.session.WindowChange(rows, cols)
	}
}

func (a *Agent) removeSSHStream(id string, expected *sshStream) {
	a.sshStreamsMu.Lock()
	if a.sshStreams[id] == expected {
		delete(a.sshStreams, id)
	}
	a.sshStreamsMu.Unlock()
}

func (a *Agent) closeSSHStream(id string) {
	a.sshStreamsMu.Lock()
	stream := a.sshStreams[id]
	opening := a.sshOpening[id]
	delete(a.sshStreams, id)
	delete(a.sshOpening, id)
	a.sshStreamsMu.Unlock()
	if opening != nil {
		opening.cancel()
	}
	if stream != nil {
		stream.close()
	}
}

func (a *Agent) handleHTTP(ctx context.Context, input message) {
	result := message{Type: "http.response", RequestID: input.RequestID, Headers: map[string][]string{}}
	defer func() { _ = a.send(result) }()
	target, err := a.resolvedLocalTarget(ctx, input.Path)
	if err != nil {
		result.Error = err.Error()
		return
	}
	body, err := base64.StdEncoding.DecodeString(input.BodyBase64)
	if err != nil {
		result.Error = "invalid request body"
		return
	}
	request, err := http.NewRequestWithContext(ctx, input.Method, target, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		return
	}
	copyHeaders(request.Header, input.Headers)
	request.Header.Del("Origin")
	request.Header.Del("Referer")
	response, err := a.client.Do(request)
	if err != nil {
		result.Error = err.Error()
		return
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		result.Error = err.Error()
		return
	}
	if len(content) > maxBodyBytes {
		result.Error = "gateway response exceeds 16MB"
		return
	}
	result.Status = response.StatusCode
	result.Headers = filteredHeaders(response.Header)
	result.BodyBase64 = base64.StdEncoding.EncodeToString(content)
}

func (a *Agent) openWebSocket(ctx context.Context, input message) {
	target, err := a.resolvedLocalWebSocketTarget(ctx, input.Path)
	if err != nil {
		_ = a.send(message{Type: "ws.error", StreamID: input.StreamID, Error: err.Error()})
		return
	}
	headers := http.Header{}
	copyHeaders(headers, input.Headers)
	headers.Del("Origin")
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  loopbackTLSConfig(),
	}
	local, _, err := dialer.DialContext(ctx, target, headers)
	if err != nil {
		_ = a.send(message{Type: "ws.error", StreamID: input.StreamID, Error: err.Error()})
		return
	}
	a.streamsMu.Lock()
	a.streams[input.StreamID] = local
	a.streamsMu.Unlock()
	_ = a.send(message{Type: "ws.opened", StreamID: input.StreamID})
	defer func() { a.closeStream(input.StreamID); _ = a.send(message{Type: "ws.close", StreamID: input.StreamID}) }()
	for {
		messageType, data, err := local.ReadMessage()
		if err != nil {
			return
		}
		if a.send(message{Type: "ws.data", StreamID: input.StreamID, DataBase64: base64.StdEncoding.EncodeToString(data), Binary: messageType == websocket.BinaryMessage}) != nil {
			return
		}
	}
}

func loopbackTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402 -- target is enforced as 127.0.0.1/localhost
}

func (a *Agent) writeWebSocket(input message) {
	data, err := base64.StdEncoding.DecodeString(input.DataBase64)
	if err != nil {
		return
	}
	a.streamsMu.Lock()
	stream := a.streams[input.StreamID]
	a.streamsMu.Unlock()
	if stream != nil {
		messageType := websocket.TextMessage
		if input.Binary {
			messageType = websocket.BinaryMessage
		}
		_ = stream.WriteMessage(messageType, data)
	}
}

func (a *Agent) send(output message) error {
	raw, err := json.Marshal(output)
	if err != nil {
		return err
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if a.cloud == nil {
		return errors.New("cloud tunnel is disconnected")
	}
	return a.cloud.WriteMessage(websocket.TextMessage, raw)
}

func (a *Agent) localTarget(path string) (string, error) {
	base, err := url.Parse(a.cfg.LocalURL)
	if err != nil {
		return "", err
	}
	if base.Hostname() != "127.0.0.1" && base.Hostname() != "localhost" {
		return "", errors.New("remote access target must be localhost")
	}
	relative, err := url.Parse(path)
	if err != nil || relative.IsAbs() || relative.Host != "" {
		return "", errors.New("invalid proxy path")
	}
	base.Path = relative.Path
	base.RawQuery = relative.RawQuery
	return base.String(), nil
}

func (a *Agent) resolvedLocalTarget(ctx context.Context, path string) (string, error) {
	target, err := a.localTarget(path)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	scheme, err := a.resolveLocalScheme(ctx, u)
	if err != nil {
		return "", err
	}
	u.Scheme = scheme
	return u.String(), nil
}

func (a *Agent) resolvedLocalWebSocketTarget(ctx context.Context, path string) (string, error) {
	target, err := a.resolvedLocalTarget(ctx, path)
	if err != nil {
		return "", err
	}
	u, _ := url.Parse(target)
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	return u.String(), nil
}

// resolveLocalScheme probes the loopback health endpoint once and caches the
// working protocol. It only switches schemes for an explicit HTTP/TLS protocol
// mismatch, so certificate, authentication and network errors cannot silently
// downgrade a configured HTTPS endpoint.
func (a *Agent) resolveLocalScheme(ctx context.Context, base *url.URL) (string, error) {
	a.localMu.Lock()
	defer a.localMu.Unlock()
	if a.localScheme != "" {
		return a.localScheme, nil
	}
	preferred := strings.ToLower(strings.TrimSpace(base.Scheme))
	if preferred != "http" && preferred != "https" {
		return "", errors.New("remote access localUrl must use http:// or https://")
	}
	matched, mismatch, err := a.probeLocalScheme(ctx, base, preferred)
	if matched {
		a.localScheme = preferred
		return preferred, nil
	}
	if !mismatch {
		return "", err
	}
	alternate := "http"
	if preferred == "http" {
		alternate = "https"
	}
	matched, _, alternateErr := a.probeLocalScheme(ctx, base, alternate)
	if !matched {
		if alternateErr != nil {
			return "", alternateErr
		}
		return "", fmt.Errorf("local management protocol detection failed for %s", base.Host)
	}
	a.localScheme = alternate
	log.Printf("[REMOTE] local management protocol auto-detected scheme=%s configured=%s host=%s", alternate, preferred, base.Host)
	return alternate, nil
}

func (a *Agent) probeLocalScheme(ctx context.Context, base *url.URL, scheme string) (matched bool, mismatch bool, err error) {
	probe := *base
	probe.Scheme = scheme
	probe.Path = "/api/healthz"
	probe.RawQuery = ""
	probe.Fragment = ""
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.String(), nil)
	if err != nil {
		return false, false, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		if localProtocolMismatchError(err) {
			return false, true, err
		}
		return false, false, err
	}
	defer response.Body.Close()
	content, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
	if readErr != nil {
		return false, false, readErr
	}
	if localProtocolMismatchResponse(response.StatusCode, content) {
		return false, true, fmt.Errorf("local management protocol mismatch for %s", probe.Host)
	}
	return true, false, nil
}

func localProtocolMismatchError(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "server gave http response to https client") ||
		strings.Contains(text, "first record does not look like a tls handshake")
}

func localProtocolMismatchResponse(status int, body []byte) bool {
	if status != http.StatusBadRequest {
		return false
	}
	text := strings.ToLower(string(body))
	return strings.Contains(text, "client sent an http request to an https server") ||
		strings.Contains(text, "plain http request was sent to https port")
}
func (a *Agent) closeStream(id string) {
	a.streamsMu.Lock()
	stream := a.streams[id]
	delete(a.streams, id)
	a.streamsMu.Unlock()
	if stream != nil {
		_ = stream.Close()
	}
}
func (a *Agent) closeStreams() {
	a.streamsMu.Lock()
	streams := a.streams
	a.streams = map[string]*websocket.Conn{}
	a.streamsMu.Unlock()
	for _, stream := range streams {
		_ = stream.Close()
	}
	a.sshStreamsMu.Lock()
	sshStreams := a.sshStreams
	openings := a.sshOpening
	a.sshStreams = map[string]*sshStream{}
	a.sshOpening = map[string]*sshOpening{}
	a.sshStreamsMu.Unlock()
	for _, opening := range openings {
		opening.cancel()
	}
	for _, stream := range sshStreams {
		stream.close()
	}
}
func copyHeaders(target http.Header, source map[string][]string) {
	for key, values := range source {
		if !hopHeader(key) {
			for _, value := range values {
				target.Add(key, value)
			}
		}
	}
}
func filteredHeaders(source http.Header) map[string][]string {
	result := map[string][]string{}
	for key, values := range source {
		if !hopHeader(key) {
			result[key] = append([]string(nil), values...)
		}
	}
	return result
}
func hopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "upgrade", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding":
		return true
	}
	return false
}
