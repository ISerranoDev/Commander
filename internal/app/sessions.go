package app

import (
	"errors"

	"github.com/ISerranoDev/WailsCommander/internal/model"
	"github.com/ISerranoDev/WailsCommander/internal/sshclient"
)

var errUnknownSecret = errors.New("unknown secret field")

// Connect opens an SSH session to a saved host in the background. The UI
// picks sessionID and subscribes to "ssh:*:<sessionID>" events first.
// password overrides the stored one (used when none is saved).
func (a *App) Connect(sessionID, hostID, password string, cols, rows int) error {
	h, err := a.vault.Host(hostID)
	if err != nil {
		return err
	}
	cfg := sshclient.Config{
		Address:  h.Address,
		Port:     h.Port,
		Username: h.Username,
		Cols:     cols,
		Rows:     rows,
	}
	switch h.AuthMethod {
	case model.AuthKey:
		cfg.PrivateKey, cfg.KeyPassphrase = h.PrivateKey, h.KeyPassphrase
	default:
		cfg.Password = h.Password
		if password != "" {
			cfg.Password = password
		}
	}
	return a.ssh.Start(sessionID, cfg)
}

func (a *App) AnswerHostKey(sessionID string, accept bool) error {
	return a.ssh.AnswerHostKey(sessionID, accept)
}

func (a *App) SendInput(sessionID, data string) error {
	return a.ssh.Write(sessionID, data)
}

func (a *App) ResizeSession(sessionID string, cols, rows int) error {
	return a.ssh.Resize(sessionID, cols, rows)
}

func (a *App) CloseSession(sessionID string) {
	a.ssh.Close(sessionID)
}

// RevealSecret returns a stored secret of a host so the UI can show it on
// demand (eye button). field is "password" or "keyPassphrase".
func (a *App) RevealSecret(hostID, field string) (string, error) {
	h, err := a.vault.Host(hostID)
	if err != nil {
		return "", err
	}
	switch field {
	case "password":
		return h.Password, nil
	case "keyPassphrase":
		return h.KeyPassphrase, nil
	}
	return "", errUnknownSecret
}
