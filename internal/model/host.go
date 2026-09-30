package model

import (
	"errors"
	"strings"
	"time"
)

// Authentication methods for a host. A host uses exactly one.
const (
	AuthPassword = "password"
	AuthKey      = "key"
)

// Host is a saved connection. Secrets (Password, PrivateKey, KeyPassphrase)
// only ever live inside the encrypted vault and are never sent to the UI.
type Host struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Address       string    `json:"address"`
	Port          int       `json:"port"`
	Username      string    `json:"username"`
	AuthMethod    string    `json:"authMethod"`
	Password      string    `json:"password,omitempty"`
	PrivateKey    string    `json:"privateKey,omitempty"`
	KeyPassphrase string    `json:"keyPassphrase,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (h *Host) Normalize() {
	h.Name = strings.TrimSpace(h.Name)
	h.Address = strings.TrimSpace(h.Address)
	h.Username = strings.TrimSpace(h.Username)
	if h.Port == 0 {
		h.Port = 22
	}
	if h.AuthMethod == "" {
		h.AuthMethod = AuthPassword
		if h.PrivateKey != "" {
			h.AuthMethod = AuthKey
		}
	}
	// Only keep the secrets of the selected method.
	switch h.AuthMethod {
	case AuthPassword:
		h.PrivateKey, h.KeyPassphrase = "", ""
	case AuthKey:
		h.Password = ""
	}
}

func (h Host) Validate() error {
	var errs []error
	if h.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if h.Address == "" {
		errs = append(errs, errors.New("address is required"))
	}
	if h.Port < 1 || h.Port > 65535 {
		errs = append(errs, errors.New("port must be between 1 and 65535"))
	}
	if h.Username == "" {
		errs = append(errs, errors.New("username is required"))
	}
	switch h.AuthMethod {
	case AuthPassword:
	case AuthKey:
		if h.PrivateKey == "" {
			errs = append(errs, errors.New("private key is required"))
		}
	default:
		errs = append(errs, errors.New("invalid authentication method"))
	}
	return errors.Join(errs...)
}
