// Package bufferfake is an in-process fake of Buffer's OAuth server and GraphQL API.
package bufferfake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// RefreshMode scripts how the token endpoint answers grant_type=refresh_token.
type RefreshMode int

const (
	RefreshOK           RefreshMode = iota // issue a new access and refresh token
	RefreshInvalidGrant                    // 400 {"error":"invalid_grant"}
	RefreshDrop                            // close the connection without a response
	RefreshServerError                     // 500
)

// Fake serves /oauth/authorize, /oauth/token and a GraphQL endpoint at /.
type Fake struct {
	Server *httptest.Server

	mu          sync.Mutex
	tokenForms  []url.Values
	tokenSeq    int
	refreshMode RefreshMode
	usedRefresh map[string]bool
	onToken     func()

	requests  []Request
	defaults  map[string]Response
	scripts   map[string][]Response
	funcs     map[string]func(Request) Response
	rejected  map[string]bool
	onRequest func(Request)
	reqSeq    int
}

// New starts a fake whose Account query returns the given account and org ids.
func New(t *testing.T, accountID, orgID string) *Fake {
	t.Helper()
	f := NewServer(accountID, orgID)
	t.Cleanup(f.Server.Close)
	return f
}

// NewServer is New without a *testing.T (the E2E server uses it); the
// caller closes f.Server.
func NewServer(accountID, orgID string) *Fake {
	f := &Fake{
		usedRefresh: map[string]bool{},
		defaults:    map[string]Response{},
		scripts:     map[string][]Response{},
		funcs:       map[string]func(Request) Response{},
		rejected:    map[string]bool{},
	}
	f.Handle("Account", map[string]any{"account": map[string]any{
		"id": accountID, "organizations": []map[string]any{{"id": orgID}},
	}})
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/authorize", f.authorize)
	mux.HandleFunc("/oauth/token", f.token)
	mux.HandleFunc("/", f.graphql)
	f.Server = httptest.NewServer(mux)
	return f
}

// AuthURL is the fake authorize page; it approves at once.
func (f *Fake) AuthURL() string { return f.Server.URL + "/oauth/authorize" }

// authorize redirects straight back to redirect_uri with a code and the state.
func (f *Fake) authorize(w http.ResponseWriter, r *http.Request) {
	back, err := url.Parse(r.URL.Query().Get("redirect_uri"))
	if err != nil || back.Scheme == "" {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	q := back.Query()
	q.Set("code", "fake-code")
	q.Set("state", r.URL.Query().Get("state"))
	back.RawQuery = q.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

// TokenURL is the fake token endpoint.
func (f *Fake) TokenURL() string { return f.Server.URL + "/oauth/token" }

// APIURL is the fake GraphQL endpoint.
func (f *Fake) APIURL() string { return f.Server.URL }

// TokenForms returns every form posted to /oauth/token.
func (f *Fake) TokenForms() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values(nil), f.tokenForms...)
}

// RefreshForms returns the forms posted with grant_type=refresh_token.
func (f *Fake) RefreshForms() []url.Values {
	var out []url.Values
	for _, form := range f.TokenForms() {
		if form.Get("grant_type") == "refresh_token" {
			out = append(out, form)
		}
	}
	return out
}

// SetRefresh scripts the answer to later refresh requests.
func (f *Fake) SetRefresh(m RefreshMode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshMode = m
}

// OnToken runs fn at the start of every token request, e.g. to add a delay.
func (f *Fake) OnToken(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onToken = fn
}

func (f *Fake) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.tokenForms = append(f.tokenForms, r.PostForm)
	hook := f.onToken
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	if r.PostForm.Get("grant_type") == "refresh_token" {
		f.refresh(w, r.PostForm.Get("refresh_token"))
		return
	}
	if r.PostForm.Get("code_verifier") == "" {
		http.Error(w, `{"error":"invalid_request","error_description":"code_verifier required"}`, http.StatusBadRequest)
		return
	}
	f.issue(w)
}

// refresh treats refresh tokens as single-use, like Buffer: a reused one gets invalid_grant.
func (f *Fake) refresh(w http.ResponseWriter, refreshToken string) {
	f.mu.Lock()
	mode := f.refreshMode
	reused := f.usedRefresh[refreshToken]
	if mode == RefreshOK && !reused {
		f.usedRefresh[refreshToken] = true
	}
	f.mu.Unlock()
	switch {
	case mode == RefreshDrop:
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	case mode == RefreshServerError:
		http.Error(w, `{"error":"server_error"}`, http.StatusInternalServerError)
		return
	case mode == RefreshInvalidGrant || reused || refreshToken == "":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token is invalid"}`))
		return
	}
	f.issue(w)
}

func (f *Fake) issue(w http.ResponseWriter) {
	f.mu.Lock()
	f.tokenSeq++
	n := f.tokenSeq
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":  fmt.Sprintf("access-%d", n),
		"refresh_token": fmt.Sprintf("refresh-%d", n),
		"token_type":    "Bearer",
		"expires_in":    3600,
	})
}
