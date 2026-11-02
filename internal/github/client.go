// Package github is Launch Crew's GitHub App client: release history, PR
// comments and the user-to-server OAuth config.
package github

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gh "github.com/google/go-github/v93/github"
	"golang.org/x/oauth2"
	oauthgh "golang.org/x/oauth2/github"

	"github.com/shubham-tomar/launchcrew/internal/config"
)

const defaultBaseURL = "https://api.github.com"

// Client talks to GitHub as the App, one cached installation client per installation.
type Client struct {
	appID      int64
	apps       *ghinstallation.AppsTransport
	baseURL    string
	transport  http.RoundTripper
	timeout    time.Duration
	oauthID    string
	oauthKey   string
	publicBase string

	mu       sync.Mutex
	installs map[int64]*gh.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL points the client at another API root, such as githubfake.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithTransport sets the underlying HTTP transport.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.transport = rt }
}

// WithOAuth sets the App's OAuth credentials and the public base URL for the callback.
func WithOAuth(clientID, clientSecret, publicBase string) Option {
	return func(c *Client) {
		c.oauthID, c.oauthKey, c.publicBase = clientID, clientSecret, strings.TrimRight(publicBase, "/")
	}
}

// New builds a client for the App appID signing with the PEM privateKey.
func New(appID int64, privateKey []byte, opts ...Option) (*Client, error) {
	c := &Client{
		appID:     appID,
		baseURL:   defaultBaseURL,
		transport: http.DefaultTransport,
		timeout:   20 * time.Second,
		installs:  map[int64]*gh.Client{},
	}
	for _, o := range opts {
		o(c)
	}
	apps, err := ghinstallation.NewAppsTransport(c.transport, appID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("github app key: %w", err)
	}
	apps.BaseURL = c.baseURL
	c.apps = apps
	return c, nil
}

// NewFromConfig builds a client from the GitHub settings in cfg.
func NewFromConfig(cfg config.Config, opts ...Option) (*Client, error) {
	id, err := strconv.ParseInt(cfg.GitHubAppID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("GITHUB_APP_ID: %w", err)
	}
	opts = append([]Option{WithOAuth(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.BaseURL)}, opts...)
	return New(id, []byte(cfg.GitHubPrivateKey), opts...)
}

// installation returns the go-github client acting as installation id.
func (c *Client) installation(id int64) (*gh.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cl, ok := c.installs[id]; ok {
		return cl, nil
	}
	tr := ghinstallation.NewFromAppsTransport(c.apps, id)
	tr.BaseURL = c.baseURL
	base := c.baseURL + "/"
	cl, err := gh.NewClient(
		gh.WithHTTPClient(&http.Client{Transport: tr, Timeout: c.timeout}),
		gh.WithURLs(&base, &base),
	)
	if err != nil {
		return nil, err
	}
	c.installs[id] = cl
	return cl, nil
}

// UserToServerConfig is the OAuth config used to verify a person's GitHub
// identity. It asks for no scopes: identity only.
func (c *Client) UserToServerConfig() oauth2.Config {
	return c.UserToServerConfigFor("/github/verify/callback")
}

// UserToServerConfigFor is UserToServerConfig with another callback path.
func (c *Client) UserToServerConfigFor(path string) oauth2.Config {
	return oauth2.Config{
		ClientID:     c.oauthID,
		ClientSecret: c.oauthKey,
		Endpoint:     oauthgh.Endpoint,
		RedirectURL:  c.publicBase + path,
	}
}
