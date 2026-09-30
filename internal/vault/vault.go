// Package vault stores hosts and credentials in a single encrypted file.
//
// The whole vault is decrypted into memory on Unlock and re-encrypted on
// every change. The derived key is held only while unlocked and is wiped on
// Lock.
package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

const MinPasswordLength = 8

var (
	ErrLocked       = errors.New("vault is locked")
	ErrExists       = errors.New("vault already exists")
	ErrNotExists    = errors.New("vault does not exist")
	ErrNotFound     = errors.New("host not found")
	ErrWeakPassword = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
)

// data is the plaintext document stored inside the encrypted file.
type data struct {
	Version int          `json:"v"`
	Hosts   []model.Host `json:"hosts"`
	// KnownHosts maps "host:port" to the trusted server key in
	// authorized_keys format ("ssh-ed25519 AAAA...").
	KnownHosts map[string]string `json:"knownHosts,omitempty"`
}

const dataVersion = 1

type Vault struct {
	path string

	mu   sync.RWMutex
	key  *sealed
	data *data
}

func New(path string) *Vault {
	return &Vault{path: path}
}

func (v *Vault) Path() string { return v.path }

func (v *Vault) Exists() bool {
	_, err := os.Stat(v.path)
	return err == nil
}

func (v *Vault) Unlocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.key != nil
}

// Create initialises a new empty vault protected by password and leaves it
// unlocked.
func (v *Vault) Create(password string, initial []model.Host) error {
	if len(password) < MinPasswordLength {
		return ErrWeakPassword
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.Exists() {
		return ErrExists
	}
	key, err := newSealed([]byte(password))
	if err != nil {
		return err
	}
	d := &data{Version: dataVersion, Hosts: []model.Host{}, KnownHosts: map[string]string{}}
	for _, h := range initial {
		d.Hosts = append(d.Hosts, prepareNew(h))
	}
	if err := writeEncrypted(v.path, key, d); err != nil {
		key.wipe()
		return err
	}
	v.key, v.data = key, d
	return nil
}

func (v *Vault) Unlock(password string) error {
	raw, err := os.ReadFile(v.path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotExists
	}
	if err != nil {
		return err
	}
	d, key, err := decode([]byte(password), raw)
	if err != nil {
		return err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.key.wipe()
	v.key, v.data = key, d
	return nil
}

func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.key.wipe()
	v.key, v.data = nil, nil
}

func (v *Vault) ChangePassword(current, next string) error {
	if len(next) < MinPasswordLength {
		return ErrWeakPassword
	}
	raw, err := os.ReadFile(v.path)
	if err != nil {
		return err
	}
	if _, old, err := decode([]byte(current), raw); err != nil {
		return err
	} else {
		old.wipe()
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	key, err := newSealed([]byte(next))
	if err != nil {
		return err
	}
	if err := writeEncrypted(v.path, key, v.data); err != nil {
		key.wipe()
		return err
	}
	v.key.wipe()
	v.key = key
	return nil
}

func (v *Vault) Hosts() ([]model.Host, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.key == nil {
		return nil, ErrLocked
	}
	return slices.Clone(v.data.Hosts), nil
}

func (v *Vault) Host(id string) (model.Host, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.key == nil {
		return model.Host{}, ErrLocked
	}
	i := v.indexOf(id)
	if i < 0 {
		return model.Host{}, ErrNotFound
	}
	return v.data.Hosts[i], nil
}

// PutHost inserts h when its ID is empty, otherwise replaces the existing
// host with that ID.
func (v *Vault) PutHost(h model.Host) (model.Host, error) {
	h.Normalize()
	if err := h.Validate(); err != nil {
		return model.Host{}, err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return model.Host{}, ErrLocked
	}

	next := slices.Clone(v.data.Hosts)
	if h.ID == "" {
		h = prepareNew(h)
		next = append(next, h)
	} else {
		i := v.indexOf(h.ID)
		if i < 0 {
			return model.Host{}, ErrNotFound
		}
		h.CreatedAt = next[i].CreatedAt
		h.UpdatedAt = time.Now().UTC()
		next[i] = h
	}
	if err := v.commit(next); err != nil {
		return model.Host{}, err
	}
	return h, nil
}

func (v *Vault) DeleteHost(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	i := v.indexOf(id)
	if i < 0 {
		return ErrNotFound
	}
	return v.commit(slices.Delete(slices.Clone(v.data.Hosts), i, i+1))
}

// Export writes every host to path, encrypted with passphrase (independent
// of the master password) using the same opaque format as the vault.
func (v *Vault) Export(path, passphrase string) error {
	if len(passphrase) < MinPasswordLength {
		return ErrWeakPassword
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.key == nil {
		return ErrLocked
	}
	key, err := newSealed([]byte(passphrase))
	if err != nil {
		return err
	}
	defer key.wipe()
	return writeEncrypted(path, key, v.data)
}

// Import merges hosts from an export file. Hosts whose ID already exists are
// replaced; the rest are added. It returns how many hosts were imported.
func (v *Vault) Import(path, passphrase string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	in, key, err := decode([]byte(passphrase), raw)
	if err != nil {
		return 0, err
	}
	key.wipe()

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return 0, ErrLocked
	}
	next := slices.Clone(v.data.Hosts)
	imported := 0
	for _, h := range in.Hosts {
		h.Normalize()
		if h.Validate() != nil {
			continue
		}
		imported++
		if i := slices.IndexFunc(next, func(x model.Host) bool { return h.ID != "" && x.ID == h.ID }); i >= 0 {
			next[i] = h
			continue
		}
		if h.ID == "" {
			h = prepareNew(h)
		}
		next = append(next, h)
	}
	if err := v.commit(next); err != nil {
		return 0, err
	}
	return imported, nil
}

// commit persists hosts and only then swaps them into memory, so a failed
// write never leaves memory and disk out of sync. Caller holds v.mu.
func (v *Vault) commit(hosts []model.Host) error {
	return v.commitData(&data{Version: dataVersion, Hosts: hosts, KnownHosts: v.data.KnownHosts})
}

func (v *Vault) commitData(d *data) error {
	if err := writeEncrypted(v.path, v.key, d); err != nil {
		return err
	}
	v.data = d
	return nil
}

// KnownHost returns the trusted server key for hostport, if any.
func (v *Vault) KnownHost(hostport string) (string, bool, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.key == nil {
		return "", false, ErrLocked
	}
	key, ok := v.data.KnownHosts[hostport]
	return key, ok, nil
}

// TrustHost records key as the trusted server key for hostport, replacing
// any previous one.
func (v *Vault) TrustHost(hostport, key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	known := maps.Clone(v.data.KnownHosts)
	if known == nil {
		known = map[string]string{}
	}
	known[hostport] = key
	return v.commitData(&data{Version: dataVersion, Hosts: v.data.Hosts, KnownHosts: known})
}

func (v *Vault) indexOf(id string) int {
	return slices.IndexFunc(v.data.Hosts, func(h model.Host) bool { return h.ID == id })
}

func prepareNew(h model.Host) model.Host {
	h.Normalize()
	h.ID = uuid.NewString()
	now := time.Now().UTC()
	h.CreatedAt, h.UpdatedAt = now, now
	return h
}

func decode(password, raw []byte) (*data, *sealed, error) {
	plaintext, key, err := open(password, raw)
	if err != nil {
		return nil, nil, err
	}
	defer wipeBytes(plaintext)
	var d data
	if err := json.Unmarshal(plaintext, &d); err != nil {
		key.wipe()
		return nil, nil, ErrDecrypt
	}
	if d.Hosts == nil {
		d.Hosts = []model.Host{}
	}
	return &d, key, nil
}

// writeEncrypted atomically replaces path: write to a temp file in the same
// directory, fsync, then rename over the target.
func writeEncrypted(path string, key *sealed, d *data) error {
	plaintext, err := json.Marshal(d)
	if err != nil {
		return err
	}
	defer wipeBytes(plaintext)
	blob, err := key.encrypt(plaintext)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(blob); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
