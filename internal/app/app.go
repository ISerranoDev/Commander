// Package app holds the struct bound to the Wails frontend. Every exported
// method on App is callable from JavaScript; keep them thin and push logic
// into the domain packages.
package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ISerranoDev/WailsCommander/internal/legacy"
	"github.com/ISerranoDev/WailsCommander/internal/model"
	"github.com/ISerranoDev/WailsCommander/internal/settings"
	"github.com/ISerranoDev/WailsCommander/internal/sshclient"
	"github.com/ISerranoDev/WailsCommander/internal/vault"
)

type App struct {
	ctx      context.Context
	vault    *vault.Vault
	settings *settings.Store
	ssh      *sshclient.Manager
	transfer transferState
}

func New(v *vault.Vault, s *settings.Store) *App {
	return &App{vault: v, settings: s}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	emit := func(event string, data ...any) { runtime.EventsEmit(ctx, event, data...) }
	a.ssh = sshclient.NewManager(emit, a.vault)
}

func (a *App) Shutdown(context.Context) {
	a.ssh.CloseAll()
	a.vault.Lock()
}

// ---- Vault lifecycle ------------------------------------------------------

type VaultStatus struct {
	Exists   bool   `json:"exists"`
	Unlocked bool   `json:"unlocked"`
	DataDir  string `json:"dataDir"`
	// LegacyFile is the path of a plaintext hosts.json from an older
	// version, offered for migration when no vault exists yet.
	LegacyFile string `json:"legacyFile"`
}

func (a *App) Status() VaultStatus {
	s := VaultStatus{
		Exists:   a.vault.Exists(),
		Unlocked: a.vault.Unlocked(),
		DataDir:  filepath.Dir(a.vault.Path()),
	}
	if !s.Exists {
		s.LegacyFile, _ = legacy.HostsFile()
	}
	return s
}

// CreateVault creates the vault. If importLegacy is set, hosts from the old
// plaintext hosts.json are imported. It returns how many were imported.
func (a *App) CreateVault(password string, importLegacy bool) (int, error) {
	var initial []model.Host
	if importLegacy {
		if path, ok := legacy.HostsFile(); ok {
			hosts, err := legacy.ReadHosts(path)
			if err != nil {
				return 0, err
			}
			initial = hosts
		}
	}
	if err := a.vault.Create(password, initial); err != nil {
		return 0, err
	}
	return len(initial), nil
}

// DeleteLegacyFile removes the old plaintext hosts.json after migration.
func (a *App) DeleteLegacyFile() error {
	path, ok := legacy.HostsFile()
	if !ok {
		return nil
	}
	return os.Remove(path)
}

func (a *App) Unlock(password string) error {
	return a.vault.Unlock(password)
}

func (a *App) Lock() {
	a.vault.Lock()
}

func (a *App) ChangeMasterPassword(current, next string) error {
	return a.vault.ChangePassword(current, next)
}

// ---- Settings -------------------------------------------------------------

func (a *App) GetSettings() settings.Settings {
	return a.settings.Load()
}

func (a *App) SaveSettings(s settings.Settings) error {
	return a.settings.Save(s)
}
