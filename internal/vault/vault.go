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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

const MinPasswordLength = 8

var (
	ErrLocked        = errors.New("vault is locked")
	ErrExists        = errors.New("vault already exists")
	ErrNotExists     = errors.New("vault does not exist")
	ErrNotFound      = errors.New("host not found")
	ErrGroupNotFound = errors.New("group not found")
	ErrWeakPassword  = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
)

// data is the plaintext document stored inside the encrypted file.
type data struct {
	Version int          `json:"v"`
	Hosts   []model.Host `json:"hosts"`
	// Groups are listed in display order; hosts are too (within and
	// across groups). Absent in files written before groups existed.
	Groups []model.Group `json:"groups,omitempty"`
	// KnownHosts maps "host:port" to the trusted server key in
	// authorized_keys format ("ssh-ed25519 AAAA...").
	KnownHosts map[string]string `json:"knownHosts,omitempty"`
}

// Version 2 added groups and made the stored host order the display order.
const dataVersion = 2

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
	d := &data{Version: dataVersion, Hosts: []model.Host{}, Groups: []model.Group{}, KnownHosts: map[string]string{}}
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

	if h.GroupID != "" && v.groupIndex(h.GroupID) < 0 {
		return model.Host{}, ErrGroupNotFound
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
		if next[i].GroupID == h.GroupID {
			next[i] = h
		} else {
			// Moving to another group puts the host last in it.
			next = append(slices.Delete(next, i, i+1), h)
		}
	}
	if err := v.commit(next, v.data.Groups); err != nil {
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
	return v.commit(slices.Delete(slices.Clone(v.data.Hosts), i, i+1), v.data.Groups)
}

func (v *Vault) Groups() ([]model.Group, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.key == nil {
		return nil, ErrLocked
	}
	return append([]model.Group{}, v.data.Groups...), nil
}

// PutGroup creates g when its ID is empty (appended last), otherwise
// renames the existing group with that ID.
func (v *Vault) PutGroup(g model.Group) (model.Group, error) {
	g.Normalize()
	if err := g.Validate(); err != nil {
		return model.Group{}, err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return model.Group{}, ErrLocked
	}

	next := slices.Clone(v.data.Groups)
	if g.ID == "" {
		g.ID = uuid.NewString()
		next = append(next, g)
	} else {
		i := v.groupIndex(g.ID)
		if i < 0 {
			return model.Group{}, ErrGroupNotFound
		}
		next[i] = g
	}
	if err := v.commit(v.data.Hosts, next); err != nil {
		return model.Group{}, err
	}
	return g, nil
}

// DeleteGroup removes the group; its hosts are kept, outside any group.
func (v *Vault) DeleteGroup(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	i := v.groupIndex(id)
	if i < 0 {
		return ErrGroupNotFound
	}
	hosts := slices.Clone(v.data.Hosts)
	for j := range hosts {
		if hosts[j].GroupID == id {
			hosts[j].GroupID = ""
		}
	}
	return v.commit(hosts, slices.Delete(slices.Clone(v.data.Groups), i, i+1))
}

// ReorderGroups sets the group order. Unknown IDs are ignored and groups
// missing from ids keep their relative order after the listed ones.
func (v *Vault) ReorderGroups(ids []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	next := make([]model.Group, 0, len(v.data.Groups))
	seen := map[string]bool{}
	for _, id := range ids {
		if i := v.groupIndex(id); i >= 0 && !seen[id] {
			seen[id] = true
			next = append(next, v.data.Groups[i])
		}
	}
	for _, g := range v.data.Groups {
		if !seen[g.ID] {
			next = append(next, g)
		}
	}
	return v.commit(v.data.Hosts, next)
}

// Placement puts a host in a group ("" for none) at its position in the
// list passed to ArrangeHosts.
type Placement struct {
	ID      string `json:"id"`
	GroupID string `json:"groupId"`
}

// ArrangeHosts sets the host order and group membership. Unknown host IDs
// are ignored, unknown group IDs mean no group, and hosts missing from
// layout keep their relative order after the listed ones.
func (v *Vault) ArrangeHosts(layout []Placement) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	next := make([]model.Host, 0, len(v.data.Hosts))
	seen := map[string]bool{}
	for _, p := range layout {
		i := v.indexOf(p.ID)
		if i < 0 || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		h := v.data.Hosts[i]
		h.GroupID = ""
		if v.groupIndex(p.GroupID) >= 0 {
			h.GroupID = p.GroupID
		}
		next = append(next, h)
	}
	for _, h := range v.data.Hosts {
		if !seen[h.ID] {
			next = append(next, h)
		}
	}
	return v.commit(next, v.data.Groups)
}

// Export writes every host and group to path, encrypted with passphrase (independent
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

// Import merges hosts and groups from an export file. Hosts and groups whose
// ID already exists are replaced; the rest are added. Files written before
// groups existed import as ungrouped hosts. It returns how many hosts were
// imported.
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
	groups := slices.Clone(v.data.Groups)
	for _, g := range in.Groups {
		g.Normalize()
		if g.ID == "" || g.Validate() != nil {
			continue
		}
		if i := slices.IndexFunc(groups, func(x model.Group) bool { return x.ID == g.ID }); i >= 0 {
			groups[i] = g
			continue
		}
		groups = append(groups, g)
	}
	known := groupIDs(groups)

	next := slices.Clone(v.data.Hosts)
	imported := 0
	for _, h := range in.Hosts {
		h.Normalize()
		if h.Validate() != nil {
			continue
		}
		if !known[h.GroupID] {
			h.GroupID = ""
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
	if err := v.commit(next, groups); err != nil {
		return 0, err
	}
	return imported, nil
}

// commit persists hosts and groups and only then swaps them into memory, so
// a failed write never leaves memory and disk out of sync. Caller holds v.mu.
func (v *Vault) commit(hosts []model.Host, groups []model.Group) error {
	return v.commitData(&data{Version: dataVersion, Hosts: hosts, Groups: groups, KnownHosts: v.data.KnownHosts})
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
	return v.commitData(&data{Version: dataVersion, Hosts: v.data.Hosts, Groups: v.data.Groups, KnownHosts: known})
}

func (v *Vault) indexOf(id string) int {
	return slices.IndexFunc(v.data.Hosts, func(h model.Host) bool { return h.ID == id })
}

func (v *Vault) groupIndex(id string) int {
	return slices.IndexFunc(v.data.Groups, func(g model.Group) bool { return g.ID == id })
}

// groupIDs is the set of valid GroupID values, including "" (no group).
func groupIDs(groups []model.Group) map[string]bool {
	ids := map[string]bool{"": true}
	for _, g := range groups {
		ids[g.ID] = true
	}
	return ids
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
	upgrade(&d)
	return &d, key, nil
}

// upgrade brings a decoded document (vault or export, any version) to the
// current shape in memory. It is saved in that shape on the next change.
func upgrade(d *data) {
	if d.Hosts == nil {
		d.Hosts = []model.Host{}
	}
	// Vaults and exports from before groups have none; the UI needs [] not null.
	if d.Groups == nil {
		d.Groups = []model.Group{}
	}
	if d.Version < 2 {
		// The UI used to sort hosts by name; keep that order now that the
		// stored order is the one shown.
		slices.SortStableFunc(d.Hosts, func(a, b model.Host) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
	}
	known := groupIDs(d.Groups)
	for i := range d.Hosts {
		if !known[d.Hosts[i].GroupID] {
			d.Hosts[i].GroupID = ""
		}
	}
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
