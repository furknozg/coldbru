package llm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// Sentinels for errors.Is; the typed errors below carry the details.
var (
	// ErrScreenBlocked: the pre-screen refused the brief (D-069).
	ErrScreenBlocked = errors.New("llm: brief blocked by the pre-screen")
	// ErrRefusal: the model stopped with stop_reason refusal. No repair, no fallback (D-069).
	ErrRefusal = errors.New("llm: model refused the request")
	// ErrRepairExhausted: three propose attempts failed (D-069).
	ErrRepairExhausted = errors.New("llm: no valid segment after the allowed attempts")
	// ErrTurnLimit: the segment loop used all its model turns (D-069).
	ErrTurnLimit = errors.New("llm: segment loop ran out of model turns")
	// ErrAPI: the API call failed (HTTP error, network, cancelled context).
	ErrAPI = errors.New("llm: API call failed")
	// ErrInvalidOutput: the structured output was cut off or is not JSON.
	ErrInvalidOutput = errors.New("llm: structured output unusable")
	// ErrContentInvalid: the validate callback of StreamStructured rejected the document.
	ErrContentInvalid = errors.New("llm: structured content failed validation")
	// ErrTools: the SegmentTools failed for a reason the model cannot repair.
	ErrTools = errors.New("llm: segment tools failed")
)

// ScreenBlockedError carries the pre-screen's reason, to show to the marketer (D-069).
type ScreenBlockedError struct{ Reason string }

func (e *ScreenBlockedError) Error() string {
	return "llm: brief blocked by the pre-screen: " + e.Reason
}

// Is makes errors.Is(err, ErrScreenBlocked) true.
func (e *ScreenBlockedError) Is(target error) bool { return target == ErrScreenBlocked }

// RefusalError is a refusal stop reason. Category is stop_details.category (cyber, bio,
// frontier_llm, reasoning_extraction, general_harms) and may be empty (D-069).
type RefusalError struct {
	Purpose     Purpose
	Category    string
	Explanation string
}

func (e *RefusalError) Error() string {
	s := fmt.Sprintf("llm: %s call refused by the model", e.Purpose)
	if e.Category != "" {
		s += " (category " + e.Category + ")"
	}
	return s
}

// Is makes errors.Is(err, ErrRefusal) true.
func (e *RefusalError) Is(target error) bool { return target == ErrRefusal }

// RepairExhaustedError ends the segment loop after MaxProposeAttempts failed attempts. Errors is
// what the last failed propose_segment reported; it is empty when the last attempt failed without
// a DSL (the model did not call the tool, or was cut off), and Reason then says so.
type RepairExhaustedError struct {
	Attempts int
	Errors   []*dsl.Error
	Reason   string
}

func (e *RepairExhaustedError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "llm: no valid segment after %d attempts", e.Attempts)
	if e.Reason != "" {
		b.WriteString(": " + e.Reason)
	}
	for _, de := range e.Errors {
		b.WriteString("; " + de.Error())
	}
	return b.String()
}

// Is makes errors.Is(err, ErrRepairExhausted) true.
func (e *RepairExhaustedError) Is(target error) bool { return target == ErrRepairExhausted }

// TurnLimitError ends the segment loop after MaxSegmentTurns model turns without an accepted filter.
type TurnLimitError struct{ Turns int }

func (e *TurnLimitError) Error() string {
	return fmt.Sprintf("llm: segment loop used all %d model turns without an accepted filter", e.Turns)
}

// Is makes errors.Is(err, ErrTurnLimit) true.
func (e *TurnLimitError) Is(target error) bool { return target == ErrTurnLimit }

// APIError is a failed API call. Status is the HTTP status, 0 for a network error or a cancelled
// context; Type is the API's error type ("rate_limit_error", "overloaded_error", ...).
type APIError struct {
	Purpose   Purpose
	Status    int
	Type      string
	RequestID string
	Err       error
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("llm: %s call failed", e.Purpose)
	if e.Status != 0 {
		s += fmt.Sprintf(" (HTTP %d", e.Status)
		if e.Type != "" {
			s += " " + e.Type
		}
		s += ")"
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

// Unwrap returns the underlying error, so errors.Is(err, context.Canceled) works.
func (e *APIError) Unwrap() error { return e.Err }

// Is makes errors.Is(err, ErrAPI) true.
func (e *APIError) Is(target error) bool { return target == ErrAPI }

// Retryable reports whether trying the same call again later may succeed: rate limiting,
// overload, a server error or a network failure.
func (e *APIError) Retryable() bool {
	return e.Status == 0 || e.Status == 408 || e.Status == 409 || e.Status == 429 || e.Status >= 500
}

// InvalidOutputError is a structured output that cannot be used: cut off by max_tokens, or not
// valid JSON. StopReason is the API's stop reason.
type InvalidOutputError struct {
	Purpose    Purpose
	StopReason string
	Reason     string
}

func (e *InvalidOutputError) Error() string {
	return fmt.Sprintf("llm: %s output unusable (stop reason %q): %s", e.Purpose, e.StopReason, e.Reason)
}

// Is makes errors.Is(err, ErrInvalidOutput) true.
func (e *InvalidOutputError) Is(target error) bool { return target == ErrInvalidOutput }

// ToolsError wraps a failure of SegmentTools that is not a DSL problem the model could repair
// (the Customer service is down, the token is rejected).
type ToolsError struct{ Err error }

func (e *ToolsError) Error() string { return "llm: segment tools failed: " + e.Err.Error() }

// Unwrap returns the underlying error.
func (e *ToolsError) Unwrap() error { return e.Err }

// Is makes errors.Is(err, ErrTools) true.
func (e *ToolsError) Is(target error) bool { return target == ErrTools }

// ContentInvalidError wraps the error the validate callback of StreamStructured returned. The
// rejected document is returned next to it.
type ContentInvalidError struct{ Err error }

func (e *ContentInvalidError) Error() string {
	return "llm: structured content failed validation: " + e.Err.Error()
}

// Unwrap returns the validation error, so errors.Is and errors.As see through it.
func (e *ContentInvalidError) Unwrap() error { return e.Err }

// Is makes errors.Is(err, ErrContentInvalid) true.
func (e *ContentInvalidError) Is(target error) bool { return target == ErrContentInvalid }
