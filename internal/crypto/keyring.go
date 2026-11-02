// Package crypto encrypts stored secrets with AES-GCM under a keyring that
// supports key rotation.
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	keySize   = 32
	nonceSize = 12
)

// Keyring holds AES-256-GCM keys by id. The first key in KEYRING encrypts;
// every key can decrypt.
type Keyring struct {
	currentID string
	keys      map[string]cipher.AEAD
	raw       map[string][]byte
}

// ParseKeyring parses KEYRING in the form "k2:<base64>,k1:<base64>".
func ParseKeyring(env string) (*Keyring, error) {
	if env == "" {
		return nil, errors.New("keyring: KEYRING is empty")
	}

	kr := &Keyring{keys: make(map[string]cipher.AEAD), raw: make(map[string][]byte)}
	for i, entry := range strings.Split(env, ",") {
		id, encoded, found := strings.Cut(entry, ":")
		if !found {
			return nil, fmt.Errorf("keyring: entry %d: missing ':' between key id and key", i)
		}
		if id == "" {
			return nil, fmt.Errorf("keyring: entry %d: empty key id", i)
		}
		if _, dup := kr.keys[id]; dup {
			return nil, fmt.Errorf("keyring: duplicate key id %q", id)
		}

		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("keyring: key %q: invalid base64: %w", id, err)
		}
		if len(key) != keySize {
			return nil, fmt.Errorf("keyring: key %q: got %d bytes, want %d", id, len(key), keySize)
		}

		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("keyring: key %q: %w", id, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("keyring: key %q: %w", id, err)
		}

		if i == 0 {
			kr.currentID = id
		}
		kr.keys[id] = aead
		kr.raw[id] = key
	}
	return kr, nil
}

// CurrentKeyID returns the id of the key used to encrypt.
func (k *Keyring) CurrentKeyID() string {
	return k.currentID
}

// Encrypt seals plain under the current key and returns the ciphertext
// (keyID ":" nonce sealed) along with the key id used.
func (k *Keyring) Encrypt(plain []byte) ([]byte, string, error) {
	aead := k.keys[k.currentID]

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", fmt.Errorf("keyring: nonce: %w", err)
	}

	out := make([]byte, 0, len(k.currentID)+1+nonceSize+len(plain)+aead.Overhead())
	out = append(out, k.currentID...)
	out = append(out, ':')
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plain, nil)
	return out, k.currentID, nil
}

// Decrypt opens ciphertext produced by Encrypt under any key in the keyring.
// It fails closed: on any error the plaintext is nil.
func (k *Keyring) Decrypt(ciphertext []byte) ([]byte, error) {
	id, rest, found := bytes.Cut(ciphertext, []byte(":"))
	if !found {
		return nil, errors.New("keyring: ciphertext has no key id")
	}
	aead, ok := k.keys[string(id)]
	if !ok {
		return nil, fmt.Errorf("keyring: unknown key id %q", id)
	}
	if len(rest) < nonceSize {
		return nil, errors.New("keyring: ciphertext too short")
	}

	nonce, sealed := rest[:nonceSize], rest[nonceSize:]
	plain, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, errors.New("keyring: decryption failed")
	}
	return plain, nil
}

// MAC returns HMAC-SHA256 of data under the current key.
func (k *Keyring) MAC(data []byte) []byte {
	m := hmac.New(sha256.New, k.raw[k.currentID])
	m.Write(data)
	return m.Sum(nil)
}
