// Package anthropicfake is an httptest fake of the Anthropic Messages API.
package anthropicfake

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// Response is one scripted answer. Zero value is an empty end_turn message.
type Response struct {
	Text       string         // text content block, if set
	ToolName   string         // tool_use content block, if set
	ToolInput  map[string]any // input of the tool_use block
	JSON       any            // marshalled into a text block (structured output)
	StopReason string         // defaults to end_turn, or tool_use with a tool block
	Delay      time.Duration  // sleep before answering
	Status     int            // HTTP status; defaults to 200
}

// Request is one captured /v1/messages request.
type Request struct {
	Header http.Header
	Body   map[string]any
}

// Fake is the fake server.
type Fake struct {
	*httptest.Server
	mu       sync.Mutex
	script   []Response
	requests []Request
}

// New starts a fake; call Close when done.
func New() *Fake {
	f := &Fake{}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

// Script queues responses; each request consumes one. With none queued the
// fake answers with an empty message.
func (f *Fake) Script(rs ...Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script = append(f.script, rs...)
}

// Requests returns the captured requests in arrival order.
func (f *Fake) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.requests...)
}

func (f *Fake) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/messages" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)

	f.mu.Lock()
	f.requests = append(f.requests, Request{Header: r.Header.Clone(), Body: body})
	var resp Response
	if len(f.script) > 0 {
		resp, f.script = f.script[0], f.script[1:]
	}
	f.mu.Unlock()

	if resp.Delay > 0 {
		select {
		case <-time.After(resp.Delay):
		case <-r.Context().Done():
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if resp.Status != 0 && resp.Status != http.StatusOK {
		w.WriteHeader(resp.Status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":  "error",
			"error": map[string]any{"type": "api_error", "message": "scripted failure"},
		})
		return
	}
	content := []map[string]any{}
	stop := resp.StopReason
	if resp.Text != "" {
		content = append(content, map[string]any{"type": "text", "text": resp.Text})
	}
	if resp.JSON != nil {
		raw, _ := json.Marshal(resp.JSON)
		content = append(content, map[string]any{"type": "text", "text": string(raw)})
	}
	if resp.ToolName != "" {
		content = append(content, map[string]any{
			"type": "tool_use", "id": "toolu_fake", "name": resp.ToolName, "input": resp.ToolInput,
		})
		if stop == "" {
			stop = "tool_use"
		}
	}
	if stop == "" {
		stop = "end_turn"
	}
	out := map[string]any{
		"id": "msg_fake", "type": "message", "role": "assistant", "model": "claude-opus-5-5",
		"content": content, "stop_reason": stop, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": 10, "output_tokens": 5},
	}
	if stop == "refusal" {
		out["stop_details"] = map[string]any{"type": "refusal", "category": nil, "explanation": nil}
	}
	_ = json.NewEncoder(w).Encode(out)
}
