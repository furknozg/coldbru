package anthropic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/anthropic"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
)

// newClient connects the real adapter to the scripted server. The SDK reads no environment
// (WithoutEnvironmentDefaults), so these tests never see ANTHROPIC_API_KEY. Transport retries are
// off unless the test asks for them.
func newClient(t *testing.T, srv *llmtest.MessagesServer, extra ...option.RequestOption) *anthropic.Client {
	t.Helper()
	opts := append([]option.RequestOption{option.WithMaxRetries(0)}, extra...)
	c, err := anthropic.New(anthropic.Config{APIKey: "test", BaseURL: srv.URL}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// withRecorder returns a context that collects the records of model calls.
func withRecorder(t *testing.T) (context.Context, *llm.Collector) {
	t.Helper()
	col := &llm.Collector{}
	return llm.WithRecorder(t.Context(), col), col
}

func msg(blocks ...llmtest.Block) llmtest.Message { return llmtest.Message{Blocks: blocks} }

func proposeBlock(id, dsl string) llmtest.Block {
	return llmtest.ToolUse(id, "propose_segment", map[string]any{"dsl": dsl})
}

func countBlock(id, dsl string) llmtest.Block {
	return llmtest.ToolUse(id, "count_audience", map[string]any{"dsl": dsl})
}

// previewerFunc is a customer.Client stand-in for llm.NewCustomerTools.
type previewerFunc func(ctx context.Context, f *compile.Filter) (*customer.Preview, error)

func (p previewerFunc) Preview(ctx context.Context, f *compile.Filter) (*customer.Preview, error) {
	return p(ctx, f)
}

func previewOf(n int) previewerFunc {
	return func(context.Context, *compile.Filter) (*customer.Preview, error) {
		return &customer.Preview{Count: n, Coverage: customer.Coverage{Both: n}}, nil
	}
}

// field digs into decoded JSON: field(v, "messages", 0, "role").
func field(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("%v: not an object at %q: %T", path, k, v)
			}
			v = m[k]
		case int:
			a, ok := v.([]any)
			if !ok || k >= len(a) {
				t.Fatalf("%v: no element %d (%T)", path, k, v)
			}
			v = a[k]
		}
	}
	return v
}

func messagesOf(t *testing.T, r llmtest.Request) []any {
	t.Helper()
	m, _ := r.JSON()["messages"].([]any)
	return m
}

// toolResultText returns the text and is_error flag of the first tool_result block of the last
// message of a request.
func lastToolResult(t *testing.T, r llmtest.Request) (id, text string, isErr bool) {
	t.Helper()
	msgs := messagesOf(t, r)
	last := msgs[len(msgs)-1]
	if field(t, last, "role") != "user" {
		t.Fatalf("last message is %v, want user", field(t, last, "role"))
	}
	blk := field(t, last, "content", 0)
	if field(t, blk, "type") != "tool_result" {
		t.Fatalf("first block of the last message is %v", blk)
	}
	id, _ = field(t, blk, "tool_use_id").(string)
	text, _ = field(t, blk, "content", 0, "text").(string)
	isErr, _ = field(t, blk, "is_error").(bool)
	return id, text, isErr
}

func decode(t *testing.T, text string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text), into); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, text)
	}
}

func newClientDefaultRetries(t *testing.T, srv *llmtest.MessagesServer) (*anthropic.Client, error) {
	t.Helper()
	return anthropic.New(anthropic.Config{APIKey: "test", BaseURL: srv.URL})
}

func marshalJSON(v any) ([]byte, error) { return json.Marshal(v) }
