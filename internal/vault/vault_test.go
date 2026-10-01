package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

const pw = "correct horse battery"

func init() {
	// Keep tests fast; production parameters are exercised by the format,
	// not the cost.
	kdfProfiles = []kdfProfile{{time: 1, memory: 1024, threads: 1}}
}

func newTestVault(t *testing.T) *Vault {
	t.Helper()
	v := New(filepath.Join(t.TempDir(), "store.dat"))
	if err := v.Create(pw, nil); err != nil {
		t.Fatal(err)
	}
	return v
}

func sampleHost() model.Host {
	return model.Host{Name: "prod", Address: "10.0.0.1", Username: "root", Password: "s3cr3t-value"}
}

func TestRoundTrip(t *testing.T) {
	v := newTestVault(t)
	h, err := v.PutHost(sampleHost())
	if err != nil {
		t.Fatal(err)
	}
	if h.ID == "" || h.Port != 22 {
		t.Fatalf("unexpected host: %+v", h)
	}

	v.Lock()
	if _, err := v.Hosts(); !errors.Is(err, ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err)
	}
	if err := v.Unlock("wrong password"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
	if err := v.Unlock(pw); err != nil {
		t.Fatal(err)
	}
	got, err := v.Host(h.ID)
	if err != nil || got.Password != "s3cr3t-value" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestFileIsOpaque(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.PutHost(sampleHost()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(v.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"s3cr3t-value", "10.0.0.1", "root", "hosts", "address"} {
		if bytes.Contains(raw, []byte(needle)) {
			t.Errorf("vault file leaks %q", needle)
		}
	}
	if len(raw)%padBlock != headerLen+16 {
		t.Errorf("unexpected size %d, padding not applied", len(raw))
	}
	info, _ := os.Stat(v.Path())
	if perm := info.Mode().Perm(); perm&0o077 != 0 && os.PathSeparator == '/' {
		t.Errorf("vault file permissions too open: %v", perm)
	}
}

func TestTamperDetected(t *testing.T) {
	v := newTestVault(t)
	raw, _ := os.ReadFile(v.Path())
	raw[len(raw)-1] ^= 1
	os.WriteFile(v.Path(), raw, 0o600)
	v.Lock()
	if err := v.Unlock(pw); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	v := newTestVault(t)
	if err := v.ChangePassword("nope-nope", "new password!"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
	if err := v.ChangePassword(pw, "new password!"); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if err := v.Unlock(pw); err == nil {
		t.Fatal("old password still works")
	}
	if err := v.Unlock("new password!"); err != nil {
		t.Fatal(err)
	}
}

func TestExportImport(t *testing.T) {
	src := newTestVault(t)
	h, _ := src.PutHost(sampleHost())
	export := filepath.Join(t.TempDir(), "backup.bin")
	if err := src.Export(export, "export-pass"); err != nil {
		t.Fatal(err)
	}

	dst := newTestVault(t)
	if _, err := dst.Import(export, pw); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("export must not open with another password, got %v", err)
	}
	n, err := dst.Import(export, "export-pass")
	if err != nil || n.Hosts != 1 {
		t.Fatalf("n=%+v err=%v", n, err)
	}
	// Re-importing replaces by ID instead of duplicating.
	if _, err := dst.Import(export, "export-pass"); err != nil {
		t.Fatal(err)
	}
	hosts, _ := dst.Hosts()
	if len(hosts) != 1 || hosts[0].ID != h.ID || hosts[0].Password != "s3cr3t-value" {
		t.Fatalf("unexpected hosts after import: %+v", hosts)
	}
}

func TestCreateRules(t *testing.T) {
	v := New(filepath.Join(t.TempDir(), "store.dat"))
	if err := v.Create("short", nil); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
	if err := v.Create(pw, nil); err != nil {
		t.Fatal(err)
	}
	if err := v.Create(pw, nil); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
}

func TestKnownHostsSurviveHostChanges(t *testing.T) {
	v := newTestVault(t)
	if err := v.TrustHost("10.0.0.1:22", "ssh-ed25519 AAAA"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.PutHost(sampleHost()); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if err := v.Unlock(pw); err != nil {
		t.Fatal(err)
	}
	key, ok, err := v.KnownHost("10.0.0.1:22")
	if err != nil || !ok || key != "ssh-ed25519 AAAA" {
		t.Fatalf("key=%q ok=%v err=%v", key, ok, err)
	}
}
