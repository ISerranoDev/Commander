package sshclient

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// ---- Fakes ------------------------------------------------------------------

type memKnownHosts struct {
	mu   sync.Mutex
	keys map[string]string
}

func (m *memKnownHosts) KnownHost(hp string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[hp]
	return k, ok, nil
}

func (m *memKnownHosts) TrustHost(hp, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[hp] = key
	return nil
}

type event struct {
	name string
	data any
}

func recorder() (Emitter, chan event) {
	ch := make(chan event, 256)
	return func(name string, data ...any) {
		var d any
		if len(data) > 0 {
			d = data[0]
		}
		ch <- event{name, d}
	}, ch
}

func waitFor(t *testing.T, events chan event, match func(event) bool) event {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e := <-events:
			if match(e) {
				return e
			}
		case <-timeout:
			t.Fatal("timed out waiting for event")
		}
	}
}

// ---- Test server ------------------------------------------------------------

// startServer runs an SSH server accepting demo/pw whose shell echoes input
// and exits on "exit".
func startServer(t *testing.T) (host string, port int, key ssh.PublicKey) {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "demo" && string(pass) == "pw" {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	config.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveConn(conn, config)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port, signer.PublicKey()
}

func serveConn(conn net.Conn, config *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			newCh.Reject(ssh.UnknownChannelType, "")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range chReqs {
				switch req.Type {
				case "pty-req", "window-change":
					req.Reply(true, nil)
				case "shell":
					req.Reply(true, nil)
					go echoShell(ch)
				default:
					req.Reply(false, nil)
				}
			}
		}()
	}
}

func echoShell(ch ssh.Channel) {
	ch.Write([]byte("welcome\r\n"))
	buf := make([]byte, 1024)
	for {
		n, err := ch.Read(buf)
		if err != nil {
			return
		}
		ch.Write(buf[:n])
		if strings.Contains(string(buf[:n]), "exit") {
			ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
			ch.Close()
			return
		}
	}
}

// ---- Tests ------------------------------------------------------------------

func TestSessionLifecycle(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // ignore the developer's ~/.ssh/known_hosts
	host, port, _ := startServer(t)
	emit, events := recorder()
	known := &memKnownHosts{keys: map[string]string{}}
	m := NewManager(emit, known)

	cfg := Config{Address: host, Port: port, Username: "demo", Password: "pw", Cols: 100, Rows: 30}
	if err := m.Start("s1", cfg); err != nil {
		t.Fatal(err)
	}

	prompt := waitFor(t, events, func(e event) bool { return e.name == "ssh:hostkey:s1" }).data.(HostKeyPrompt)
	if prompt.Changed || !strings.HasPrefix(prompt.Fingerprint, "SHA256:") {
		t.Fatalf("unexpected prompt %+v", prompt)
	}
	if err := m.AnswerHostKey("s1", true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, events, func(e event) bool { return e.name == "ssh:status:s1" && e.data == StageReady })

	if _, ok, _ := known.KnownHost(net.JoinHostPort(host, strconv.Itoa(port))); !ok {
		t.Fatal("accepted host key was not stored")
	}

	waitFor(t, events, func(e event) bool { return e.name == "ssh:data:s1" && decode(e) == "welcome\r\n" })
	if err := m.Write("s1", "hello"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, events, func(e event) bool { return e.name == "ssh:data:s1" && decode(e) == "hello" })
	if err := m.Resize("s1", 120, 40); err != nil {
		t.Fatal(err)
	}

	m.Write("s1", "exit")
	closed := waitFor(t, events, func(e event) bool { return e.name == "ssh:closed:s1" })
	if closed.data != "" {
		t.Fatalf("clean exit should have no reason, got %q", closed.data)
	}

	// Second connection: key is now trusted, no prompt.
	if err := m.Start("s2", cfg); err != nil {
		t.Fatal(err)
	}
	e := waitFor(t, events, func(e event) bool {
		return e.name == "ssh:hostkey:s2" || (e.name == "ssh:status:s2" && e.data == StageReady)
	})
	if e.name == "ssh:hostkey:s2" {
		t.Fatal("trusted host asked again")
	}
	m.Close("s2")
	waitFor(t, events, func(e event) bool { return e.name == "ssh:closed:s2" })
}

func TestChangedHostKeyAndBadPassword(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	host, port, _ := startServer(t)
	emit, events := recorder()
	hp := net.JoinHostPort(host, strconv.Itoa(port))
	known := &memKnownHosts{keys: map[string]string{hp: "ssh-ed25519 AAAAsomethingelse"}}
	m := NewManager(emit, known)

	m.Start("s1", Config{Address: host, Port: port, Username: "demo", Password: "wrong"})
	prompt := waitFor(t, events, func(e event) bool { return e.name == "ssh:hostkey:s1" }).data.(HostKeyPrompt)
	if !prompt.Changed {
		t.Fatal("changed key not flagged")
	}
	m.AnswerHostKey("s1", false)
	closed := waitFor(t, events, func(e event) bool { return e.name == "ssh:closed:s1" })
	if closed.data != ErrHostKeyRejected.Error() {
		t.Fatalf("got %q", closed.data)
	}

	m.Start("s2", Config{Address: host, Port: port, Username: "demo", Password: "wrong"})
	waitFor(t, events, func(e event) bool { return e.name == "ssh:hostkey:s2" })
	m.AnswerHostKey("s2", true)
	closed = waitFor(t, events, func(e event) bool { return e.name == "ssh:closed:s2" })
	if closed.data != ErrAuthFailed.Error() {
		t.Fatalf("got %q", closed.data)
	}
}

func decode(e event) string {
	b, _ := base64.StdEncoding.DecodeString(e.data.(string))
	return string(b)
}
