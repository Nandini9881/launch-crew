// Package httpserver builds the HTTP handler: API routes plus the embedded front end.
package httpserver

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/shubham-tomar/launchcrew/internal/auth"
	"github.com/shubham-tomar/launchcrew/internal/identity"
)

//go:embed all:dist
var distFS embed.FS

// Deps holds the collaborators handlers need. Nil members leave their routes unmounted.
type Deps struct {
	BufferOAuth   *auth.BufferOAuth
	GitHubWebhook http.Handler
	Install       *identity.Install
	Verify        *identity.Verify
	// Crew serves the crew JSON routes, mounted at /api.
	Crew http.Handler
	// Captain serves the captain JSON routes, mounted at /api/captain.
	Captain http.Handler
	// Settings serves /me, /me/opt-out and /opt-out under /api.
	Settings http.Handler
	// E2E serves the test control routes at /e2e; set only when LAUNCHCREW_E2E=1.
	E2E http.Handler
}

// New returns the application handler.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	if oa := deps.BufferOAuth; oa != nil {
		r.Get("/auth/buffer/start", oa.Start)
		r.Get("/auth/buffer/callback", oa.Callback)
		r.Get("/auth/logout", oa.Logout)
	}
	if in := deps.Install; in != nil {
		r.Get("/github/install/start", in.Start)
		r.Get("/github/install/callback", in.Callback)
	}
	if v := deps.Verify; v != nil {
		r.Get("/github/verify/start", v.Start)
		r.Get("/github/verify/callback", v.Callback)
	}
	if h := deps.GitHubWebhook; h != nil {
		r.Method(http.MethodPost, "/webhooks/github", h)
	}
	if deps.E2E != nil {
		r.Mount("/e2e", deps.E2E)
	} else {
		r.Handle("/e2e/*", http.HandlerFunc(apiNotFound))
	}
	if deps.Captain != nil {
		r.Mount("/api/captain", deps.Captain)
	}
	if deps.Settings != nil {
		s := http.StripPrefix("/api", deps.Settings)
		for _, p := range []string{"/api/me", "/api/me/*", "/api/opt-out"} {
			r.Handle(p, s)
		}
	}
	if deps.Crew != nil {
		r.Mount("/api", deps.Crew)
	} else {
		r.Handle("/api/*", http.HandlerFunc(apiNotFound))
	}
	r.Handle("/*", spaHandler())
	return r
}

func apiNotFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"not found"}`))
}

// spaHandler serves built assets, falling back to index.html for client routes.
func spaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "front end not built", http.StatusServiceUnavailable)
			return
		}
		p := r.URL.Path
		if len(p) > 1 {
			if st, err := fs.Stat(sub, p[1:]); err == nil && !st.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		_, _ = w.Write(index)
	})
}
