# Prompt evaluation (`make eval`, D-115)

`make eval` plays hand-written model answers through the validation pipeline that production uses
and prints one line per case:

```
PASS  segment-unknown-field  segment  expected=repair_requested/UNKNOWN_FIELD  actual=repair_requested/UNKNOWN_FIELD  (accepted after 2 turns: ...)
```

It exits non-zero on any mismatch. No API key is needed and none is read: the real
`anthropic.Client` is pointed at the scripted Messages API server of `internal/proposal/llm/llmtest`
with a dummy key, and the server answers with the turns recorded in the case. It is not part of
`make test` (the runner is behind the `eval` build tag); CI runs it as its own step after `make test`.

Run one case with `make eval EVAL_RUN=content-sms-good`. The equivalent plain command is
`cd eval && go test -tags=eval -count=1` (run from this directory with no package argument, so that
go test streams the per-case lines instead of holding them until a failure).

## What the answers are

The recorded answers are **written by hand**, not captured from a model. Each case shows what the
validators do with a particular answer, good or broken: it proves that a known defect is caught and
that a sound answer passes, not what Sonnet or Haiku actually write for the brief. Tone, model
comparison and real-model regressions are out of scope (D-115 dropped `make eval-live` and
promptfoo).

## What runs, and what is stubbed

| prompt | entry point | production steps covered | not covered |
| --- | --- | --- | --- |
| `screen` | `anthropic.Client.Screen` | stop reason check (refusal, `max_tokens` fail closed), structured output parsing (`parseVerdict`), block verdict as `ScreenBlockedError` | the title check of the flow (`llm.CheckTitle`, D-088) |
| `segment` | `anthropic.Client.ProposeSegment` over `llm.NewCustomerTools` | tool loop and its attempt and turn limits, tool input parsing, `compile.Compile` = `dsl.Parse` then `catalog.Check` (field names, operators, types, closed values), the empty-audience rule of `llm.CustomerTools`, the `is_error` tool_result the model gets back (`llm.FormatErrors`), nudges after a text-only or cut-off turn, `send_summary` checks | **the audience count**: it needs Customer and Postgres, so the Customer client is replaced by a stub that answers `audience_count` (default 46) or the per-filter `audience_by_dsl` of the case. The GraphQL filter compiled from the DSL is built but not sent anywhere, so Customer's own filter errors (unknown city, D-062) are not exercised. |
| `content` | `contentgen.Generator.Generate` over `anthropic.Client.StreamStructured` | streamed structured output, stop reason check, JSON parsing, the document shape check of contentgen (requested channel filled, other null), every rule of `internal/content` (`CheckSMS`, `CheckEmail`: lengths with `{{first_name}}` expanded to the longest name, placeholder whitelist, URL and HTML bans, single line, paragraph counts), the SMS budget of D-109 (`content.SMSBudget` of the default `TRACKING_BASE_URL`, 125), measurements and cost with the demo prices, the repair message with `repair.problems` | the longest first name comes from the case, not from Customer; `channel_plan` and `schedule` prompts are not evaluated |

The verdict of a case is read from what the pipeline sent back to the "model", not from internal
state: a segment repair is the first error code in the `is_error` tool_result of the second request;
a content repair is the first problem (`rule`, `field`) in the `repair` object of the second request's
user message. The only production code replaced is the Customer client of the segment step.

## Case format

One JSON file per case in `cases/`; the file name without `.json` is the case name. Unknown keys are
rejected, so a typo fails the run.

```json
{
  "description": "What the case shows.",
  "prompt": "segment",
  "input": {
    "brief": "The marketer's brief.",
    "context_notes": ["optional"],
    "audience_count": 46,
    "audience_by_dsl": {"city === \"Ankara\"": 0},
    "language": "tr", "channel": "sms", "recipients": 33, "max_first_name_length": 9
  },
  "turns": [
    {"tool_calls": [{"name": "propose_segment", "input": {"dsl": "..."}}]},
    {"json": {"sms": {"text": "..."}, "email": null, "summary": "..."}},
    {"text": "{\"sms\":{\"text\":\"cut", "stop_reason": "max_tokens"},
    {"refusal": {"category": "general_harms", "explanation": "..."}}
  ],
  "expect": {"verdict": "repair_requested", "error": "UNKNOWN_FIELD", "field": "sms.text"}
}
```

- `prompt`: `screen`, `segment` or `content` (the prompt packages of `internal/proposal/prompts`).
- `input`: `brief` and `context_notes` for every prompt; `audience_count` and `audience_by_dsl`
  (keyed by the exact DSL text) for `segment`; `language`, `channel`, `recipients` and
  `max_first_name_length` for `content`.
- `turns`: the assistant messages the scripted server answers with, in order, in the shape the
  validators consume. `json` is the structured output of a screen or content turn (compacted before
  it is sent); `text` is raw text for an answer that is cut off, not JSON, or prose instead of a
  tool call; `tool_calls` are the tool uses of a segment turn (`propose_segment`, `count_audience`,
  `ask_user`, `send_summary`) with their inputs; `stop_reason` overrides the default (`tool_use`
  with tool calls, `end_turn` otherwise), for example `max_tokens`; `refusal` makes the turn a
  refusal with these `stop_details`.
- `expect.verdict`:
  - `accepted`: the first turn passed every check.
  - `repair_requested`: the first turn was rejected, the pipeline asked the model to repair it, and
    the next turn was accepted. `error` is the DSL error code (`UNKNOWN_FIELD`, `SYNTAX_ERROR`,
    `TYPE_MISMATCH`, `UNKNOWN_VALUE`, `EMPTY_AUDIENCE`, ...), the content rule (`max_length`,
    `placeholder`, `url`, `invalid_output`, ...), or `no_tool_call` / `max_tokens` for a segment turn
    that was nudged. `field` (content only, optional) is the field of the first problem.
  - `rejected`: the call ended in a terminal error; `error` is `refusal`, `max_tokens`,
    `invalid_output`, `repair_exhausted`, `turn_limit`, `tools_error` or `api_error`.
  - `flagged`: the pre-screen blocked the brief.

A segment turn that proposes an accepted filter should also call `send_summary` in the same turn;
otherwise the loop asks for one more turn, which the case must then provide. For
`repair_requested` the case must give the follow-up turn; a case that gives three rejected turns
ends `rejected/repair_exhausted`.

## Adding a case

1. Copy the closest file in `cases/` and give it a name that says what the answer does wrong (or
   right): `<prompt>-<what>.json`.
2. Write the answer in the shape above. Write it by hand; do not paste a real model answer that
   contains customer data.
3. Set `expect` to the outcome the validators *should* produce, then run `make eval`. A mismatch
   is either a wrong expectation or a validator defect; decide which before changing either.

## Limits

- The answers are hand-written; the eval does not measure the model.
- The audience count and the longest first name are inputs of the case, not query results.
- Prompt text changes do not change the verdicts: the runner sends the real prompt package (and
  records its version and hash as production does), but the scripted server ignores it. A prompt
  regression shows up only with a real model, which this eval does not call.
