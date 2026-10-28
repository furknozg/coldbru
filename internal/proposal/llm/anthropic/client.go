// Package anthropic is the real adapter of llm.LLM on the official Go SDK, anthropic-sdk-go/v2
// (D-047, D-069). It implements the three calls of the agent flow:
//
//   - Screen: Haiku 5.5, thinking disabled, native structured output, fail-closed (D-069).
//   - ProposeSegment: Sonnet 5.5, adaptive thinking, a non-streaming strict tool loop with
//     tool_choice auto in which invalid filters come back as is_error tool results (D-027, D-069).
//   - StreamStructured: Sonnet 5.5, thinking between_tools, streamed structured output.
//
// The system text, tool definitions and output schema come from the embedded prompt packages
// (D-059) and are sent unchanged, with a cache breakpoint on the system block, so the prefix is
// byte-identical between calls (D-060, D-061). No call sets temperature or prefills the answer.
package anthropic

import (
	"errors"
	"fmt"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// Call parameters of D-069 that are not part of a prompt package's prefix hash.
const (
	// DefaultMaxRetries is the SDK's transport retry count (408, 409, 429, 5xx, network), separate
	// from the repair loop.
	DefaultMaxRetries = 2

	ScreenMaxTokens  = 256
	SegmentMaxTokens = 4096
	ContentMaxTokens = 2048
)

// Client implements llm.LLM. It is safe for concurrent use.
type Client struct {
	api     sdk.Client
	screen  *prompts.Package
	segment *prompts.Package
}

var _ llm.LLM = (*Client)(nil)

// New returns a Client for cfg. The SDK reads nothing from the environment (the platform config
// has already done that); extra options come last and may override anything, for example
// option.WithMaxRetries(0) or option.WithHTTPClient in tests.
func New(cfg Config, extra ...option.RequestOption) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("anthropic: missing required config: " + KeyAPIKey)
	}
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(cfg.APIKey),
		option.WithMaxRetries(DefaultMaxRetries),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	opts = append(opts, extra...)
	return &Client{api: sdk.NewClient(opts...), screen: prompts.Screen(), segment: prompts.Segment()}, nil
}

// errorf is fmt.Errorf with the package prefix.
func errorf(format string, args ...any) error { return fmt.Errorf("anthropic: "+format, args...) }
