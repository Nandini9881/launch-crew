package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/session"
)

const defaultAPIURL = "https://api.github.com"

// inviteToken is the shape of an invite token as it appears in /s/<token>.
var inviteToken = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Verify proves a crew member is the GitHub user their PR comment was addressed to.
type Verify struct {
	OAuth    oauth2.Config
	Q        *sqlc.Queries
	Sessions *session.Store
	// HTTPClient is used for the token exchange and /user; defaults to http.DefaultClient.
	HTTPClient *http.Client
	// APIURL is the GitHub API root; defaults to https://api.github.com.
	APIURL string
}

// Start takes ?crew=<id>&t=<invite token> from the invite page and sends the person to GitHub.
func (v *Verify) Start(w http.ResponseWriter, r *http.Request) {
	crewID, err := uuid.Parse(r.URL.Query().Get("crew"))
	tok := r.URL.Query().Get("t")
	if err != nil || !inviteToken.MatchString(tok) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	row, err := v.Q.GetCrewMember(r.Context(), crewID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// The token must be this row's own invite, not any valid-looking string.
	if subtle.ConstantTimeCompare(crew.HashToken(tok), row.InviteTokenHash) != 1 {
		http.NotFound(w, r)
		return
	}
	if row.TokenExpiresAt.Valid && !time.Now().Before(row.TokenExpiresAt.Time) {
		http.Error(w, "this link has expired", http.StatusGone)
		return
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(nonce)
	sess := v.Sessions.Get(r)
	sess.VerifyState, sess.VerifyCrewID, sess.VerifyReturn = state, crewID, tok
	if err := v.Sessions.Set(w, sess); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, v.OAuth.AuthCodeURL(state), http.StatusFound)
}

// Callback exchanges the code, reads the GitHub user and compares it with the crew row.
func (v *Verify) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := v.Sessions.Get(r)
	// Consume the state up front so every attempt is single-use.
	spent := sess
	spent.VerifyState, spent.VerifyCrewID, spent.VerifyReturn = "", uuid.Nil, ""
	if err := v.Sessions.Set(w, spent); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	got := r.URL.Query().Get("state")
	if sess.VerifyState == "" || subtle.ConstantTimeCompare([]byte(got), []byte(sess.VerifyState)) != 1 {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || sess.VerifyCrewID == uuid.Nil || !inviteToken.MatchString(sess.VerifyReturn) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	back := "/s/" + sess.VerifyReturn

	member, err := v.Q.GetCrewMember(ctx, sess.VerifyCrewID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if v.HTTPClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, v.HTTPClient)
	}
	tok, err := v.OAuth.Exchange(ctx, code)
	if err != nil {
		log.Printf("github verify: exchange: %v", err)
		http.Error(w, "verification failed", http.StatusBadGateway)
		return
	}
	ghID, login, err := v.fetchUser(ctx, tok.AccessToken)
	if err != nil {
		log.Printf("github verify: user: %v", err)
		http.Error(w, "verification failed", http.StatusBadGateway)
		return
	}
	if !member.GithubUserID.Valid || member.GithubUserID.Int64 != ghID {
		http.Redirect(w, r, back+"?foreign=1", http.StatusFound)
		return
	}
	userID, fresh, conflict, err := v.resolveUser(ctx, sess.UserID, member, ghID)
	if err != nil {
		log.Printf("github verify: user row: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// A fresh user records the GitHub proof even when the row is taken.
	if !conflict || fresh {
		if _, err := v.Q.UpsertGitHubIdentity(ctx, sqlc.UpsertGitHubIdentityParams{
			GithubUserID: ghID, UserID: userID, Login: login,
		}); err != nil {
			log.Printf("github verify: identity: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if conflict {
		http.Redirect(w, r, back+"?foreign=1", http.StatusFound)
		return
	}
	if err := v.Q.UpdateCrewMemberUser(ctx, sqlc.UpdateCrewMemberUserParams{
		ID: member.ID, UserID: uuid.NullUUID{UUID: userID, Valid: true},
	}); err != nil {
		log.Printf("github verify: crew user: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Signed in as the verified user, so the crew row's mutating routes admit them.
	spent.UserID = userID
	if err := v.Sessions.Set(w, spent); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, back, http.StatusFound)
}

// resolveUser picks the user to bind: the user already tied to this GitHub
// account wins, then the signed-in user if they are a blank slate, else a
// new one (fresh). conflict reports a crew row already bound to someone else.
func (v *Verify) resolveUser(ctx context.Context, sessionUser uuid.UUID, m sqlc.CrewMember, ghID int64) (id uuid.UUID, fresh, conflict bool, err error) {
	switch ident, err := v.Q.GetGitHubIdentity(ctx, ghID); {
	case err == nil:
		id = ident.UserID
	case !errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, false, err
	case sessionUser != uuid.Nil:
		adopt, err := v.adoptable(ctx, sessionUser)
		if err != nil {
			return uuid.Nil, false, false, err
		}
		if adopt {
			id = sessionUser
		}
	}
	if id != uuid.Nil {
		return id, false, m.UserID.Valid && m.UserID.UUID != id, nil
	}
	// Never adopt the row's user: only a GitHub proof may bind a GitHub row.
	u, err := v.Q.CreateUser(ctx)
	if err != nil {
		return uuid.Nil, false, false, err
	}
	return u.ID, true, m.UserID.Valid, nil
}

// adoptable: on a shared browser the session may belong to someone else, so
// only a user with no GitHub identity and no launches is adopted.
func (v *Verify) adoptable(ctx context.Context, user uuid.UUID) (bool, error) {
	ids, err := v.Q.ListGitHubIdentitiesByUser(ctx, user)
	if err != nil || len(ids) > 0 {
		return false, err
	}
	n, err := v.Q.CountUserLaunches(ctx, uuid.NullUUID{UUID: user, Valid: true})
	return err == nil && n == 0, err
}

func (v *Verify) fetchUser(ctx context.Context, token string) (int64, string, error) {
	api := v.APIURL
	if api == "" {
		api = defaultAPIURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/user", nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	hc := v.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var u struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return 0, "", err
	}
	if u.ID == 0 {
		return 0, "", errors.New("no user id")
	}
	return u.ID, u.Login, nil
}
