-- name: InsertClick :one
-- Records one click; first is true when the code had no click before. Run after LockLink in the
-- same transaction, so two simultaneous first clicks cannot both see an empty history. Served by
-- click_code_idx.
INSERT INTO click (code, first)
VALUES (sqlc.arg(code), NOT EXISTS (SELECT 1 FROM click c WHERE c.code = sqlc.arg(code)))
RETURNING id, clicked_at, first;

-- name: CountClicks :one
SELECT count(*)::bigint FROM click WHERE code = $1;
