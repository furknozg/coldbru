// Package llmtest holds the test doubles of the LLM layer (D-025): a scripted fake of the
// llm.LLM port for the unit tests of the agent flow, and an httptest server that speaks the
// Messages API (plain JSON and SSE) so that the real adapter runs against it. Nothing here reads
// ANTHROPIC_API_KEY or contacts the network.
package llmtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// Block is one content block of a scripted assistant message.
type Block struct {
	// Type is "text", "thinking" or "tool_use".
	Type string
	// Text is the text of a text block.
	Text string
	// Chunks, for a text block in an SSE response, are the text_delta fragments; when empty the
	// text is cut into pieces of 8 runes.
	Chunks []string
	// Thinking and Signature are the fields of a thinking block.
	Thinking, Signature string
	// ID, Name and Input are the fields of a tool_use block.
	ID, Name string
	Input    any
}

// Text returns a text block. With chunks, the text is their concatenation and an SSE response
// sends one text_delta per chunk.
func Text(s string, chunks ...string) Block {
	if len(chunks) > 0 {
		s = strings.Join(chunks, "")
	}
	return Block{Type: "text", Text: s, Chunks: chunks}
}

// Thinking returns a thinking block. The default display of Sonnet 5.5 omits the text, so
// Thinking("", "sig") is the realistic shape.
func Thinking(text, signature string) Block {
	return Block{Type: "thinking", Thinking: text, Signature: signature}
}

// ToolUse returns a tool_use block; input is marshalled to JSON.
func ToolUse(id, name string, input any) Block {
	return Block{Type: "tool_use", ID: id, Name: name, Input: input}
}

// Refusal is the stop_details of a refusal stop reason.
type Refusal struct{ Category, Explanation string }

// Usage is the token usage of a scripted message.
type Usage struct{ Input, Output, CacheRead, CacheCreation int }

// Message is a scripted assistant message.
type Message struct {
	Model  string
	Blocks []Block
	// StopReason defaults to "tool_use" when a block is a tool_use and to "end_turn" otherwise.
	StopReason string
	// Refusal, when set, makes the stop reason "refusal" with these stop_details.
	Refusal *Refusal
	Usage   Usage
}

func (m Message) stopReason() string {
	switch {
	case m.Refusal != nil:
		return "refusal"
	case m.StopReason != "":
		return m.StopReason
	}
	for _, b := range m.Blocks {
		if b.Type == "tool_use" {
			return "tool_use"
		}
	}
	return "end_turn"
}

func (m Message) stopDetails() any {
	if m.Refusal == nil {
		return nil
	}
	var cat, expl any
	if m.Refusal.Category != "" {
		cat = m.Refusal.Category
	}
	if m.Refusal.Explanation != "" {
		expl = m.Refusal.Explanation
	}
	return map[string]any{"type": "refusal", "category": cat, "explanation": expl}
}

func (m Message) model() string {
	if m.Model != "" {
		return m.Model
	}
	return "claude-sonnet-5-5"
}

func (m Message) usage() map[string]any {
	return map[string]any{
		"input_tokens":                m.Usage.Input,
		"output_tokens":               m.Usage.Output,
		"cache_read_input_tokens":     m.Usage.CacheRead,
		"cache_creation_input_tokens": m.Usage.CacheCreation,
	}
}

// block renders a block as it appears in a complete message.
func (b Block) block() map[string]any {
	switch b.Type {
	case "thinking":
		return map[string]any{"type": "thinking", "thinking": b.Thinking, "signature": b.Signature}
	case "tool_use":
		return map[string]any{"type": "tool_use", "id": b.ID, "name": b.Name, "input": b.input()}
	default:
		return map[string]any{"type": "text", "text": b.Text}
	}
}

func (b Block) input() any {
	if b.Input == nil {
		return map[string]any{}
	}
	return b.Input
}

// Response is one scripted HTTP response of MessagesServer.
type Response struct {
	write func(w http.ResponseWriter)
}

// JSON answers a non-streaming request with a complete message.
func JSON(m Message) Response {
	return Response{write: func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_test", "type": "message", "role": "assistant", "model": m.model(),
			"content":       blocks(m),
			"stop_reason":   m.stopReason(),
			"stop_sequence": nil,
			"stop_details":  m.stopDetails(),
			"usage":         m.usage(),
		})
	}}
}

func blocks(m Message) []map[string]any {
	out := make([]map[string]any, len(m.Blocks))
	for i, b := range m.Blocks {
		out[i] = b.block()
	}
	return out
}

// SSE answers a streaming request with the event sequence of the Messages API: message_start,
// then per block content_block_start, ping once, content_block_delta, content_block_stop, then
// message_delta (stop reason and usage) and message_stop.
func SSE(m Message) Response {
	return Response{write: func(w http.ResponseWriter) { writeStream(w, m, streamFull) }}
}

// SSETruncated sends the stream without message_delta and message_stop, as if the connection broke.
func SSETruncated(m Message) Response {
	return Response{write: func(w http.ResponseWriter) { writeStream(w, m, streamTruncated) }}
}

// SSEError sends the message's blocks and then an `error` event in the middle of the stream, such
// as the overloaded_error the API can send after the response has started.
func SSEError(m Message, errType, message string) Response {
	return Response{write: func(w http.ResponseWriter) {
		writeStream(w, m, streamTruncated)
		writeEvent(w, "error", map[string]any{"type": "error", "error": map[string]any{"type": errType, "message": message}})
	}}
}

// HTTPError answers with an API error body and status. It sends Retry-After-Ms: 1 so that the SDK's
// transport retries do not slow a test down.
func HTTPError(status int, errType, message string) Response {
	return Response{write: func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After-Ms", "1")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "error", "error": map[string]any{"type": errType, "message": message}, "request_id": "req_test",
		})
	}}
}

type streamMode int

const (
	streamFull streamMode = iota
	streamTruncated
)

func writeEvent(w http.ResponseWriter, name string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeStream(w http.ResponseWriter, m Message, mode streamMode) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	u := m.usage()
	u["output_tokens"] = 1
	writeEvent(w, "message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg_test", "type": "message", "role": "assistant", "model": m.model(),
		"content": []any{}, "stop_reason": nil, "stop_sequence": nil, "stop_details": nil, "usage": u,
	}})
	for i, b := range m.Blocks {
		start, deltas := b.streamParts()
		writeEvent(w, "content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": start})
		if i == 0 {
			writeEvent(w, "ping", map[string]any{"type": "ping"})
		}
		for _, d := range deltas {
			writeEvent(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": d})
		}
		last := i == len(m.Blocks)-1
		if mode == streamTruncated && last {
			return // the last block is left open
		}
		writeEvent(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
	}
	if mode == streamTruncated {
		return
	}
	writeEvent(w, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason": m.stopReason(), "stop_sequence": nil, "stop_details": m.stopDetails(),
		},
		"usage": m.usage(),
	})
	writeEvent(w, "message_stop", map[string]any{"type": "message_stop"})
}

// streamParts returns the content_block_start block and the deltas of a block.
func (b Block) streamParts() (start map[string]any, deltas []map[string]any) {
	switch b.Type {
	case "thinking":
		return map[string]any{"type": "thinking", "thinking": ""}, []map[string]any{
			{"type": "thinking_delta", "thinking": b.Thinking},
			{"type": "signature_delta", "signature": b.Signature},
		}
	case "tool_use":
		raw, _ := json.Marshal(b.input())
		s := string(raw)
		half := len(s) / 2
		return map[string]any{"type": "tool_use", "id": b.ID, "name": b.Name, "input": map[string]any{}}, []map[string]any{
			{"type": "input_json_delta", "partial_json": s[:half]},
			{"type": "input_json_delta", "partial_json": s[half:]},
		}
	}
	chunks := b.Chunks
	if len(chunks) == 0 {
		chunks = splitRunes(b.Text, 8)
	}
	for _, c := range chunks {
		deltas = append(deltas, map[string]any{"type": "text_delta", "text": c})
	}
	return map[string]any{"type": "text", "text": ""}, deltas
}

func splitRunes(s string, n int) []string {
	var out []string
	for len(s) > 0 {
		cut := 0
		for i := 0; i < n && cut < len(s); i++ {
			_, size := utf8.DecodeRuneInString(s[cut:])
			cut += size
		}
		out = append(out, s[:cut])
		s = s[cut:]
	}
	return out
}

// Request is a request the server received.
type Request struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// JSON decodes the request body into a generic map.
func (r Request) JSON() map[string]any {
	var v map[string]any
	if err := json.Unmarshal(r.Body, &v); err != nil {
		return nil
	}
	return v
}

// Streaming reports whether the request asked for a stream.
func (r Request) Streaming() bool {
	b, _ := r.JSON()["stream"].(bool)
	return b
}

// MessagesServer is an httptest server for POST /v1/messages that answers with scripted
// responses in order and records every request body.
type MessagesServer struct {
	*httptest.Server
	t     testing.TB
	mu    sync.Mutex
	queue []Response
	reqs  []Request
}

// NewMessagesServer starts the server, closes it when the test ends and queues responses.
func NewMessagesServer(t testing.TB, responses ...Response) *MessagesServer {
	t.Helper()
	s := &MessagesServer{t: t, queue: responses}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Server.Close)
	return s
}

// Enqueue appends responses to the script.
func (s *MessagesServer) Enqueue(rs ...Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, rs...)
}

// Requests returns the requests received so far.
func (s *MessagesServer) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.reqs...)
}

func (s *MessagesServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.reqs = append(s.reqs, Request{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: body})
	var resp *Response
	if len(s.queue) > 0 {
		resp, s.queue = &s.queue[0], s.queue[1:]
	}
	s.mu.Unlock()
	if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
		http.NotFound(w, r)
		return
	}
	if resp == nil {
		s.t.Errorf("llmtest: request %d arrived with no scripted response left", len(s.Requests()))
		HTTPError(http.StatusBadRequest, "invalid_request_error", "llmtest: no scripted response left").write(w)
		return
	}
	resp.write(w)
}
