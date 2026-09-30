package sshclient

import (
	"encoding/base64"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// KnownHosts is where trusted server keys are kept (the encrypted vault).
type KnownHosts interface {
	KnownHost(hostport string) (key string, ok bool, err error)
	TrustHost(hostport, key string) error
}

// HostKeyPrompt describes an unknown or changed server key for the user.
type HostKeyPrompt struct {
	Host        string `json:"host"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	// Changed means a different key was trusted before: possible
	// man-in-the-middle, or the server was reinstalled.
	Changed bool `json:"changed"`
}

var ErrHostKeyRejected = errors.New("host key rejected")

type keyStatus int

const (
	keyTrusted keyStatus = iota
	keyUnknown
	keyChanged
)

func encodeKey(key ssh.PublicKey) string {
	return key.Type() + " " + base64.StdEncoding.EncodeToString(key.Marshal())
}

// checkHostKey looks the key up in the vault first, then in the user's
// OpenSSH known_hosts (read-only) so hosts already trusted there are not
// asked about again.
func checkHostKey(store KnownHosts, hostport string, remote net.Addr, key ssh.PublicKey) (keyStatus, error) {
	encoded := encodeKey(key)
	stored, ok, err := store.KnownHost(hostport)
	if err != nil {
		return 0, err
	}
	if ok {
		if stored == encoded {
			return keyTrusted, nil
		}
		return keyChanged, nil
	}

	switch systemKnownHosts(hostport, remote, key) {
	case keyTrusted:
		return keyTrusted, store.TrustHost(hostport, encoded)
	case keyChanged:
		return keyChanged, nil
	}
	return keyUnknown, nil
}

func systemKnownHosts(hostport string, remote net.Addr, key ssh.PublicKey) keyStatus {
	home, err := os.UserHomeDir()
	if err != nil {
		return keyUnknown
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(path); err != nil {
		return keyUnknown
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return keyUnknown
	}
	err = callback(hostport, remote, key)
	var keyErr *knownhosts.KeyError
	switch {
	case err == nil:
		return keyTrusted
	case errors.As(err, &keyErr) && len(keyErr.Want) > 0:
		// Only a mismatch for the same key type means "changed"; a host
		// known with a different algorithm is just unknown for this one.
		for _, want := range keyErr.Want {
			if want.Key.Type() == key.Type() {
				return keyChanged
			}
		}
	}
	return keyUnknown
}

func promptFor(hostport string, key ssh.PublicKey, status keyStatus) HostKeyPrompt {
	return HostKeyPrompt{
		Host:        hostport,
		KeyType:     strings.TrimPrefix(key.Type(), "ssh-"),
		Fingerprint: ssh.FingerprintSHA256(key),
		Changed:     status == keyChanged,
	}
}
