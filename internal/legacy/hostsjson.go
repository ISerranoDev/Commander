// Package legacy reads the plaintext hosts.json written by versions before
// the encrypted vault, so it can be migrated on first run.
package legacy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

type legacyHost struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Name string `json:"name"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

// HostsFile returns the path of the old hosts.json (next to the executable)
// if it exists.
func HostsFile() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	path := filepath.Join(filepath.Dir(exe), "hosts.json")
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

func ReadHosts(path string) ([]model.Host, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var in []legacyHost
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	out := make([]model.Host, 0, len(in))
	for _, h := range in {
		out = append(out, model.Host{
			Name:     h.Name,
			Address:  h.Host,
			Port:     h.Port,
			Username: h.User,
			Password: h.Pass,
		})
	}
	return out, nil
}
