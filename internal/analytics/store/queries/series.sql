-- name: SeriesBuckets :many
-- Every bucket start of the series of [from_ts, to_ts), UTC-aligned. generate_series runs on
-- timestamp (without time zone) so the step never depends on the session zone or DST; the last
-- bucket is the one that contains the instant just before to_ts, which keeps the range half-open.
SELECT (b AT TIME ZONE 'UTC')::timestamptz AS bucket_start
FROM generate_series(
    date_trunc(sqlc.arg('bucket')::text, sqlc.arg('from_ts')::timestamptz AT TIME ZONE 'UTC'),
    (sqlc.arg('to_ts')::timestamptz AT TIME ZONE 'UTC') - interval '1 microsecond',
    ('1 ' || sqlc.arg('bucket')::text)::interval
) AS b
ORDER BY b;
