package settings

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := NewStore(path)

	if got := s.Load(); got != Defaults() {
		t.Fatalf("missing file should give defaults, got %+v", got)
	}
	want := Settings{Language: "es", AutoLockMinutes: 30}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	if got := NewStore(path).Load(); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if err := s.Save(Settings{Language: "fr", AutoLockMinutes: 15}); !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("want ErrInvalidLanguage, got %v", err)
	}
	if err := s.Save(Settings{Language: "en", AutoLockMinutes: 7}); !errors.Is(err, ErrInvalidAutoLock) {
		t.Fatalf("want ErrInvalidAutoLock, got %v", err)
	}

	os.WriteFile(path, []byte("{broken"), 0o600)
	if got := s.Load(); got != Defaults() {
		t.Fatalf("corrupt file should give defaults, got %+v", got)
	}
}
