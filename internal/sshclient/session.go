// Package sshclient runs interactive SSH shell sessions and streams them to
// the UI through an event sink.
package sshclient

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	dialTimeout     = 15 * time.Second
	hostKeyTimeout  = 3 * time.Minute
	keepAliveEvery  = 30 * time.Second
	readBufferSize  = 32 * 1024
	defaultTermType = "xterm-256color"
	eventStatus     = "ssh:status:"
	eventData       = "ssh:data:"
	eventHostKey    = "ssh:hostkey:"
	eventClosed     = "ssh:closed:"
)

// Connection stages reported to the UI, in order.
const (
	StageConnecting     = "connecting"
	StageVerifying      = "verifying"
	StageAuthenticating = "authenticating"
	StageShell          = "shell"
	StageReady          = "ready"
)

var (
	ErrAuthFailed      = errors.New("authentication failed")
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExists   = errors.New("session already exists")
)

// Emitter delivers events to the UI (runtime.EventsEmit in the app).
type Emitter func(event string, data ...any)

type Config struct {
	Address       string
	Port          int
	Username      string
	Password      string
	PrivateKey    string
	KeyPassphrase string
	Cols, Rows    int
}

type Manager struct {
	emit       Emitter
	knownHosts KnownHosts

	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager(emit Emitter, knownHosts KnownHosts) *Manager {
	return &Manager{emit: emit, knownHosts: knownHosts, sessions: map[string]*session{}}
}

type session struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc

	hostKeyAnswer chan bool

	mu     sync.Mutex
	client *ssh.Client
	shell  *ssh.Session
	stdin  io.WriteCloser
}

// Start connects in the background. Progress, output and the end of the
// session are reported as events suffixed with id; the caller subscribes to
// them before calling Start.
func (m *Manager) Start(id string, cfg Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{id: id, ctx: ctx, cancel: cancel, hostKeyAnswer: make(chan bool, 1)}

	m.mu.Lock()
	if _, exists := m.sessions[id]; exists {
		m.mu.Unlock()
		cancel()
		return ErrSessionExists
	}
	m.sessions[id] = s
	m.mu.Unlock()

	go m.run(s, cfg)
	return nil
}

func (m *Manager) run(s *session, cfg Config) {
	err := m.connect(s, cfg)
	if err == nil {
		err = m.pump(s)
	}
	m.finish(s, err)
}

func (m *Manager) connect(s *session, cfg Config) error {
	hostport := net.JoinHostPort(cfg.Address, strconv.Itoa(cfg.Port))
	m.emit(eventStatus+s.id, StageConnecting)

	auth, err := authMethods(cfg, func() { m.emit(eventStatus+s.id, StageAuthenticating) })
	if err != nil {
		return err
	}
	clientConfig := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            auth,
		HostKeyCallback: m.hostKeyCallback(s),
		Timeout:         dialTimeout,
	}

	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(s.ctx, "tcp", hostport)
	if err != nil {
		return err
	}
	// Abort the handshake if the tab is closed while it is in progress.
	stop := context.AfterFunc(s.ctx, func() { conn.Close() })
	defer stop()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, hostport, clientConfig)
	if err != nil {
		conn.Close()
		if s.ctx.Err() != nil {
			return context.Canceled
		}
		return normalizeHandshakeError(err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)

	m.emit(eventStatus+s.id, StageShell)
	shell, err := client.NewSession()
	if err != nil {
		client.Close()
		return err
	}
	cols, rows := cfg.Cols, cfg.Rows
	if cols <= 0 || rows <= 0 {
		cols, rows = 80, 24
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := shell.RequestPty(defaultTermType, rows, cols, modes); err != nil {
		client.Close()
		return err
	}
	stdin, err := shell.StdinPipe()
	if err != nil {
		client.Close()
		return err
	}
	stdout, err := shell.StdoutPipe()
	if err != nil {
		client.Close()
		return err
	}
	if err := shell.Shell(); err != nil {
		client.Close()
		return err
	}

	s.mu.Lock()
	s.client, s.shell, s.stdin = client, shell, stdin
	s.mu.Unlock()

	go m.keepAlive(s, client)
	go m.stream(s, stdout)
	m.emit(eventStatus+s.id, StageReady)
	return nil
}

func (m *Manager) hostKeyCallback(s *session) ssh.HostKeyCallback {
	return func(hostport string, remote net.Addr, key ssh.PublicKey) error {
		m.emit(eventStatus+s.id, StageVerifying)
		status, err := checkHostKey(m.knownHosts, hostport, remote, key)
		if err != nil || status == keyTrusted {
			return err
		}

		m.emit(eventHostKey+s.id, promptFor(hostport, key, status))
		select {
		case accepted := <-s.hostKeyAnswer:
			if !accepted {
				return ErrHostKeyRejected
			}
			return m.knownHosts.TrustHost(hostport, encodeKey(key))
		case <-time.After(hostKeyTimeout):
			return ErrHostKeyRejected
		case <-s.ctx.Done():
			return context.Canceled
		}
	}
}

// AnswerHostKey delivers the user's decision about an unknown/changed key.
func (m *Manager) AnswerHostKey(id string, accept bool) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	select {
	case s.hostKeyAnswer <- accept:
	default:
	}
	return nil
}

func (m *Manager) stream(s *session, stdout io.Reader) {
	buf := make([]byte, readBufferSize)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			// Base64 keeps arbitrary bytes (and UTF-8 split across reads)
			// intact through the JSON event bridge.
			m.emit(eventData+s.id, base64.StdEncoding.EncodeToString(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

// pump blocks until the remote shell exits or the session is closed.
func (m *Manager) pump(s *session) error {
	done := make(chan error, 1)
	go func() { done <- s.shell.Wait() }()
	select {
	case err := <-done:
		var exit *ssh.ExitError
		if errors.As(err, &exit) || errors.Is(err, io.EOF) {
			return nil
		}
		var missing *ssh.ExitMissingError
		if errors.As(err, &missing) {
			return nil
		}
		return err
	case <-s.ctx.Done():
		return context.Canceled
	}
}

func (m *Manager) keepAlive(s *session, client *ssh.Client) {
	ticker := time.NewTicker(keepAliveEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				client.Close()
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (m *Manager) finish(s *session, err error) {
	s.cancel()
	s.mu.Lock()
	if s.client != nil {
		s.client.Close()
	}
	s.mu.Unlock()

	m.mu.Lock()
	delete(m.sessions, s.id)
	m.mu.Unlock()

	reason := ""
	if err != nil && !errors.Is(err, context.Canceled) {
		reason = err.Error()
	}
	m.emit(eventClosed+s.id, reason)
}

func (m *Manager) Write(id, data string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdin == nil {
		return nil
	}
	_, err = io.WriteString(s.stdin, data)
	return err
}

func (m *Manager) Resize(id string, cols, rows int) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shell == nil || cols <= 0 || rows <= 0 {
		return nil
	}
	return s.shell.WindowChange(rows, cols)
}

func (m *Manager) Close(id string) {
	if s, err := m.get(id); err == nil {
		s.cancel()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		s.cancel()
	}
}

func (m *Manager) get(id string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return s, nil
}

func authMethods(cfg Config, onAttempt func()) ([]ssh.AuthMethod, error) {
	var once sync.Once
	notify := func() { once.Do(onAttempt) }

	if cfg.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if cfg.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(cfg.PrivateKey), []byte(cfg.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("private key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			notify()
			return []ssh.Signer{signer}, nil
		})}, nil
	}

	password := cfg.Password
	return []ssh.AuthMethod{
		ssh.PasswordCallback(func() (string, error) {
			notify()
			return password, nil
		}),
		// Many servers only enable keyboard-interactive; answer every
		// prompt with the password.
		ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			notify()
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = password
			}
			return answers, nil
		}),
	}, nil
}

func normalizeHandshakeError(err error) error {
	if errors.Is(err, ErrHostKeyRejected) {
		return ErrHostKeyRejected
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.New("connection timed out")
	}
	// x/crypto/ssh reports auth failures as a plain error string.
	if msg := err.Error(); strings.Contains(msg, "unable to authenticate") || strings.Contains(msg, "no supported methods remain") {
		return ErrAuthFailed
	}
	return err
}
