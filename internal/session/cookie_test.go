package session_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/session"
)

func newStore(t *testing.T) *session.Store {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	kr, err := crypto.ParseKeyring("k1:" + key)
	if err != nil {
		t.Fatal(err)
	}
	return &session.Store{Keyring: kr}
}

func TestSessionRoundTrip(t *testing.T) {
	st := newStore(t)
	want := session.Session{UserID: uuid.New(), ConnectionID: uuid.New(), ReturnTo: "/s/abc", OAuthState: "st", PKCEVerifier: "v"}

	rr := httptest.NewRecorder()
	if err := st.Set(rr, want); err != nil {
		t.Fatal(err)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies", len(cookies))
	}
	c := cookies[0]
	if c.Name != "lc_s" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 30*24*3600 {
		t.Fatalf("cookie attrs: %+v", c)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(c)
	if got := st.Get(req); got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}

	for name, val := range map[string]string{
		"tampered": c.Value[:len(c.Value)-2] + "AA",
		"garbage":  "!!not-base64!!",
		"empty":    "",
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "lc_s", Value: val})
		if got := st.Get(req); got != (session.Session{}) {
			t.Fatalf("%s: got %+v", name, got)
		}
	}
	if got := st.Get(httptest.NewRequest("GET", "/", nil)); got != (session.Session{}) {
		t.Fatalf("missing cookie: got %+v", got)
	}

	rr = httptest.NewRecorder()
	st.Clear(rr)
	if c := rr.Result().Cookies()[0]; c.Name != "lc_s" || c.MaxAge >= 0 {
		t.Fatalf("clear: %+v", c)
	}
}
