package app

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ISerranoDev/WailsCommander/internal/model"
	"github.com/ISerranoDev/WailsCommander/internal/sshkey"
)

// HostSummary is what the UI gets: no secrets, only whether they are set.
type HostSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Address          string `json:"address"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	AuthMethod       string `json:"authMethod"`
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
		HasPassword:      h.Password != "",
		HasPrivateKey:    h.PrivateKey != "",
		HasKeyPassphrase: h.KeyPassphrase != "",
	}
}

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
