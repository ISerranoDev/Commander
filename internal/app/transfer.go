package app

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ISerranoDev/WailsCommander/internal/vault"
)

// Export asks for a destination and writes an encrypted backup protected by
// passphrase. It returns the chosen path, or "" if the user cancelled.
func (a *App) Export(passphrase string) (string, error) {
	if len(passphrase) < vault.MinPasswordLength {
		return "", vault.ErrWeakPassword
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           a.dialogTitle("export"),
		DefaultFilename: "wailscommander-backup.bin",
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := a.vault.Export(path, passphrase); err != nil {
		return "", err
	}
	return path, nil
}

// Import asks for an export file and merges it into the vault. It returns
// the number of imported hosts, or -1 if the user cancelled.
func (a *App) Import(passphrase string) (int, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: a.dialogTitle("import"),
	})
	if err != nil {
		return 0, err
	}
	if path == "" {
		return -1, nil
	}
	return a.vault.Import(path, passphrase)
}

// Native dialog titles are set from Go, so they are translated here; the
// rest of the UI is translated in the frontend.
var dialogTitles = map[string]map[string]string{
	"en": {"export": "Export hosts", "import": "Import hosts", "pickKey": "Choose private key"},
	"es": {"export": "Exportar hosts", "import": "Importar hosts", "pickKey": "Elegir clave privada"},
}

func (a *App) dialogTitle(key string) string {
	if titles, ok := dialogTitles[a.settings.Load().Language]; ok {
		return titles[key]
	}
	return dialogTitles["en"][key]
}
