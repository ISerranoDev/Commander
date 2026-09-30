package sshkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"testing"

	"golang.org/x/crypto/ssh"
)

func genKey(t *testing.T, passphrase string) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(block)
}

func TestInspectAndVerify(t *testing.T) {
	plain := genKey(t, "")
	if enc, err := Inspect(plain); err != nil || enc {
		t.Fatalf("plain key: enc=%v err=%v", enc, err)
	}
	if err := Verify(plain, ""); err != nil {
		t.Fatal(err)
	}

	locked := genKey(t, "hunter22")
	if enc, err := Inspect(locked); err != nil || !enc {
		t.Fatalf("encrypted key: enc=%v err=%v", enc, err)
	}
	if err := Verify(locked, ""); !errors.Is(err, ErrPassphraseNeeded) {
		t.Fatalf("want ErrPassphraseNeeded, got %v", err)
	}
	if err := Verify(locked, "nope"); !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("want ErrWrongPassphrase, got %v", err)
	}
	if err := Verify(locked, "hunter22"); err != nil {
		t.Fatal(err)
	}

	if _, err := Inspect([]byte("ssh-ed25519 AAAA... this is a public key")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("public key must be rejected, got %v", err)
	}
}
