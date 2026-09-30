package config

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	AppName      = "Commander"
	vaultFile    = "store.dat"
	settingsFile = "settings.json"

	// DataDirEnv overrides the data directory (portable installs, dev).
	DataDirEnv = "COMMANDER_DATA_DIR"

	// Names used before the app was renamed to Commander.
	legacyAppName    = "WailsCommander"
	legacyDataDirEnv = "WAILSCOMMANDER_DATA_DIR"
)

// DataDir is the per-user directory for application data:
//
//	macOS:   ~/Library/Application Support/Commander
//	Windows: %AppData%\Commander
//	Linux:   $XDG_CONFIG_HOME/Commander (~/.config/Commander)
//
// Data from the old WailsCommander directory is moved here on first use.
func DataDir() (string, error) {
	for _, env := range []string{DataDirEnv, legacyDataDirEnv} {
		if dir := os.Getenv(env); dir != "" {
			return dir, nil
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return migrateLegacyDir(filepath.Join(base, legacyAppName), filepath.Join(base, AppName)), nil
}

// migrateLegacyDir moves legacy to current when only legacy exists. If the
// move fails the legacy directory keeps being used, so the vault is never
// left behind.
func migrateLegacyDir(legacy, current string) string {
	if exists(current) || !exists(legacy) {
		return current
	}
	if err := os.Rename(legacy, current); err != nil {
		return legacy
	}
	return current
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
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
