// Package session keeps the signed-in user in an encrypted cookie.
package session

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
)

const (
	cookieName = "lc_s"
	maxAge     = 30 * 24 * time.Hour
)

// Session is the cookie payload. The zero value means signed out.
type Session struct {
	// IssuedAt is when Set wrote the cookie. Get refuses a session older
	// than maxAge, so a copied cookie dies on its own instead of living
	// until KEYRING rotates. Set fills it in; callers leave it alone.
	IssuedAt     time.Time
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
	// Now is the clock; nil means time.Now. Tests set it.
	Now func() time.Time
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Get returns the session, or the zero Session if the cookie is missing,
// invalid, tampered or older than maxAge.
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
	if sess.IssuedAt.IsZero() || s.now().Sub(sess.IssuedAt) > maxAge {
		return Session{}
	}
	return sess
}

// Set writes the session cookie, stamped with the current time.
func (s *Store) Set(w http.ResponseWriter, sess Session) error {
	sess.IssuedAt = s.now()
	plain, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	enc, _, err := s.Keyring.Encrypt(plain)
	if err != nil {
		return err
	}
	http.SetCookie(w, s.cookie(base64.RawURLEncoding.EncodeToString(enc), int(maxAge/time.Second)))
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
