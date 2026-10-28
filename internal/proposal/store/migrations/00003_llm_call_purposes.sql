-- llm_call.purpose gains channel_plan and schedule: the plan and schedule steps of the proposal
-- flow are model calls too (D-070, which replaces the purpose list of D-068). Nothing else changes.

-- +goose Up
ALTER TABLE llm_call DROP CONSTRAINT llm_call_purpose_check;
ALTER TABLE llm_call ADD CONSTRAINT llm_call_purpose_check
    CHECK (purpose IN ('screen', 'segment', 'content', 'channel_plan', 'schedule'));

-- +goose Down
-- Fails when llm_call already holds a channel_plan or schedule row; the table is append-only, so
-- rolling back past this migration is only possible on a database that has none.
ALTER TABLE llm_call DROP CONSTRAINT llm_call_purpose_check;
ALTER TABLE llm_call ADD CONSTRAINT llm_call_purpose_check
    CHECK (purpose IN ('screen', 'segment', 'content'));
