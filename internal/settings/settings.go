// Package settings persists non-secret UI preferences. They live outside the
// vault because some (like the language) are needed before it is unlocked.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

var (
	Languages       = []string{"en", "es"}
	AutoLockOptions = []int{0, 5, 15, 30, 60} // minutes; 0 = never

	ErrInvalidLanguage = errors.New("unsupported language")
	ErrInvalidAutoLock = errors.New("unsupported auto-lock interval")
)

type Settings struct {
	// Language is empty until the user (or the UI, from the OS locale)
	// picks one.
	Language        string `json:"language"`
	AutoLockMinutes int    `json:"autoLockMinutes"`
}

func Defaults() Settings {
	return Settings{AutoLockMinutes: 15}
}

func (s Settings) Validate() error {
	if s.Language != "" && !slices.Contains(Languages, s.Language) {
		return ErrInvalidLanguage
	}
	if !slices.Contains(AutoLockOptions, s.AutoLockMinutes) {
		return ErrInvalidAutoLock
	}
	return nil
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load returns the stored settings, falling back to defaults for a missing
// or unreadable file so a broken preferences file never blocks startup.
func (s *Store) Load() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Defaults()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return out
	}
	if json.Unmarshal(raw, &out) != nil || out.Validate() != nil {
		return Defaults()
	}
	return out
}

func (s *Store) Save(in Settings) error {
	if err := in.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
