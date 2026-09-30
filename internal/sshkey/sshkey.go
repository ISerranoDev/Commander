// Package sshkey validates SSH private keys before they are stored.
package sshkey

import (
	"errors"

	"golang.org/x/crypto/ssh"
)

// MaxSize caps key files; real private keys are a few KiB at most.
const MaxSize = 64 * 1024

var (
	ErrInvalid          = errors.New("invalid private key")
	ErrPassphraseNeeded = errors.New("this private key requires a passphrase")
	ErrWrongPassphrase  = errors.New("wrong private key passphrase")
	ErrTooLarge         = errors.New("file is too large to be a private key")
)

// Inspect reports whether pem is a parseable private key and whether it is
// protected by a passphrase.
func Inspect(pem []byte) (encrypted bool, err error) {
	if len(pem) > MaxSize {
		return false, ErrTooLarge
	}
	_, err = ssh.ParseRawPrivateKey(pem)
	var missing *ssh.PassphraseMissingError
	switch {
	case err == nil:
		return false, nil
	case errors.As(err, &missing):
		return true, nil
	default:
		return false, ErrInvalid
	}
}

// Verify checks that pem can be decoded, using passphrase if it is encrypted.
func Verify(pem []byte, passphrase string) error {
	encrypted, err := Inspect(pem)
	if err != nil || !encrypted {
		return err
	}
	if passphrase == "" {
		return ErrPassphraseNeeded
	}
	if _, err := ssh.ParseRawPrivateKeyWithPassphrase(pem, []byte(passphrase)); err != nil {
		return ErrWrongPassphrase
	}
	return nil
}
