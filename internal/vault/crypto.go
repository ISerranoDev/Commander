package vault

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// On-disk format (vault and export files share it):
//
//	salt (16 bytes) || nonce (24 bytes) || XChaCha20-Poly1305 ciphertext
//
// There is deliberately no magic number, version byte or visible KDF
// parameters: without the password the file is indistinguishable from
// random data. The plaintext is length-prefixed and padded to a multiple of
// padBlock so the file size leaks only a coarse upper bound of its content.
//
// KDF parameters are fixed per format revision. If they ever change, add the
// new set to kdfProfiles (newest first) and open() will try each in turn.
const (
	saltSize  = 16
	keySize   = chacha20poly1305.KeySize
	nonceSize = chacha20poly1305.NonceSizeX
	headerLen = saltSize + nonceSize
	padBlock  = 4096
)

type kdfProfile struct {
	time    uint32
	memory  uint32 // KiB
	threads uint8
}

var kdfProfiles = []kdfProfile{
	{time: 3, memory: 64 * 1024, threads: 4},
}

// ErrDecrypt is returned for a wrong password or a corrupted/tampered file.
// The two cases are intentionally indistinguishable.
var ErrDecrypt = errors.New("wrong password or corrupted file")

func deriveKey(password []byte, salt []byte, p kdfProfile) []byte {
	return argon2.IDKey(password, salt, p.time, p.memory, p.threads, keySize)
}

// sealed holds a derived key together with the salt it came from, so the
// vault can be re-encrypted on every save without re-running Argon2.
type sealed struct {
	key  []byte
	salt []byte
}

func newSealed(password []byte) (*sealed, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return &sealed{key: deriveKey(password, salt, kdfProfiles[0]), salt: salt}, nil
}

func (s *sealed) wipe() {
	if s == nil {
		return
	}
	for i := range s.key {
		s.key[i] = 0
	}
}

func (s *sealed) encrypt(plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(s.key)
	if err != nil {
		return nil, err
	}

	padded := pad(plaintext)
	defer wipeBytes(padded)

	out := make([]byte, headerLen, headerLen+len(padded)+aead.Overhead())
	copy(out, s.salt)
	if _, err := rand.Read(out[saltSize:headerLen]); err != nil {
		return nil, err
	}
	// The header is authenticated as associated data so the salt cannot be
	// swapped without detection.
	return aead.Seal(out, out[saltSize:headerLen], padded, out[:headerLen]), nil
}

// open decrypts data with password, returning the plaintext and a sealed
// key that can be reused to re-encrypt with the same salt.
func open(password []byte, data []byte) ([]byte, *sealed, error) {
	if len(data) < headerLen+chacha20poly1305.Overhead {
		return nil, nil, ErrDecrypt
	}
	salt := data[:saltSize]
	nonce := data[saltSize:headerLen]

	for _, p := range kdfProfiles {
		key := deriveKey(password, salt, p)
		aead, err := chacha20poly1305.NewX(key)
		if err != nil {
			return nil, nil, err
		}
		padded, err := aead.Open(nil, nonce, data[headerLen:], data[:headerLen])
		if err != nil {
			wipeBytes(key)
			continue
		}
		plaintext, err := unpad(padded)
		if err != nil {
			wipeBytes(key)
			return nil, nil, err
		}
		s := &sealed{key: key, salt: append([]byte(nil), salt...)}
		// Always re-encrypt with the current profile on next save.
		if p != kdfProfiles[0] {
			s.wipe()
			if s, err = newSealed(password); err != nil {
				return nil, nil, err
			}
		}
		return plaintext, s, nil
	}
	return nil, nil, ErrDecrypt
}

func pad(plaintext []byte) []byte {
	n := 4 + len(plaintext)
	size := (n/padBlock + 1) * padBlock
	buf := make([]byte, size)
	binary.BigEndian.PutUint32(buf, uint32(len(plaintext)))
	copy(buf[4:], plaintext)
	// Random padding rather than zeros: keeps the plaintext itself
	// uninteresting even if the cipher were ever misused.
	if _, err := rand.Read(buf[n:]); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return buf
}

func unpad(padded []byte) ([]byte, error) {
	if len(padded) < 4 {
		return nil, ErrDecrypt
	}
	n := binary.BigEndian.Uint32(padded)
	if int(n) > len(padded)-4 {
		return nil, ErrDecrypt
	}
	return padded[4 : 4+n], nil
}

func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
