// Package r2fake is an in-memory stand-in for the S3-compatible R2 API:
// path-style PUT, GET and HEAD of objects in one bucket.
package r2fake

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// Object is one stored object.
type Object struct {
	Body         []byte
	ContentType  string
	CacheControl string
}

// Server is the fake. URL is its base; objects live at URL/<bucket>/<key>.
type Server struct {
	*httptest.Server
	Bucket string

	mu      sync.Mutex
	objects map[string]Object
	missing bool
}

// SetMissing makes every GET and HEAD answer 404 while on, as if the
// objects were gone (E2E uses it to turn the card check red).
func (s *Server) SetMissing(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.missing = on
}

// New starts a fake serving bucket.
func New(bucket string) *Server {
	s := &Server{Bucket: bucket, objects: map[string]Object{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// Object returns the stored object at key.
func (s *Server) Object(key string) (Object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[key]
	return o, ok
}

// Len is the number of stored objects.
func (s *Server) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.objects)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	key, ok := strings.CutPrefix(r.URL.Path, "/"+s.Bucket+"/")
	if !ok || key == "" {
		http.Error(w, "NoSuchBucket", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.objects[key] = Object{
			Body:         body,
			ContentType:  r.Header.Get("Content-Type"),
			CacheControl: r.Header.Get("Cache-Control"),
		}
		s.mu.Unlock()
		w.Header().Set("ETag", `"fake"`)
		w.WriteHeader(http.StatusOK)
	case http.MethodGet, http.MethodHead:
		o, ok := s.Object(key)
		s.mu.Lock()
		gone := s.missing
		s.mu.Unlock()
		if !ok || gone {
			http.Error(w, "NoSuchKey", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", o.ContentType)
		w.Header().Set("Cache-Control", o.CacheControl)
		w.Header().Set("Content-Length", strconv.Itoa(len(o.Body)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(o.Body)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
