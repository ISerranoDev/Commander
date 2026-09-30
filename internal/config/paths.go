package config

import (
	"os"
	"path/filepath"
)

const (
	AppName      = "WailsCommander"
	vaultFile    = "store.dat"
	settingsFile = "settings.json"

	// DataDirEnv overrides the data directory (portable installs, dev).
	DataDirEnv = "WAILSCOMMANDER_DATA_DIR"
)

// DataDir is the per-user directory for application data:
//
//	macOS:   ~/Library/Application Support/WailsCommander
//	Windows: %AppData%\WailsCommander
//	Linux:   $XDG_CONFIG_HOME/WailsCommander (~/.config/WailsCommander)
func DataDir() (string, error) {
	if dir := os.Getenv(DataDirEnv); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

func VaultPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, vaultFile), nil
}

func SettingsPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, settingsFile), nil
}
