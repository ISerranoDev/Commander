package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ISerranoDev/WailsCommander/internal/model"
	"github.com/ISerranoDev/WailsCommander/internal/settings"
	"github.com/ISerranoDev/WailsCommander/internal/vault"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	v := vault.New(filepath.Join(dir, "store.dat"))
	if err := v.Create("master-password", nil); err != nil {
		t.Fatal(err)
	}
	return New(v, settings.NewStore(filepath.Join(dir, "settings.json")))
}

// The dialogs can't run in tests, so the chosen paths are set directly.
func TestTransferRequiresChosenFile(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.ExportHosts("passphrase"); !errors.Is(err, errNoFileChosen) {
		t.Fatalf("export without file: %v", err)
	}
	if _, err := a.ImportHosts("passphrase"); !errors.Is(err, errNoFileChosen) {
		t.Fatalf("import without file: %v", err)
	}
}

func TestTransferRoundTrip(t *testing.T) {
	src := newTestApp(t)
	if _, err := src.vault.PutHost(model.Host{Name: "web", Address: "10.0.0.1", Username: "root", Password: "pw"}); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "b.commander")
	src.transfer.exportPath = backup
	if path, err := src.ExportHosts("backup-pass"); err != nil || path != backup {
		t.Fatalf("path=%q err=%v", path, err)
	}
	if src.transfer.exportPath != "" {
		t.Fatal("export path not cleared")
	}

	dst := newTestApp(t)
	dst.transfer.importPath = backup
	if _, err := dst.ImportHosts("wrong-pass"); !errors.Is(err, vault.ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
	if dst.transfer.importPath != backup {
		t.Fatal("chosen file must survive a wrong passphrase")
	}
	n, err := dst.ImportHosts("backup-pass")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if dst.transfer.importPath != "" {
		t.Fatal("import path not cleared")
	}
}
