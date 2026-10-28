package llmtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// ErrScriptExhausted is returned when a call arrives and its queue is empty.
var ErrScriptExhausted = errors.New("llmtest: no scripted response left")

// Scripted is a fake llm.LLM driven by one queue of responses per method (research 03 §8). Queue
// responses, run the code under test, then read the recorded calls. It is safe for concurrent use.
type Scripted struct {
	mu         sync.Mutex
	screens    []screenStep
	segments   []segmentStep
	structured []structuredStep

	screenBriefs  []string
	segmentInputs []llm.SegmentInput
	streamCalls   []StreamCall
}

var _ llm.LLM = (*Scripted)(nil)

type screenStep struct {
	res     llm.ScreenResult
	err     error
	records []llm.CallRecord
}

type segmentStep struct {
	res     llm.SegmentResult
	err     error
	dsls    []string
	records []llm.CallRecord
}

type structuredStep struct {
	raw     json.RawMessage
	deltas  []string
	usage   llm.Usage
	err     error
	records []llm.CallRecord
}

// StreamCall is one recorded StreamStructured call.
type StreamCall struct {
	Package  *prompts.Package
	UserJSON []byte
}

// NewScripted returns an empty fake.
func NewScripted() *Scripted { return &Scripted{} }

// AllowScreen queues an allow verdict.
func (s *Scripted) AllowScreen() *Scripted {
	return s.pushScreen(screenStep{res: llm.ScreenResult{Verdict: llm.VerdictAllow, Reason: "ordinary marketing request"}})
}

// BlockScreen queues a block verdict with reason; Screen returns it with *llm.ScreenBlockedError.
func (s *Scripted) BlockScreen(reason string) *Scripted {
	return s.pushScreen(screenStep{
		res: llm.ScreenResult{Verdict: llm.VerdictBlock, Reason: reason},
		err: &llm.ScreenBlockedError{Reason: reason},
	})
}

// FailScreen queues an error (an API failure, a refusal); the verdict is zero, as in the adapter.
func (s *Scripted) FailScreen(err error) *Scripted { return s.pushScreen(screenStep{err: err}) }

func (s *Scripted) pushScreen(st screenStep) *Scripted {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.screens = append(s.screens, st)
	return s
}

// ReplySegment queues a finished loop: Segment returns res.
func (s *Scripted) ReplySegment(res llm.SegmentResult) *Scripted {
	return s.pushSegment(segmentStep{res: res})
}

// FailSegment queues an error, for example &llm.RepairExhaustedError{}.
func (s *Scripted) FailSegment(err error) *Scripted { return s.pushSegment(segmentStep{err: err}) }

// ProposeDSLs queues a loop that plays the model: it proposes the DSL texts in order through the
// SegmentTools it is given, as propose_segment calls, and returns the first one Validate accepts.
// After llm.MaxProposeAttempts rejections it returns *llm.RepairExhaustedError with the last
// errors, as the real loop does. "Invalid then valid" is ProposeDSLs(bad, good).
func (s *Scripted) ProposeDSLs(dsls ...string) *Scripted {
	return s.pushSegment(segmentStep{dsls: dsls})
}

func (s *Scripted) pushSegment(st segmentStep) *Scripted {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.segments = append(s.segments, st)
	return s
}

// StreamJSON queues a streamed document: the fragments reach onDelta in order and their
// concatenation is the result; the validate callback of the call runs on it, as in the adapter. Pass the whole document as one fragment to skip splitting.
func (s *Scripted) StreamJSON(fragments ...string) *Scripted {
	raw := ""
	for _, f := range fragments {
		raw += f
	}
	return s.pushStructured(structuredStep{raw: json.RawMessage(raw), deltas: fragments})
}

// FailStream queues an error after the fragments were delivered to onDelta (a mid-stream failure);
// pass none for a call that fails at once.
func (s *Scripted) FailStream(err error, fragments ...string) *Scripted {
	return s.pushStructured(structuredStep{deltas: fragments, err: err})
}

func (s *Scripted) pushStructured(st structuredStep) *Scripted {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.structured = append(s.structured, st)
	return s
}

// WithRecords attaches records to the step queued last of the given purpose; the fake reports
// them to the recorder in ctx when the step runs, so tests of the persistence path see rows.
func (s *Scripted) WithRecords(p llm.Purpose, recs ...llm.CallRecord) *Scripted {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch p {
	case llm.PurposeScreen:
		s.screens[len(s.screens)-1].records = recs
	case llm.PurposeSegment:
		s.segments[len(s.segments)-1].records = recs
	case llm.PurposeContent:
		s.structured[len(s.structured)-1].records = recs
	}
	return s
}

func report(ctx context.Context, recs []llm.CallRecord) {
	for _, r := range recs {
		llm.Report(ctx, r)
	}
}

// Screen pops the next screen step.
func (s *Scripted) Screen(ctx context.Context, brief string) (llm.ScreenResult, error) {
	s.mu.Lock()
	s.screenBriefs = append(s.screenBriefs, brief)
	if len(s.screens) == 0 {
		s.mu.Unlock()
		return llm.ScreenResult{}, fmt.Errorf("screen: %w", ErrScriptExhausted)
	}
	st := s.screens[0]
	s.screens = s.screens[1:]
	s.mu.Unlock()
	report(ctx, st.records)
	return st.res, st.err
}

// ProposeSegment pops the next segment step.
func (s *Scripted) ProposeSegment(ctx context.Context, in llm.SegmentInput, tools llm.SegmentTools) (llm.SegmentResult, error) {
	s.mu.Lock()
	s.segmentInputs = append(s.segmentInputs, in)
	if len(s.segments) == 0 {
		s.mu.Unlock()
		return llm.SegmentResult{}, fmt.Errorf("segment: %w", ErrScriptExhausted)
	}
	st := s.segments[0]
	s.segments = s.segments[1:]
	s.mu.Unlock()
	report(ctx, st.records)
	if st.err != nil || st.dsls == nil {
		return st.res, st.err
	}
	var lastErrs []*dsl.Error
	attempts := 0
	for _, src := range st.dsls {
		if attempts == llm.MaxProposeAttempts {
			break
		}
		attempts++
		f, p, errs, err := tools.Validate(ctx, src)
		if err != nil {
			return llm.SegmentResult{}, &llm.ToolsError{Err: err}
		}
		if len(errs) == 0 {
			return llm.SegmentResult{DSL: src, Filter: f, Preview: p, Turn: attempts, Turns: attempts, ProposeAttempts: attempts}, nil
		}
		lastErrs = errs
	}
	return llm.SegmentResult{}, &llm.RepairExhaustedError{Attempts: attempts, Errors: lastErrs}
}

// StreamStructured pops the next structured step, delivering its fragments to onDelta.
func (s *Scripted) StreamStructured(ctx context.Context, pkg *prompts.Package, userJSON []byte, onDelta func(string), validate func(json.RawMessage) error) (json.RawMessage, llm.Usage, error) {
	s.mu.Lock()
	s.streamCalls = append(s.streamCalls, StreamCall{Package: pkg, UserJSON: append([]byte(nil), userJSON...)})
	if len(s.structured) == 0 {
		s.mu.Unlock()
		return nil, llm.Usage{}, fmt.Errorf("stream: %w", ErrScriptExhausted)
	}
	st := s.structured[0]
	s.structured = s.structured[1:]
	s.mu.Unlock()
	report(ctx, st.records)
	for _, d := range st.deltas {
		if onDelta != nil {
			onDelta(d)
		}
	}
	if st.err != nil {
		return nil, llm.Usage{}, st.err
	}
	if validate != nil {
		if verr := validate(st.raw); verr != nil {
			return st.raw, st.usage, &llm.ContentInvalidError{Err: verr}
		}
	}
	return st.raw, st.usage, nil
}

// ScreenBriefs returns the briefs Screen received.
func (s *Scripted) ScreenBriefs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.screenBriefs...)
}

// SegmentInputs returns the inputs ProposeSegment received.
func (s *Scripted) SegmentInputs() []llm.SegmentInput {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]llm.SegmentInput(nil), s.segmentInputs...)
}

// StreamCalls returns the StreamStructured calls received.
func (s *Scripted) StreamCalls() []StreamCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]StreamCall(nil), s.streamCalls...)
}

// Tools is a configurable llm.SegmentTools. The zero value accepts every filter with an audience of
// 10 reachable customers.
type Tools struct {
	// ValidateFunc and CountFunc override the defaults.
	ValidateFunc func(ctx context.Context, src string) (*compile.Filter, *customer.Preview, []*dsl.Error, error)
	CountFunc    func(ctx context.Context, src string) (*customer.Preview, error)

	mu        sync.Mutex
	validated []string
	counted   []string
}

var _ llm.SegmentTools = (*Tools)(nil)

// Preview10 is the preview the zero Tools answers with.
func Preview10() *customer.Preview {
	return &customer.Preview{Count: 10, Coverage: customer.Coverage{Both: 4, EmailOnly: 3, SmsOnly: 3}}
}

// Validate records src and answers.
func (t *Tools) Validate(ctx context.Context, src string) (*compile.Filter, *customer.Preview, []*dsl.Error, error) {
	t.mu.Lock()
	t.validated = append(t.validated, src)
	t.mu.Unlock()
	if t.ValidateFunc != nil {
		return t.ValidateFunc(ctx, src)
	}
	return nil, Preview10(), nil, nil
}

// CountAudience records src and answers.
func (t *Tools) CountAudience(ctx context.Context, src string) (*customer.Preview, error) {
	t.mu.Lock()
	t.counted = append(t.counted, src)
	t.mu.Unlock()
	if t.CountFunc != nil {
		return t.CountFunc(ctx, src)
	}
	return Preview10(), nil
}

// Validated returns the filters passed to Validate.
func (t *Tools) Validated() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.validated...)
}

// Counted returns the filters passed to CountAudience.
func (t *Tools) Counted() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.counted...)
}
