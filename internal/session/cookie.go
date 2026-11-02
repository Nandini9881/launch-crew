// Package session keeps the signed-in user in an encrypted cookie.
package session

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
)

const (
	cookieName = "lc_s"
	maxAge     = 30 * 24 * 3600
)

// Session is the cookie payload. The zero value means signed out.
type Session struct {
	UserID       uuid.UUID
	ConnectionID uuid.UUID
	ReturnTo     string
	OAuthState   string
	PKCEVerifier string

	// GitHub App install binding.
	InstallState string
	// InstallID is the installation awaiting user authorization.
	InstallID int64
	// GitHub identity verification.
	VerifyState  string
	VerifyCrewID uuid.UUID
	VerifyReturn string
}

// Store reads and writes the encrypted session cookie.
type Store struct {
	Keyring *crypto.Keyring
	// PlainHTTP drops the Secure flag so a plain-http test server keeps its
	// cookie (WebKit refuses Secure cookies on http://localhost). E2E only.
	PlainHTTP bool
}

// Get returns the session, or the zero Session if the cookie is missing, invalid or tampered.
func (s *Store) Get(r *http.Request) Session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return Session{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return Session{}
	}
	plain, err := s.Keyring.Decrypt(raw)
	if err != nil {
		return Session{}
	}
	var sess Session
	if err := json.Unmarshal(plain, &sess); err != nil {
		return Session{}
	}
	return sess
}

// Set writes the session cookie.
func (s *Store) Set(w http.ResponseWriter, sess Session) error {
	plain, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	enc, _, err := s.Keyring.Encrypt(plain)
	if err != nil {
		return err
	}
	http.SetCookie(w, s.cookie(base64.RawURLEncoding.EncodeToString(enc), maxAge))
	return nil
}

// Clear expires the session cookie.
func (s *Store) Clear(w http.ResponseWriter) {
	http.SetCookie(w, s.cookie("", -1))
}

func (s *Store) cookie(value string, age int) *http.Cookie {
	return &http.Cookie{
		Name: cookieName, Value: value, Path: "/", MaxAge: age,
		HttpOnly: true, Secure: !s.PlainHTTP, SameSite: http.SameSiteLaxMode,
	}
}
