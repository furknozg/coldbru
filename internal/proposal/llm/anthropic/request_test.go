package anthropic_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// requestShape checks what every call must and must not send (D-026, D-061, D-069).
func requestShape(t *testing.T, r llmtest.Request, pkg *prompts.Package, wantTools bool) map[string]any {
	t.Helper()
	body := r.JSON()
	if r.Method != "POST" || r.Path != "/v1/messages" {
		t.Errorf("request = %s %s", r.Method, r.Path)
	}
	// anthropic-sdk-go v2 sends the key as a bearer token.
	if got := r.Header.Get("Authorization"); got != "Bearer test" {
		t.Errorf("authorization = %q: the key comes from Config", got)
	}
	// 5.x models answer 400 to sampling parameters and to a prefilled assistant turn.
	for _, k := range []string{"temperature", "top_p", "top_k", "metadata", "stop_sequences"} {
		if _, ok := body[k]; ok {
			t.Errorf("request sets %s", k)
		}
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) == 0 || field(t, msgs[len(msgs)-1], "role") != "user" {
		t.Errorf("the last message must be a user turn (no prefill)")
	}
	if body["model"] != pkg.Model {
		t.Errorf("model = %v, want %s", body["model"], pkg.Model)
	}
	if field(t, body, "output_config", "effort") != pkg.Effort {
		t.Errorf("effort = %v, want %s", field(t, body, "output_config", "effort"), pkg.Effort)
	}

	// D-061: system and tools are exactly the embedded package, with a cache breakpoint at the end
	// of the system block (which caches the tools before it).
	system, _ := body["system"].([]any)
	if len(system) != 1 || field(t, system[0], "text") != pkg.System {
		t.Errorf("system = %v, want the package text", system)
	}
	if !reflect.DeepEqual(field(t, system[0], "cache_control"), map[string]any{"type": "ephemeral"}) {
		t.Errorf("cache_control = %v", field(t, system[0], "cache_control"))
	}
	var raw struct {
		Tools        []json.RawMessage `json:"tools"`
		OutputConfig struct {
			Format struct {
				Type   string          `json:"type"`
				Schema json.RawMessage `json:"schema"`
			} `json:"format"`
		} `json:"output_config"`
	}
	if err := json.Unmarshal(r.Body, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Tools) != len(pkg.Tools) {
		t.Fatalf("tools = %d, want %d", len(raw.Tools), len(pkg.Tools))
	}
	for i, tool := range raw.Tools {
		canon, err := prompts.Canonical(tool)
		if err != nil || string(canon) != string(pkg.Tools[i].JSON) {
			t.Errorf("tool %d differs from the package:\n got %s\nwant %s", i, canon, pkg.Tools[i].JSON)
		}
	}
	switch {
	case pkg.OutputSchema == nil && raw.OutputConfig.Format.Type != "":
		t.Errorf("unexpected output format %s", raw.OutputConfig.Format.Type)
	case pkg.OutputSchema != nil:
		canon, err := prompts.Canonical(raw.OutputConfig.Format.Schema)
		if raw.OutputConfig.Format.Type != "json_schema" || err != nil || string(canon) != string(pkg.OutputSchema) {
			t.Errorf("output format = %s %s, want json_schema %s", raw.OutputConfig.Format.Type, canon, pkg.OutputSchema)
		}
	}
	if wantTools {
		if !reflect.DeepEqual(body["tool_choice"], map[string]any{"type": "auto"}) {
			t.Errorf("tool_choice = %v, want auto (Sonnet 5.5 rejects forced tool use)", body["tool_choice"])
		}
	} else if _, ok := body["tool_choice"]; ok {
		t.Error("tool_choice set on a call without tools")
	}
	return body
}

func TestRequestBodyScreen(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, screenReply("allow", "ok"))
	if _, err := newClient(t, srv).Screen(t.Context(), "b"); err != nil {
		t.Fatal(err)
	}
	body := requestShape(t, srv.Requests()[0], prompts.Screen(), false)
	if body["max_tokens"] != float64(256) {
		t.Errorf("max_tokens = %v", body["max_tokens"])
	}
	if !reflect.DeepEqual(body["thinking"], map[string]any{"type": "disabled"}) {
		t.Errorf("thinking = %v, want disabled (Haiku 5.5 accepts it at effort low)", body["thinking"])
	}
	if _, ok := body["stream"]; ok {
		t.Error("screen must not stream")
	}
}

func TestRequestBodySegment(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(msg(proposeBlock("t", `city === "Ankara"`))))
	if _, err := newClient(t, srv).ProposeSegment(t.Context(), brief, &llmtest.Tools{}); err != nil {
		t.Fatal(err)
	}
	body := requestShape(t, srv.Requests()[0], prompts.Segment(), true)
	if body["max_tokens"] != float64(4096) {
		t.Errorf("max_tokens = %v", body["max_tokens"])
	}
	if !reflect.DeepEqual(body["thinking"], map[string]any{"type": "adaptive"}) {
		t.Errorf("thinking = %v, want adaptive", body["thinking"])
	}
	if _, ok := body["stream"]; ok {
		t.Error("the tool loop is not streamed")
	}
	// No call-specific data in the cached prefix (D-060): the brief appears only in the user message.
	const phrase = "hatırlatması gönder"
	if s := string(srv.Requests()[0].Body); countOccurrences(s, phrase) != 1 {
		t.Errorf("the brief appears %d times in the request, want once (user message only)", countOccurrences(s, phrase))
	}
}

func TestRequestBodyStream(t *testing.T) {
	t.Parallel()
	pkg := contentPackage(t)
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(msg(llmtest.Text(contentDoc))))
	if _, _, err := newClient(t, srv).StreamStructured(t.Context(), pkg, []byte(`{}`), nil, nil); err != nil {
		t.Fatal(err)
	}
	r := srv.Requests()[0]
	body := requestShape(t, r, pkg, false)
	if body["max_tokens"] != float64(2048) {
		t.Errorf("max_tokens = %v", body["max_tokens"])
	}
	// Sonnet 5.5 rejects "disabled"; between_tools is its lowest thinking setting (D-069).
	if !reflect.DeepEqual(body["thinking"], map[string]any{"type": "between_tools"}) {
		t.Errorf("thinking = %v, want between_tools", body["thinking"])
	}
	if !r.Streaming() {
		t.Error("stream flag missing")
	}
}

func TestRepeatedCallsSendIdenticalPrefixBytes(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(proposeBlock("t", `city === "Ankara"`))),
		llmtest.JSON(msg(proposeBlock("t", `city === "Ankara"`))),
	)
	c := newClient(t, srv)
	for _, in := range []llm.SegmentInput{{Brief: "birinci"}, {Brief: "ikinci", ContextNotes: []string{"not"}}} {
		if _, err := c.ProposeSegment(t.Context(), in, &llmtest.Tools{}); err != nil {
			t.Fatal(err)
		}
	}
	prefix := func(r llmtest.Request) string {
		var b struct {
			Tools  json.RawMessage `json:"tools"`
			System json.RawMessage `json:"system"`
		}
		if err := json.Unmarshal(r.Body, &b); err != nil {
			t.Fatal(err)
		}
		return string(b.Tools) + string(b.System)
	}
	reqs := srv.Requests()
	if prefix(reqs[0]) != prefix(reqs[1]) {
		t.Error("tools and system differ between two briefs: the cache would miss")
	}
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
