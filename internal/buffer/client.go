// Package buffer is the client for Buffer's GraphQL API. It owns token
// refresh, the rate budget and the buffer_calls audit; everything that
// talks to Buffer goes through Client.Do.
package buffer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

const (
	defaultAPIURL   = "https://api.buffer.com"
	defaultTokenURL = "https://auth.buffer.com/token"

	// requestIDHeader is not documented by Buffer; see SCHEMA.md.
	requestIDHeader = "X-Request-Id"

	statusOK             = "ok"
	statusNeedsReconnect = "needs_reconnect"

	refreshSkew  = 60 * time.Second
	maxWait      = 15 * time.Minute // never sleep past one budget window
	cacheTTL     = time.Hour
	fallbackTTL  = 15 * time.Minute // built-in rules kept while Buffer's configuration query fails
	limitsTTL    = time.Minute      // dailyPostingLimits kept in memory per connection
	refreshFloor = 15 * time.Second // RefreshChannels reuses a cache younger than this
	refreshLimit = 30 * time.Second
	maxPage      = 100
)

// DraftNote is CreatePostResult.Note when a full queue made the post a draft.
const DraftNote = "your queue is full, saved as a draft instead"

// ErrNeedsReconnect means Buffer rejected the refresh token; the user must sign in again.
var ErrNeedsReconnect = errors.New("buffer: connection needs reconnect")

var errInvalidGrant = errors.New("invalid_grant")

// LimitReachedError is createPost's LimitReachedError: the channel's queue is full.
type LimitReachedError struct{ Message string }

func (e *LimitReachedError) Error() string { return "buffer: limit reached: " + e.Message }

// MutationError is any other typed mutation error (InvalidInputError and so on).
type MutationError struct{ Type, Message string }

func (e *MutationError) Error() string { return fmt.Sprintf("buffer: %s: %s", e.Type, e.Message) }

// HTTPError is a non-2xx answer other than 401 and 429.
type HTTPError struct {
	Op      string
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	msg := ""
	if e.Message != "" {
		msg = ": " + e.Message
	}
	return fmt.Sprintf("buffer: %s: HTTP %d%s", e.Op, e.Status, msg)
}

// GraphQLError is the first entry of a response's errors array.
type GraphQLError struct{ Op, Code, Message string }

func (e *GraphQLError) Error() string {
	return fmt.Sprintf("buffer: %s: %s (%s)", e.Op, e.Message, e.Code)
}

// Client talks to Buffer on behalf of stored connections.
type Client struct {
	hc                     *http.Client
	q                      *sqlc.Queries
	kr                     *crypto.Keyring
	apiURL, tokenURL       string
	clientID, clientSecret string
	now                    func() time.Time
	sleep                  func(time.Duration)

	mu      sync.Mutex
	locks   map[uuid.UUID]*sync.Mutex // per-connection refresh locks
	windows map[uuid.UUID]int         // largest 15-min quota seen
	limits  map[uuid.UUID]limitsEntry // DailyPostingLimits answers, limitsTTL old at most
}

type limitsEntry struct {
	at     time.Time
	limits map[string]Limit
}

// Option configures a Client.
type Option func(*Client)

// WithEndpoints overrides the API and token URLs.
func WithEndpoints(apiURL, tokenURL string) Option {
	return func(c *Client) { c.apiURL, c.tokenURL = apiURL, tokenURL }
}

// WithCredentials sets the OAuth client id and secret used to refresh tokens.
func WithCredentials(id, secret string) Option {
	return func(c *Client) { c.clientID, c.clientSecret = id, secret }
}

// WithClock replaces time.Now and the sleeper used for Retry-After and budget waits.
func WithClock(now func() time.Time, sleep func(time.Duration)) Option {
	return func(c *Client) { c.now, c.sleep = now, sleep }
}

// New returns a client. httpClient should carry a timeout.
func New(httpClient *http.Client, q *sqlc.Queries, kr *crypto.Keyring, opts ...Option) *Client {
	c := &Client{
		hc: httpClient, q: q, kr: kr,
		apiURL: defaultAPIURL, tokenURL: defaultTokenURL,
		now:   time.Now,
		locks: map[uuid.UUID]*sync.Mutex{}, windows: map[uuid.UUID]int{},
		limits: map[uuid.UUID]limitsEntry{},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type launchKey struct{}

// WithLaunch tags the Buffer calls made with ctx with a launch id in the audit.
func WithLaunch(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, launchKey{}, id)
}

func (c *Client) wait(ctx context.Context, d time.Duration) error {
	if c.sleep != nil {
		c.sleep(d)
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var opRe = regexp.MustCompile(`^\s*(?:query|mutation)\s+([A-Za-z_][A-Za-z0-9_]*)`)

func opName(query string) string {
	if m := opRe.FindStringSubmatch(query); m != nil {
		return m[1]
	}
	return "anonymous"
}

type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

// Do runs one GraphQL operation for a connection and decodes "data" into out.
// It refreshes the token when it expires within 60 s or on a 401 (then
// retries once), waits once on a 429's Retry-After, and waits for the
// window reset when less than 20% of the 15-minute budget is left.
func (c *Client) Do(ctx context.Context, connID uuid.UUID, query string, vars map[string]any, out any) error {
	op := opName(query)
	conn, err := c.q.GetBufferConnection(ctx, connID)
	if err != nil {
		return fmt.Errorf("buffer: load connection: %w", err)
	}
	if conn.Status == statusNeedsReconnect {
		return ErrNeedsReconnect
	}
	if err := c.waitBudget(ctx, conn); err != nil {
		return err
	}
	tok, err := c.decrypt(conn.AccessTokenEnc)
	if err != nil {
		return err
	}
	if c.expiringSoon(conn) {
		if tok, err = c.refresh(ctx, connID, tok, false); err != nil {
			return err
		}
	}
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}

	refreshed, waited := false, false
	var waitedFor time.Duration
	for {
		status, hdr, raw, err := c.send(ctx, connID, op, tok, body)
		if err != nil {
			return err
		}
		var env struct {
			Data   json.RawMessage `json:"data"`
			Errors []gqlError      `json:"errors"`
		}
		_ = json.Unmarshal(raw, &env)
		unauthorized := status == http.StatusUnauthorized ||
			(len(env.Errors) > 0 && env.Errors[0].Extensions.Code == "UNAUTHORIZED")

		switch {
		case unauthorized && !refreshed:
			refreshed = true
			if tok, err = c.refresh(ctx, connID, tok, true); err != nil {
				return err
			}
			continue
		case unauthorized:
			return fmt.Errorf("buffer: %s: unauthorized after token refresh", op)
		case status == http.StatusTooManyRequests && !waited:
			waited, waitedFor = true, retryAfter(hdr)
			if waitedFor > maxWait {
				return fmt.Errorf("buffer: %s: rate limited; Buffer asks to retry after %s, longer than the %s cap",
					op, waitedFor, maxWait)
			}
			if err := c.wait(ctx, waitedFor); err != nil {
				return err
			}
			continue
		case status == http.StatusTooManyRequests:
			return fmt.Errorf("buffer: %s: rate limited again after waiting %s; Buffer asks to retry after %s",
				op, waitedFor, retryAfter(hdr))
		case status/100 != 2:
			msg := ""
			if len(env.Errors) > 0 {
				msg = env.Errors[0].Message
			}
			return &HTTPError{Op: op, Status: status, Message: msg}
		}
		if len(env.Errors) > 0 {
			return &GraphQLError{Op: op, Code: env.Errors[0].Extensions.Code, Message: env.Errors[0].Message}
		}
		if out != nil {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("buffer: %s: decode: %w", op, err)
			}
		}
		return nil
	}
}

func retryAfter(h http.Header) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After"))); err == nil && s >= 0 {
		return time.Duration(s) * time.Second
	}
	return time.Minute
}

// send makes one API request and records it in buffer_calls and the rate columns.
func (c *Client) send(ctx context.Context, connID uuid.UUID, op, tok string, body []byte) (int, http.Header, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		c.audit(ctx, connID, op, 0, "", pgtype.Int4{})
		return 0, nil, nil, fmt.Errorf("buffer: %s: %w", op, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))

	var remaining pgtype.Int4
	if r, ok := parseRate(resp.Header, c.now()); ok {
		remaining = pgtype.Int4{Int32: int32(r.remaining), Valid: true}
		c.noteWindow(connID, r)
		if err := c.q.UpdateConnectionRate(context.WithoutCancel(ctx), sqlc.UpdateConnectionRateParams{
			ID: connID, RateRemaining: remaining,
			RateResetAt: pgtype.Timestamptz{Time: r.reset, Valid: true},
		}); err != nil {
			log.Printf("buffer: save rate: %v", err)
		}
	}
	c.audit(ctx, connID, op, resp.StatusCode, resp.Header.Get(requestIDHeader), remaining)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("buffer: %s: read body: %w", op, err)
	}
	return resp.StatusCode, resp.Header, raw, nil
}

func (c *Client) audit(ctx context.Context, connID uuid.UUID, op string, status int, requestID string, remaining pgtype.Int4) {
	launch, ok := ctx.Value(launchKey{}).(uuid.UUID)
	err := c.q.InsertBufferCall(context.WithoutCancel(ctx), sqlc.InsertBufferCallParams{
		LaunchID:      uuid.NullUUID{UUID: launch, Valid: ok},
		ConnectionID:  connID,
		Op:            op,
		Status:        int32(status),
		RequestID:     pgtype.Text{String: requestID, Valid: requestID != ""},
		RateRemaining: remaining,
	})
	if err != nil {
		log.Printf("buffer: audit %s: %v", op, err)
	}
}

func (c *Client) decrypt(b []byte) (string, error) {
	p, err := c.kr.Decrypt(b)
	if err != nil {
		return "", fmt.Errorf("buffer: decrypt token: %w", err)
	}
	return string(p), nil
}

func (c *Client) expiringSoon(conn sqlc.BufferConnection) bool {
	return conn.ExpiresAt.Valid && conn.ExpiresAt.Time.Sub(c.now()) < refreshSkew
}

func (c *Client) connLock(id uuid.UUID) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.locks[id]
	if !ok {
		m = &sync.Mutex{}
		c.locks[id] = m
	}
	return m
}

// refresh swaps the refresh token for a new pair under the connection's lock
// and returns the access token to use. stale is the access token the caller
// holds; if the stored one differs, another goroutine already refreshed.
// force refreshes even when expires_at is far out (after a 401).
func (c *Client) refresh(ctx context.Context, connID uuid.UUID, stale string, force bool) (string, error) {
	mu := c.connLock(connID)
	mu.Lock()
	defer mu.Unlock()

	// Once Buffer answers, the old refresh token is spent, so finish
	// persisting even if the caller gives up.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), refreshLimit)
	defer cancel()

	conn, err := c.q.GetBufferConnection(ctx, connID)
	if err != nil {
		return "", fmt.Errorf("buffer: load connection: %w", err)
	}
	if conn.Status == statusNeedsReconnect {
		return "", ErrNeedsReconnect
	}
	cur, err := c.decrypt(conn.AccessTokenEnc)
	if err != nil {
		return "", err
	}
	if cur != stale || (!force && !c.expiringSoon(conn)) {
		return cur, nil
	}
	if len(conn.RefreshTokenEnc) == 0 {
		return "", c.markNeedsReconnect(ctx, connID)
	}
	rt, err := c.decrypt(conn.RefreshTokenEnc)
	if err != nil {
		return "", err
	}
	tok, err := c.exchange(ctx, rt)
	if errors.Is(err, errInvalidGrant) {
		// The refresh lock is per process. Another machine (two overlap
		// during a deploy) may have spent this refresh token and stored
		// a good pair meanwhile; then its tokens stand and nothing is wrong.
		if fresh, ok, ferr := c.foreignRefresh(ctx, connID, conn.AccessTokenEnc); ferr != nil {
			return "", ferr
		} else if ok {
			return fresh, nil
		}
		return "", c.markNeedsReconnect(ctx, connID)
	}
	if err != nil {
		return "", fmt.Errorf("buffer: refresh token: %w", err)
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = rt
	}
	access, keyID, err := c.kr.Encrypt([]byte(tok.AccessToken))
	if err != nil {
		return "", err
	}
	refresh, _, err := c.kr.Encrypt([]byte(tok.RefreshToken))
	if err != nil {
		return "", err
	}
	var exp pgtype.Timestamptz
	if tok.ExpiresIn > 0 {
		exp = pgtype.Timestamptz{Time: c.now().Add(time.Duration(tok.ExpiresIn) * time.Second), Valid: true}
	}
	if err := c.q.UpdateConnectionTokens(ctx, sqlc.UpdateConnectionTokensParams{
		ID: connID, AccessTokenEnc: access, RefreshTokenEnc: refresh, KeyID: keyID, ExpiresAt: exp,
	}); err != nil {
		return "", fmt.Errorf("buffer: save refreshed tokens: %w", err)
	}
	return tok.AccessToken, nil
}

// foreignRefresh re-reads the connection and, when its access token is no
// longer the one this refresh started from, returns the stored token.
func (c *Client) foreignRefresh(ctx context.Context, connID uuid.UUID, startedWith []byte) (string, bool, error) {
	conn, err := c.q.GetBufferConnection(ctx, connID)
	if err != nil {
		return "", false, fmt.Errorf("buffer: load connection: %w", err)
	}
	if conn.Status == statusNeedsReconnect || bytes.Equal(conn.AccessTokenEnc, startedWith) {
		return "", false, nil
	}
	tok, err := c.decrypt(conn.AccessTokenEnc)
	if err != nil {
		return "", false, err
	}
	log.Printf("buffer: connection %s was refreshed elsewhere; using the stored token", connID)
	return tok, true, nil
}

func (c *Client) markNeedsReconnect(ctx context.Context, connID uuid.UUID) error {
	if err := c.q.UpdateConnectionStatus(ctx, sqlc.UpdateConnectionStatusParams{ID: connID, Status: statusNeedsReconnect}); err != nil {
		return fmt.Errorf("buffer: mark needs_reconnect: %w", err)
	}
	return ErrNeedsReconnect
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func (c *Client) exchange(ctx context.Context, refreshToken string) (tokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.clientID},
	}
	if c.clientSecret != "" {
		form.Set("client_secret", c.clientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.hc.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var tr tokenResponse
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tr)
	switch {
	case resp.StatusCode == http.StatusBadRequest && tr.Error == "invalid_grant":
		return tokenResponse{}, errInvalidGrant
	case resp.StatusCode != http.StatusOK:
		return tokenResponse{}, fmt.Errorf("token endpoint HTTP %d %s", resp.StatusCode, tr.Error)
	case tr.AccessToken == "":
		return tokenResponse{}, errors.New("token endpoint returned no access_token")
	}
	return tr, nil
}
