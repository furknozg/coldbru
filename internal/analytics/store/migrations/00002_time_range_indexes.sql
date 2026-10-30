-- Time indexes for the dashboard's range filters and series (D-097): llm_usage by created_at,
-- campaign_summary by completed_at. Range predicates are half-open [from, to); a one-column btree
-- serves both bounds and the date_trunc grouping reads in index order.

-- +goose Up
CREATE INDEX llm_usage_created_at_idx ON llm_usage (created_at);

CREATE INDEX campaign_summary_completed_at_idx ON campaign_summary (completed_at);

-- +goose Down
DROP INDEX campaign_summary_completed_at_idx;
DROP INDEX llm_usage_created_at_idx;
