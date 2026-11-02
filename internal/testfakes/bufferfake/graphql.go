package bufferfake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// RequestIDHeader is the header the fake stamps on every GraphQL response.
const RequestIDHeader = "X-Request-Id"

// Response is one scripted GraphQL answer. Status defaults to 200.
type Response struct {
	Status int
	Header map[string]string
	Data   any
	Errors []map[string]any
}

// Request is one GraphQL request the fake received.
type Request struct {
	Op        string
	Query     string
	Vars      map[string]any
	Auth      string // full Authorization header
	RequestID string
}

var opRe = regexp.MustCompile(`^\s*(?:query|mutation)\s+([A-Za-z_][A-Za-z0-9_]*)`)

// OpName returns the GraphQL operation name of query, or "".
func OpName(query string) string {
	if m := opRe.FindStringSubmatch(query); m != nil {
		return m[1]
	}
	return ""
}

// Handle sets the "data" payload returned for operation op whenever no
// scripted response is queued. Later calls replace earlier ones.
func (f *Fake) Handle(op string, data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.defaults[op] = Response{Data: data}
}

// HandleFunc answers op with fn whenever no scripted response is queued;
// it wins over Handle.
func (f *Fake) HandleFunc(op string, fn func(Request) Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.funcs[op] = fn
}

// Script queues responses for op; each request consumes one, then Handle's applies.
func (f *Fake) Script(op string, rs ...Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[op] = append(f.scripts[op], rs...)
}

// RejectToken makes requests bearing this access token get a 401.
func (f *Fake) RejectToken(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejected[token] = true
}

// OnRequest runs fn for each GraphQL request before it is answered
// (to inspect state or inject a delay).
func (f *Fake) OnRequest(fn func(Request)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onRequest = fn
}

// Requests returns every GraphQL request received.
func (f *Fake) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.requests...)
}

// Count returns how many requests named op were received.
func (f *Fake) Count(op string) int {
	n := 0
	for _, r := range f.Requests() {
		if r.Op == op {
			n++
		}
	}
	return n
}

// GraphQLQueries returns the query text of every GraphQL request.
func (f *Fake) GraphQLQueries() []string {
	var out []string
	for _, r := range f.Requests() {
		out = append(out, r.Query)
	}
	return out
}

func (f *Fake) graphql(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return
	}
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.reqSeq++
	req := Request{
		Op: OpName(body.Query), Query: body.Query, Vars: body.Variables,
		Auth: auth, RequestID: "req-" + strconv.Itoa(f.reqSeq),
	}
	f.requests = append(f.requests, req)
	hook := f.onRequest
	f.mu.Unlock()
	if hook != nil {
		hook(req)
	}

	f.mu.Lock()
	var resp Response
	found := true
	switch {
	case f.rejected[strings.TrimPrefix(auth, "Bearer ")]:
		resp = Response{Status: http.StatusUnauthorized, Errors: []map[string]any{{
			"message": "Not authorized", "extensions": map[string]any{"code": "UNAUTHORIZED"},
		}}}
	case len(f.scripts[req.Op]) > 0:
		resp = f.scripts[req.Op][0]
		f.scripts[req.Op] = f.scripts[req.Op][1:]
	case f.funcs[req.Op] != nil:
		fn := f.funcs[req.Op]
		f.mu.Unlock()
		resp = fn(req)
		f.mu.Lock()
	default:
		resp, found = f.defaults[req.Op]
	}
	f.mu.Unlock()
	if !found {
		resp = Response{Errors: []map[string]any{{"message": fmt.Sprintf("no fake handler for %q", req.Op)}}}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(RequestIDHeader, req.RequestID)
	for k, v := range resp.Header {
		w.Header().Set(k, v)
	}
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	out := map[string]any{}
	if resp.Data != nil {
		out["data"] = resp.Data
	}
	if resp.Errors != nil {
		out["errors"] = resp.Errors
	}
	_ = json.NewEncoder(w).Encode(out)
}
