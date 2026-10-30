package flow

import (
	"testing"

	"github.com/google/uuid"
)

// pendingQuestions picks, per step, the latest question without an answer (D-088).
func TestPendingQuestions(t *testing.T) {
	t.Parallel()
	stepA, stepB := uuid.New(), uuid.New()
	q1, q2, q3, q4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	msgs := []ChatMessageView{
		{ID: q1, StepID: &stepA, Kind: MessageKindQuestion, Body: "first"},
		{ID: uuid.New(), StepID: &stepA, Kind: MessageKindAnswer, Body: "a", ReplyTo: &q1},
		{ID: q2, StepID: &stepA, Kind: MessageKindQuestion, Body: "second"},
		{ID: uuid.New(), StepID: &stepA, Kind: MessageKindSummary, Body: "s"},
		{ID: q3, StepID: &stepB, Kind: MessageKindQuestion, Body: "older open"},
		{ID: q4, StepID: &stepB, Kind: MessageKindQuestion, Body: "newer open"},
		{ID: uuid.New(), Kind: MessageKindQuestion, Body: "no step"},
	}
	got := pendingQuestions(msgs)
	if len(got) != 2 || got[stepA] == nil || got[stepA].ID != q2 || got[stepB] == nil || got[stepB].ID != q4 {
		t.Errorf("pending = %v", got)
	}
	// Answering the newest question of B leaves the older one pending.
	msgs = append(msgs, ChatMessageView{ID: uuid.New(), StepID: &stepB, Kind: MessageKindAnswer, Body: "x", ReplyTo: &q4})
	if got := pendingQuestions(msgs); got[stepB] == nil || got[stepB].ID != q3 {
		t.Errorf("pending after answering q4 = %v", got)
	}
}

func TestQuestionsLeft(t *testing.T) {
	t.Parallel()
	for asked, want := range map[int]int{0: 2, 1: 1, 2: 0, 3: 0} {
		if got := (&fillCtx{questions: asked}).questionsLeft(); got != want {
			t.Errorf("questionsLeft after %d questions = %d, want %d", asked, got, want)
		}
	}
}
