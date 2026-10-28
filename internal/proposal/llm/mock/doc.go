// Package mock is the production mock of llm.LLM (D-072): a deterministic model that needs no API
// key, so that `docker compose up` works without one. It is not the test double (that is
// llmtest.Scripted, which plays whatever a test scripts); it answers a fixed set of briefs with
// hand-written responses, and every other brief with a simple default.
//
//   - Screen always allows (reason "mock").
//   - ProposeSegment returns the hand-written DSL filter of the sample brief and of every past
//     campaign brief of stub/generate_stub_data.py (matched on the normalized brief text), and the
//     default filter "totalOrders >= 1" for any other brief. The filter goes through the
//     SegmentTools it is given, as in the real loop, so the audience, the validation and the
//     "empty audience" rule are the real ones.
//   - ProposeChannelPlan proposes primary_fallback with the channel that reaches more people as the
//     primary (email on a tie, D-010).
//   - ProposeSchedule proposes "send immediately" (send_at null) for every language.
//   - StreamStructured writes SMS or email content for the requested language and channel from
//     fixed templates, streamed in a few chunks. The text of a brief that is not hand-written
//     carries a "[mock]" marker, and the rationales of the plan and the schedule say "mock yanıt".
//
// A call that is a regeneration (the input carries a rejection reason or a context note) says so
// visibly while the output stays deterministic: the rationales begin "[mock] yeniden üretim
// (regeneration): <reason or note>" and the content text gets the "[mock]" marker. The audience
// step has no text to mark and is unchanged.
//
// Every answer passes the same validators as a real model's, and every call reports a CallRecord
// with model "mock", zero tokens and the real prompt package identity, so the llm_call table looks
// the same in both modes.
package mock
