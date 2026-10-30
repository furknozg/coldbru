-- The chat between the agent and the marketer (D-088): the agent's clarifying questions, the
-- marketer's answers and the agent's step summaries. Append-only like context_note and llm_call
-- (the reject_row_change trigger of 00002). A question may carry 2 to 4 options (a JSON array of
-- strings); an answer points at its question through reply_to and a question has at most one answer.

-- +goose Up
CREATE TABLE chat_message (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id uuid NOT NULL REFERENCES campaign (id),
    -- The step the message belongs to: the step that asked or was summarised, or the step of the
    -- answered question.
    step_id     uuid REFERENCES proposal_step (id),
    kind        text NOT NULL,
    body        text NOT NULL,
    -- Only a question has options: a JSON array of strings.
    options     jsonb,
    -- An answer names its question; nothing else does.
    reply_to    uuid REFERENCES chat_message (id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chat_message_kind_check CHECK (kind IN ('question', 'answer', 'summary')),
    CONSTRAINT chat_message_body_check CHECK (body ~ '\S'),
    CONSTRAINT chat_message_options_check CHECK (
        options IS NULL
        OR (kind = 'question' AND jsonb_typeof(options) = 'array' AND NOT (options @? '$[*] ? (@.type() != "string")'))
    ),
    CONSTRAINT chat_message_reply_to_check CHECK ((kind = 'answer') = (reply_to IS NOT NULL)),
    -- At most one answer per question.
    CONSTRAINT chat_message_reply_to_key UNIQUE (reply_to)
);

CREATE INDEX chat_message_campaign_id_idx ON chat_message (campaign_id);
CREATE INDEX chat_message_step_id_idx ON chat_message (step_id);

CREATE TRIGGER chat_message_append_only
    BEFORE UPDATE OR DELETE ON chat_message
    FOR EACH ROW EXECUTE FUNCTION reject_row_change();

-- +goose Down
DROP TRIGGER chat_message_append_only ON chat_message;
DROP INDEX chat_message_step_id_idx;
DROP INDEX chat_message_campaign_id_idx;
DROP TABLE chat_message;
