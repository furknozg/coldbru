package flow

import (
	"errors"
	"strings"
)

// Errors of the actions. Callers (T-025's REST layer) map them with errors.Is.
var (
	// ErrNotFound: the campaign, step or variant does not exist.
	ErrNotFound = errors.New("flow: not found")
	// ErrNotOpen: the step is not open (approved, superseded, or not yet opened).
	ErrNotOpen = errors.New("flow: step is not open")
	// ErrNotDraft: the variant is not a draft (approved, rejected or superseded).
	ErrNotDraft = errors.New("flow: variant is not a draft")
	// ErrInvalidContent: edited content failed the validators of its step; see *InvalidContentError.
	ErrInvalidContent = errors.New("flow: content failed validation")
	// ErrNotApproved: Reopen was asked for a step that is not approved.
	ErrNotApproved = errors.New("flow: step is not approved")
	// ErrNotDrafting: the campaign is no longer a draft (cancelled, or confirmed).
	ErrNotDrafting = errors.New("flow: campaign is not a draft")
	// ErrHasSteps: RegenerateCampaign was asked for a campaign that already has steps; use
	// Regenerate on its open step.
	ErrHasSteps = errors.New("flow: campaign already has steps")
	// ErrInvalidInput: a required argument is empty (a brief, a rejection reason, a note).
	ErrInvalidInput = errors.New("flow: invalid input")
	// ErrConfirm: the schedule step was approved but the Confirmer failed; the approval stands.
	ErrConfirm = errors.New("flow: confirming the campaign failed")
	// ErrNoContentGenerator: the content step needs a ContentGenerator and the service has none.
	ErrNoContentGenerator = errors.New("flow: no content generator configured")
)

// InvalidContentError is the error of an edit that did not pass the validators. Problems are
// the validators' messages, one per problem.
type InvalidContentError struct{ Problems []string }

func (e *InvalidContentError) Error() string {
	return ErrInvalidContent.Error() + ": " + strings.Join(e.Problems, "; ")
}

// Is makes errors.Is(err, ErrInvalidContent) true.
func (e *InvalidContentError) Is(target error) bool { return target == ErrInvalidContent }
