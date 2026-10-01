package app

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ISerranoDev/WailsCommander/internal/vault"
)

// Backup files use the same opaque encrypted format as the vault.
const backupExt = ".commander"

var errNoFileChosen = errors.New("no file chosen")

// Export and import are two-step: the user first picks a file in a native
// dialog, then enters the passphrase. The chosen path stays in Go, so the
// frontend can never make the app read or write an arbitrary path.
type transferState struct {
	mu         sync.Mutex
	exportPath string
	importPath string
}

// ChosenFile describes the file picked in a dialog. Name is empty when the
// dialog was cancelled.
type ChosenFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// ChooseExportFile opens a "Save as" dialog for the backup destination.
func (a *App) ChooseExportFile() (ChosenFile, error) {
	name := "commander-backup-" + time.Now().Format("2006-01-02") + backupExt
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                a.dialogTitle("export"),
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters:              a.backupFilters(false),
	})
	if err != nil || path == "" {
		return ChosenFile{}, err
	}
	if filepath.Ext(path) == "" {
		path += backupExt
	}
	a.transfer.mu.Lock()
	a.transfer.exportPath = path
	a.transfer.mu.Unlock()
	return ChosenFile{Name: filepath.Base(path), Path: path}, nil
}

// ExportHosts writes the encrypted backup to the file chosen with
// ChooseExportFile, protected by passphrase.
func (a *App) ExportHosts(passphrase string) (string, error) {
	a.transfer.mu.Lock()
	path := a.transfer.exportPath
	a.transfer.mu.Unlock()
	if path == "" {
		return "", errNoFileChosen
	}
	if err := a.vault.Export(path, passphrase); err != nil {
		return "", err
	}
	a.transfer.mu.Lock()
	a.transfer.exportPath = ""
	a.transfer.mu.Unlock()
	return path, nil
}

// ChooseImportFile opens an "Open" dialog to pick a backup to import.
func (a *App) ChooseImportFile() (ChosenFile, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   a.dialogTitle("import"),
		Filters: a.backupFilters(true),
	})
	if err != nil || path == "" {
		return ChosenFile{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return ChosenFile{}, err
	}
	if info.IsDir() {
		return ChosenFile{}, errNoFileChosen
	}
	a.transfer.mu.Lock()
	a.transfer.importPath = path
	a.transfer.mu.Unlock()
	return ChosenFile{Name: filepath.Base(path), Path: path, Size: info.Size()}, nil
}

// ImportHosts merges the file chosen with ChooseImportFile into the vault.
// The chosen file is kept after a wrong passphrase so the user can retry.
func (a *App) ImportHosts(passphrase string) (vault.Imported, error) {
	a.transfer.mu.Lock()
	path := a.transfer.importPath
	a.transfer.mu.Unlock()
	if path == "" {
		return vault.Imported{}, errNoFileChosen
	}
	n, err := a.vault.Import(path, passphrase)
	if err != nil {
		return vault.Imported{}, err
	}
	a.transfer.mu.Lock()
	a.transfer.importPath = ""
	a.transfer.mu.Unlock()
	return n, nil
}

func (a *App) backupFilters(includeAll bool) []runtime.FileFilter {
	filters := []runtime.FileFilter{{
		DisplayName: a.dialogTitle("backupFiles") + " (*" + backupExt + ")",
		Pattern:     "*" + backupExt,
	}}
	if includeAll {
		// Backups made before the .commander extension existed.
		filters = append(filters, runtime.FileFilter{DisplayName: a.dialogTitle("allFiles"), Pattern: "*"})
	}
	return filters
}

// Native dialog titles are set from Go, so they are translated here; the
// rest of the UI is translated in the frontend.
var dialogTitles = map[string]map[string]string{
	"en": {
		"export": "Export hosts", "import": "Import hosts", "pickKey": "Choose private key",
		"backupFiles": "Commander backup", "allFiles": "All files",
	},
	"es": {
		"export": "Exportar hosts", "import": "Importar hosts", "pickKey": "Elegir clave privada",
		"backupFiles": "Copia de Commander", "allFiles": "Todos los archivos",
	},
}

func (a *App) dialogTitle(key string) string {
	if titles, ok := dialogTitles[a.settings.Load().Language]; ok {
		return titles[key]
	}
	return dialogTitles["en"][key]
}
