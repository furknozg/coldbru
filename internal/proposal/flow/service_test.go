package flow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// deadPool is a pool that never connects: nothing listens on port 1. pgxpool connects lazily, so
// building it needs no server.
func deadPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(t.Context(), "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestNewChecksItsDependencies(t *testing.T) {
	t.Parallel()
	pool := deadPool(t)
	tests := []struct {
		name string
		deps Deps
		want string
	}{
		{"no pool", Deps{LLM: llmtest.NewScripted(), Tools: &llmtest.Tools{}}, "Pool"},
		{"no llm", Deps{Pool: pool, Tools: &llmtest.Tools{}}, "LLM"},
		{"no tools and no customer", Deps{Pool: pool, LLM: llmtest.NewScripted()}, "Tools or Deps.Customer"},
	}
	for _, tt := range tests {
		if _, err := New(tt.deps); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tt.name, err, tt.want)
		}
	}
	one, err := pgxpool.New(t.Context(), "postgres://nobody@127.0.0.1:1/none?sslmode=disable&pool_max_conns=1")
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()
	if _, err := New(Deps{Pool: one, LLM: llmtest.NewScripted(), Tools: &llmtest.Tools{}}); err == nil || !strings.Contains(err.Error(), "at least 2 connections") {
		t.Errorf("a pool of one connection: err = %v", err)
	}
	s, err := New(Deps{Pool: pool, LLM: llmtest.NewScripted(), Customer: previewer{}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(t.Context())
	if s.Hub() == nil || s.tools == nil || s.clock == nil || s.confirm == nil || s.log == nil {
		t.Error("defaults were not filled in")
	}
	if _, err := s.CreateCampaign(t.Context(), " \n"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("an empty brief: err = %v", err)
	}
	if err := s.Reject(t.Context(), uuid.New(), "  "); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("an empty reason: err = %v", err)
	}
	if _, err := s.AddContext(t.Context(), uuid.New(), ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("an empty note: err = %v", err)
	}
	if _, err := s.Reopen(t.Context(), uuid.New(), "bogus"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("an unknown kind: err = %v", err)
	}
}

type previewer struct{}

func (previewer) Preview(context.Context, *compile.Filter) (*customer.Preview, error) {
	return &customer.Preview{Count: 1}, nil
}

// A worker that cannot take the campaign lock fails its step instead of hanging, leaves no job
// behind, and says why.
func TestUnlockableCampaignFailsTheJob(t *testing.T) {
	t.Parallel()
	s, err := New(Deps{Pool: deadPool(t), LLM: llmtest.NewScripted(), Tools: &llmtest.Tools{}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(t.Context())
	campaign, step := uuid.New(), uuid.New()
	sub := s.Hub().Subscribe(campaign)
	defer sub.Close()

	s.enqueue(job{kind: jobFill, campaign: campaign, step: step})
	s.enqueue(job{kind: jobFill, campaign: campaign, step: step}) // while the first waits: dropped
	if !s.isGenerating(step) {
		t.Error("a queued job does not count as generating")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatalf("WaitIdle: %v", err)
	}
	if s.isGenerating(step) || s.pending != 0 || len(s.runners) != 0 {
		t.Errorf("state after the job: generating %v, pending %d, runners %d", s.isGenerating(step), s.pending, len(s.runners))
	}
	select {
	case e := <-sub.C:
		f, ok := e.(GenerationFailed)
		if !ok || f.StepID != step || !strings.Contains(f.Reason, "could not lock the campaign") {
			t.Errorf("event = %#v", e)
		}
	default:
		t.Error("no GenerationFailed event")
	}
	if got := s.failureOf(step); !strings.Contains(got, "could not lock") {
		t.Errorf("failureOf = %q", got)
	}
	s.clearFailures(step)
	if s.failureOf(step) != "" {
		t.Error("clearFailures left a failure")
	}
}

func TestCloseStopsAcceptingJobs(t *testing.T) {
	t.Parallel()
	s, err := New(Deps{Pool: deadPool(t), LLM: llmtest.NewScripted(), Tools: &llmtest.Tools{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.enqueue(job{kind: jobFill, campaign: uuid.New(), step: uuid.New()})
	if s.pending != 0 {
		t.Error("a closed service queued a job")
	}
}

func TestFailureReasonsAreJoinedBySlot(t *testing.T) {
	t.Parallel()
	s, _ := New(Deps{Pool: deadPool(t), LLM: llmtest.NewScripted(), Tools: &llmtest.Tools{}})
	defer s.Close(t.Context())
	step := uuid.New()
	s.setFailure(step, Slot{}, "boom")
	if got := s.failureOf(step); got != "boom" {
		t.Errorf("failureOf = %q", got)
	}
	s.setFailure(step, Slot{Language: "tr", Channel: "sms"}, "refused")
	s.setFailure(step, Slot{Language: "en", Channel: "email"}, "cut off")
	s.setFailure(step, Slot{Language: "tr", Timezone: "Europe/London"}, "late")
	s.setFailure(step, Slot{Language: "tr", Timezone: "Europe/Istanbul"}, "early")
	if got := s.failureOf(step); got != "-: boom; en/email: cut off; tr@Europe/Istanbul: early; tr@Europe/London: late; tr/sms: refused" {
		t.Errorf("failureOf = %q", got)
	}
}

func TestLockKey(t *testing.T) {
	t.Parallel()
	a, b := uuid.New(), uuid.New()
	if LockKey(a) != LockKey(a) {
		t.Error("the key is not stable")
	}
	if LockKey(a) == LockKey(b) {
		t.Error("two campaigns share a key")
	}
	// UUIDv7 ids created in the same millisecond share their first bytes; the key must still differ.
	x, y := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	if LockKey(x) == LockKey(y) {
		t.Error("neighbouring UUIDv7 ids share a key")
	}
}

func TestFailureOfCalls(t *testing.T) {
	t.Parallel()
	row := func(purpose, outcome, errText string) store.LlmCall {
		return store.LlmCall{Purpose: purpose, Outcome: outcome, Error: pgtype.Text{String: errText, Valid: errText != ""}}
	}
	tests := []struct {
		name   string
		rows   []store.LlmCall // newest first
		failed bool
		reason string
	}{
		{"no calls yet", nil, false, ""},
		{"accepted", []store.LlmCall{row("segment", "accepted", "")}, false, ""},
		{"repaired: the newest is accepted", []store.LlmCall{row("segment", "accepted", ""), row("segment", "validation_failed", "bad")}, false, ""},
		{"repair exhausted", []store.LlmCall{row("segment", "validation_failed", "propose_segment was rejected"), row("segment", "validation_failed", "x"), row("segment", "validation_failed", "y")},
			true, "segment: validation_failed: propose_segment was rejected"},
		{"refusal", []store.LlmCall{row("channel_plan", "refusal", "refusal (stop_details.category: general_harms)")}, true, "channel_plan: refusal: refusal (stop_details.category: general_harms)"},
		{"api error", []store.LlmCall{row("schedule", "api_error", "HTTP 529")}, true, "schedule: api_error: HTTP 529"},
		{"failure without text", []store.LlmCall{row("content", "validation_failed", "")}, true, "content: validation_failed: validation_failed"},
	}
	for _, tt := range tests {
		failed, reason := failureOfCalls(tt.rows)
		if failed != tt.failed || reason != tt.reason {
			t.Errorf("%s: = %v, %q; want %v, %q", tt.name, failed, reason, tt.failed, tt.reason)
		}
	}
}

func TestModelNotesAddTheRejectionReason(t *testing.T) {
	t.Parallel()
	rejected := func(lang, reason string) store.ProposalVariant {
		return store.ProposalVariant{Language: pgtype.Text{String: lang, Valid: lang != ""}, RejectionReason: pgtype.Text{String: reason, Valid: true}}
	}
	fc := &fillCtx{
		step:     store.ProposalStep{Kind: KindChannelPlan},
		notes:    []string{"keep it short"},
		rejected: map[Slot]store.ProposalVariant{{}: rejected("", "too wide")},
	}
	got := fc.modelNotes(Slot{})
	if len(got) != 2 || got[0] != "keep it short" || !strings.Contains(got[1], "rejected the previous channel plan proposal") || !strings.HasSuffix(got[1], "Reason: too wide") {
		t.Errorf("notes = %q", got)
	}
	// A slot that was not rejected adds nothing, and the notes are not aliased.
	if n := fc.modelNotes(Slot{Language: "tr"}); len(n) != 1 {
		t.Errorf("notes = %q", n)
	}
	got[0] = "changed"
	if fc.notes[0] != "keep it short" {
		t.Error("modelNotes aliases the notes of the context")
	}
	fc.step.Kind = KindSchedule
	fc.rejected = map[Slot]store.ProposalVariant{{Language: "tr", Timezone: "Europe/Istanbul"}: rejected("tr", "too early")}
	if n := fc.modelNotes(Slot{Language: "tr", Timezone: "Europe/Istanbul"}, Slot{Language: "tr", Timezone: "Europe/London"}); len(n) != 2 || !strings.Contains(n[1], "schedule (tr@Europe/Istanbul) proposal") {
		t.Errorf("notes = %q", n)
	}
}

func TestNewAudienceContent(t *testing.T) {
	t.Parallel()
	p := &customer.Preview{
		Count: 46, MaxFirstNameLength: 9, Coverage: customer.Coverage{Both: 20, EmailOnly: 13, SmsOnly: 13, Unreachable: 3},
		ByLanguage: []customer.LanguageCoverage{
			{Language: "tr", Count: 30, MaxFirstNameLength: 9, Coverage: customer.Coverage{Both: 13, EmailOnly: 8, SmsOnly: 9, Unreachable: 2}},
			{Language: "en", Count: 16, MaxFirstNameLength: 7, Coverage: customer.Coverage{Both: 7, EmailOnly: 5, SmsOnly: 4, Unreachable: 1}},
		},
		ByLocale: []customer.LocaleCoverage{
			{Language: "tr", Timezone: "Europe/London", Count: 1, MaxFirstNameLength: 3, Coverage: customer.Coverage{EmailOnly: 1}},
			{Language: "en", Timezone: "Europe/London", Count: 0, Coverage: customer.Coverage{Unreachable: 1}},
			{Language: "tr", Timezone: "Europe/Istanbul", Count: 29, MaxFirstNameLength: 9, Coverage: customer.Coverage{Both: 13, EmailOnly: 7, SmsOnly: 9, Unreachable: 2}},
			{Language: "en", Timezone: "Europe/Istanbul", Count: 16, MaxFirstNameLength: 7, Coverage: customer.Coverage{Both: 7, EmailOnly: 5, SmsOnly: 4}},
		},
	}
	c, err := NewAudienceContent(`city === "Ankara"`, nil, p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Count != 46 || c.Coverage != sample || c.ByLanguage[0].Language != "en" || c.ByLanguage[1].Count != 30 {
		t.Errorf("content = %+v", c)
	}
	if string(c.Filter) != "null" {
		t.Errorf("filter = %s, want null for a missing filter", c.Filter)
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"dsl"`, `"filter"`, `"count"`, `"coverage"`, `"email_only"`, `"sms_only"`, `"unreachable"`, `"by_language"`, `"by_locale"`, `"timezone":"Europe/London"`, `"max_first_name_length":9`, `"max_first_name_length":7`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("content lacks %s: %s", key, b)
		}
	}
	// by_locale is ordered by language then timezone whatever order Customer answered in (D-076).
	var order []string
	for _, l := range c.ByLocale {
		order = append(order, l.Language+"/"+l.Timezone)
	}
	if got := strings.Join(order, " "); got != "en/Europe/Istanbul en/Europe/London tr/Europe/Istanbul tr/Europe/London" {
		t.Errorf("by_locale order = %s", got)
	}
	if c.ByLocale[1].Count != 0 || c.ByLocale[3].Coverage.EmailOnly != 1 {
		t.Errorf("by_locale = %+v", c.ByLocale)
	}
	// The round trip through the coverage table the plan prompt reads.
	back := c.Preview()
	if len(back.ByLocale) != 4 || back.ByLocale[3].Timezone != "Europe/London" || back.ByLocale[3].Coverage.EmailOnly != 1 {
		t.Errorf("Preview() lost by_locale: %+v", back.ByLocale)
	}
	if back.Count != 46 || back.Coverage != p.Coverage || len(back.ByLanguage) != 2 || back.ByLanguage[0].Language != "en" {
		t.Errorf("Preview() = %+v", back)
	}
	if back.MaxFirstNameLength != 9 || back.ByLanguage[0].MaxFirstNameLength != 7 || back.ByLanguage[1].MaxFirstNameLength != 9 {
		t.Errorf("Preview() lost the longest first names: %+v", back)
	}
	// Each language is measured with its own audience's longest name; an unknown language with the overall one.
	for lang, want := range map[string]int{"en": 7, "tr": 9, "de": 9} {
		if got := c.MaxFirstNameLengthOf(lang); got != want {
			t.Errorf("MaxFirstNameLengthOf(%s) = %d, want %d", lang, got, want)
		}
	}
	if _, err := NewAudienceContent("x", nil, nil); err == nil {
		t.Error("a missing preview was accepted")
	}
}

func TestCheckObject(t *testing.T) {
	t.Parallel()
	for in, ok := range map[string]bool{`{"a":1}`: true, ` {"a":1} `: true, `[]`: false, `"x"`: false, `{`: false, ``: false, `null`: false} {
		if err := checkObject(json.RawMessage(in)); (err == nil) != ok {
			t.Errorf("checkObject(%q) = %v, want ok=%v", in, err, ok)
		}
	}
}

func TestInvalidContentError(t *testing.T) {
	t.Parallel()
	err := error(&InvalidContentError{Problems: []string{"a", "b"}})
	if !errors.Is(err, ErrInvalidContent) || errors.Is(err, ErrNotDraft) || !strings.Contains(err.Error(), "a; b") {
		t.Errorf("err = %v", err)
	}
	var ice *InvalidContentError
	if !errors.As(err, &ice) || len(ice.Problems) != 2 {
		t.Error("errors.As failed")
	}
}

func TestSlotString(t *testing.T) {
	t.Parallel()
	for slot, want := range map[Slot]string{
		{}: "-", {Language: "tr"}: "tr", {Language: "tr", Channel: "sms"}: "tr/sms",
		{Language: "tr", Timezone: "Europe/Istanbul"}: "tr@Europe/Istanbul",
	} {
		if slot.String() != want {
			t.Errorf("%+v = %q, want %q", slot, slot.String(), want)
		}
	}
}

func TestCostOf(t *testing.T) {
	t.Parallel()
	v := func(status, channel, content string) VariantView {
		return VariantView{Status: status, Channel: channel, Content: json.RawMessage(content)}
	}
	steps := []StepView{
		{StepInfo: StepInfo{Kind: KindAudience}, Variants: []VariantView{v(VariantStatusApproved, "", `{"estimated_cost_gbp":"500.0000"}`)}},
		{StepInfo: StepInfo{Kind: KindContent}, Variants: []VariantView{
			v(VariantStatusDraft, "sms", `{"text":"a","estimated_cost_gbp":"1.3200"}`),
			v(VariantStatusApproved, "sms", `{"estimated_cost_gbp":"2.6400"}`),
			v(VariantStatusRejected, "sms", `{"estimated_cost_gbp":"100.0000"}`),
			v(VariantStatusDraft, "email", `{"estimated_cost_gbp":"0.0330"}`),
			v(VariantStatusDraft, "email", `{"estimated_cost_gbp":"0.0001"}`),
			v(VariantStatusDraft, "email", `{"text":"fake content without an estimate"}`),
			v(VariantStatusDraft, "email", `{"estimated_cost_gbp":"not money"}`),
			v(VariantStatusDraft, "email", `not json`),
			v(VariantStatusDraft, "push", `{"estimated_cost_gbp":"7.0000"}`),
		}},
	}
	want := CostSummary{SMSGBP: "3.9600", EmailGBP: "0.0331", TotalGBP: "3.9931"}
	if got := costOf(steps); got != want {
		t.Errorf("costOf = %+v, want %+v", got, want)
	}
	if got := costOf(nil); got != (CostSummary{SMSGBP: "0.0000", EmailGBP: "0.0000", TotalGBP: "0.0000"}) {
		t.Errorf("costOf(nil) = %+v", got)
	}
	// 0.1 + 0.2 style sums do not drift.
	more := []StepView{{StepInfo: StepInfo{Kind: KindContent}, Variants: []VariantView{
		v(VariantStatusDraft, "sms", `{"estimated_cost_gbp":"0.1000"}`), v(VariantStatusDraft, "email", `{"estimated_cost_gbp":"0.2000"}`)}}}
	if got := costOf(more); got.TotalGBP != "0.3000" {
		t.Errorf("total = %s", got.TotalGBP)
	}
}
