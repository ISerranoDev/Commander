package app

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ISerranoDev/WailsCommander/internal/model"
	"github.com/ISerranoDev/WailsCommander/internal/sshkey"
	"github.com/ISerranoDev/WailsCommander/internal/vault"
)

// HostSummary is what the UI gets: no secrets, only whether they are set.
type HostSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Address          string `json:"address"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	AuthMethod       string `json:"authMethod"`
	GroupID          string `json:"groupId"`
	HasPassword      bool   `json:"hasPassword"`
	HasPrivateKey    bool   `json:"hasPrivateKey"`
	HasKeyPassphrase bool   `json:"hasKeyPassphrase"`
}

func summarize(h model.Host) HostSummary {
	return HostSummary{
		ID:               h.ID,
		Name:             h.Name,
		Address:          h.Address,
		Port:             h.Port,
		Username:         h.Username,
		AuthMethod:       h.AuthMethod,
		GroupID:          h.GroupID,
		HasPassword:      h.Password != "",
		HasPrivateKey:    h.PrivateKey != "",
		HasKeyPassphrase: h.KeyPassphrase != "",
	}
}

// ListHosts returns the hosts in display order.
func (a *App) ListHosts() ([]HostSummary, error) {
	hosts, err := a.vault.Hosts()
	if err != nil {
		return nil, err
	}
	out := make([]HostSummary, len(hosts))
	for i, h := range hosts {
		out[i] = summarize(h)
	}
	return out, nil
}

func (a *App) GetHost(id string) (HostSummary, error) {
	h, err := a.vault.Host(id)
	if err != nil {
		return HostSummary{}, err
	}
	return summarize(h), nil
}

// HostInput comes from the edit form. When updating a host that keeps the
// same auth method, empty secret fields keep the stored values.
type HostInput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthMethod    string `json:"authMethod"`
	GroupID       string `json:"groupId"`
	Password      string `json:"password"`
	PrivateKey    string `json:"privateKey"`
	KeyPassphrase string `json:"keyPassphrase"`
}

func (a *App) SaveHost(in HostInput) (HostSummary, error) {
	h := model.Host{
		ID:            in.ID,
		Name:          in.Name,
		Address:       in.Address,
		Port:          in.Port,
		Username:      in.Username,
		AuthMethod:    in.AuthMethod,
		GroupID:       in.GroupID,
		Password:      in.Password,
		PrivateKey:    in.PrivateKey,
		KeyPassphrase: in.KeyPassphrase,
	}
	if in.ID != "" {
		current, err := a.vault.Host(in.ID)
		if err != nil {
			return HostSummary{}, err
		}
		if current.AuthMethod == h.AuthMethod {
			if h.Password == "" {
				h.Password = current.Password
			}
			if h.PrivateKey == "" {
				h.PrivateKey = current.PrivateKey
				if h.KeyPassphrase == "" {
					h.KeyPassphrase = current.KeyPassphrase
				}
			}
		}
	}
	if h.AuthMethod == model.AuthKey && h.PrivateKey != "" {
		if err := sshkey.Verify([]byte(h.PrivateKey), h.KeyPassphrase); err != nil {
			return HostSummary{}, err
		}
	}
	saved, err := a.vault.PutHost(h)
	if err != nil {
		return HostSummary{}, err
	}
	return summarize(saved), nil
}

func (a *App) DeleteHost(id string) error {
	return a.vault.DeleteHost(id)
}

// ListGroups returns the groups in display order.
func (a *App) ListGroups() ([]model.Group, error) {
	return a.vault.Groups()
}

// SaveGroup creates a group (empty ID) or renames an existing one.
func (a *App) SaveGroup(g model.Group) (model.Group, error) {
	return a.vault.PutGroup(g)
}

// DeleteGroup removes a group, keeping its hosts outside any group.
func (a *App) DeleteGroup(id string) error {
	return a.vault.DeleteGroup(id)
}

func (a *App) ReorderGroups(ids []string) error {
	return a.vault.ReorderGroups(ids)
}

// ArrangeHosts saves the host order and group membership after a drag.
func (a *App) ArrangeHosts(layout []vault.Placement) error {
	return a.vault.ArrangeHosts(layout)
}

// KeyFile is a private key the user picked from disk. Name is empty when
// the dialog was cancelled.
type KeyFile struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	Encrypted bool   `json:"encrypted"`
}

// ReadKeyFile lets the user pick a private key file (defaults to ~/.ssh)
// and validates it before it reaches the form.
func (a *App) ReadKeyFile() (KeyFile, error) {
	opts := runtime.OpenDialogOptions{
		Title:           a.dialogTitle("pickKey"),
		ShowHiddenFiles: true,
	}
	if home, err := os.UserHomeDir(); err == nil {
		if dir := filepath.Join(home, ".ssh"); isDir(dir) {
			opts.DefaultDirectory = dir
		}
	}
	path, err := runtime.OpenFileDialog(a.ctx, opts)
	if err != nil || path == "" {
		return KeyFile{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return KeyFile{}, err
	}
	if info.Size() > sshkey.MaxSize {
		return KeyFile{}, sshkey.ErrTooLarge
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return KeyFile{}, err
	}
	encrypted, err := sshkey.Inspect(raw)
	if err != nil {
		return KeyFile{}, err
	}
	return KeyFile{Name: filepath.Base(path), Content: string(raw), Encrypted: encrypted}, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
