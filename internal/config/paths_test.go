package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyDir(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, legacyAppName)
	current := filepath.Join(base, AppName)

	// Nothing yet: use the new name.
	if got := migrateLegacyDir(legacy, current); got != current {
		t.Fatalf("got %s", got)
	}

	// Only legacy exists: it is moved, contents included.
	os.MkdirAll(legacy, 0o700)
	os.WriteFile(filepath.Join(legacy, vaultFile), []byte("x"), 0o600)
	if got := migrateLegacyDir(legacy, current); got != current {
		t.Fatalf("got %s", got)
	}
	if _, err := os.Stat(filepath.Join(current, vaultFile)); err != nil {
		t.Fatal("vault not moved:", err)
	}
	if exists(legacy) {
		t.Fatal("legacy dir still present")
	}

	// Both exist: the new one wins and legacy is left untouched.
	os.MkdirAll(legacy, 0o700)
	if got := migrateLegacyDir(legacy, current); got != current || !exists(legacy) {
		t.Fatalf("got %s", got)
	}
}
