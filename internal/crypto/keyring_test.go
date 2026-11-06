package crypto

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func testKey(fill byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	return key
}

func encodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func mustParse(t *testing.T, env string) *Keyring {
	t.Helper()
	kr, err := ParseKeyring(env)
	if err != nil {
		t.Fatalf("ParseKeyring(%q): %v", env, err)
	}
	return kr
}

func TestRoundTrip(t *testing.T) {
	kr := mustParse(t, "k2:"+encodeKey(testKey(2))+",k1:"+encodeKey(testKey(1)))
	plain := []byte("launch crew draft")

	cipherText, keyID, err := kr.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if keyID != "k2" {
		t.Fatalf("Encrypt keyID = %q, want k2", keyID)
	}
	if kr.CurrentKeyID() != "k2" {
		t.Fatalf("CurrentKeyID = %q, want k2", kr.CurrentKeyID())
	}
	if !bytes.HasPrefix(cipherText, []byte("k2:")) {
		t.Fatalf("ciphertext %q does not start with key id prefix k2:", cipherText)
	}

	got, err := kr.Decrypt(cipherText)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("Decrypt = %q, want %q", got, plain)
	}
}

func TestOldKeyDecrypts(t *testing.T) {
	plain := []byte("written before rotation")

	old := mustParse(t, "k1:"+encodeKey(testKey(1)))
	cipherText, _, err := old.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt under k1: %v", err)
	}

	rotated := mustParse(t, "k2:"+encodeKey(testKey(2))+",k1:"+encodeKey(testKey(1)))
	got, err := rotated.Decrypt(cipherText)
	if err != nil {
		t.Fatalf("Decrypt under rotated keyring: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("Decrypt = %q, want %q", got, plain)
	}
}

func TestWrongKeyFailsClosed(t *testing.T) {
	writer := mustParse(t, "k2:"+encodeKey(testKey(2)))
	cipherText, _, err := writer.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	tests := []struct {
		name    string
		keyring *Keyring
		input   []byte
	}{
		{"unknown key id", mustParse(t, "k3:"+encodeKey(testKey(3))), cipherText},
		{"same id, different key", mustParse(t, "k2:"+encodeKey(testKey(9))), cipherText},
		{"short input", writer, []byte("k2:short")},
		{"no key id separator", writer, []byte("nocolonhere")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.keyring.Decrypt(tt.input)
			if err == nil {
				t.Fatal("Decrypt succeeded, want error")
			}
			if len(got) != 0 {
				t.Fatalf("Decrypt returned %q alongside error, want empty plaintext", got)
			}
		})
	}
}

func TestParseRejectsBadBase64(t *testing.T) {
	_, err := ParseKeyring("k1:!!!not-base64!!!")
	if err == nil {
		t.Fatal("ParseKeyring accepted invalid base64")
	}
}

func TestParseRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{"empty env", ""},
		{"wrong key length", "k1:" + base64.StdEncoding.EncodeToString(make([]byte, 16))},
		{"empty key id", ":" + encodeKey(testKey(1))},
		{"duplicate key id", "k1:" + encodeKey(testKey(1)) + ",k1:" + encodeKey(testKey(2))},
		{"missing separator", "k1" + encodeKey(testKey(1))},
		{"trailing comma", "k1:" + encodeKey(testKey(1)) + ","},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kr, err := ParseKeyring(tt.env)
			if err == nil {
				t.Fatalf("ParseKeyring(%q) succeeded, want error", strings.TrimSpace(tt.env))
			}
			if kr != nil {
				t.Fatalf("ParseKeyring returned keyring alongside error")
			}
		})
	}
}

func TestMAC(t *testing.T) {
	a := mustParse(t, "k1:"+encodeKey(testKey(1)))
	a2 := mustParse(t, "k1:"+encodeKey(testKey(1)))
	b := mustParse(t, "k1:"+encodeKey(testKey(2)))
	data := []byte("payload")
	if !bytes.Equal(a.MAC(data), a2.MAC(data)) {
		t.Error("same key must give the same MAC")
	}
	if bytes.Equal(a.MAC(data), b.MAC(data)) {
		t.Error("different keys must give different MACs")
	}
	if bytes.Equal(a.MAC(data), a.MAC([]byte("other"))) {
		t.Error("different data must give different MACs")
	}
}

// The MAC key is derived from the AES key, never the AES key itself, so a
// MAC can leak nothing about the key that seals cookies and tokens.
func TestMACUsesDerivedKey(t *testing.T) {
	kr := mustParse(t, "k1:"+encodeKey(testKey(1)))
	data := []byte("payload")

	raw := hmac.New(sha256.New, testKey(1))
	raw.Write(data)
	if bytes.Equal(kr.MAC(data), raw.Sum(nil)) {
		t.Fatal("MAC is HMAC under the raw AES key")
	}

	// Stable for a fixed key: HMAC(HMAC(key, "launchcrew/mac/v1"), data).
	// Existing signed states must keep verifying across restarts.
	const want = "ff4acf2199004e8446b00c43ae49f70467c0e6de91c328d692bf08037cead018"
	if got := hex.EncodeToString(kr.MAC(data)); got != want {
		t.Fatalf("MAC = %s, want %s", got, want)
	}
}
