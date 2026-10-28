package audience

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

var errStop = errors.New("stop here")

// fakeTx records what the service sends and fails the first query, so a test
// sees the exact SQL without a database.
type fakeTx struct {
	pgx.Tx
	order      []string
	execSQL    []string
	querySQL   string
	queryArgs  []any
	rolledBack bool
}

func (f *fakeTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.order = append(f.order, "exec")
	f.execSQL = append(f.execSQL, sql)
	return pgconn.CommandTag{}, nil
}

func (f *fakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.order = append(f.order, "query")
	f.querySQL, f.queryArgs = sql, args
	return nil, errStop
}

func (f *fakeTx) Rollback(context.Context) error { f.rolledBack = true; return nil }

type fakeDB struct {
	tx    *fakeTx
	opts  pgx.TxOptions
	begun int
}

func (d *fakeDB) BeginTx(_ context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	d.begun++
	d.opts = opts
	d.tx = &fakeTx{}
	return d.tx, nil
}

func testService(t *testing.T) (*Service, *fakeDB) {
	t.Helper()
	db := &fakeDB{}
	return NewService(db, testCompiler(t).Cities), db
}

func istanbulFilter() *model.CustomerFilter {
	return &model.CustomerFilter{City: &model.StringFilter{Eq: ptr("istanbul")}}
}

func TestPreviewRunsInReadOnlyTxWithTimeout(t *testing.T) {
	t.Parallel()
	svc, db := testService(t)
	_, err := svc.Preview(t.Context(), istanbulFilter())
	if !errors.Is(err, errStop) {
		t.Fatalf("error = %v, want the fake query failure", err)
	}
	if db.opts.AccessMode != pgx.ReadOnly {
		t.Errorf("access mode = %q, want read only", db.opts.AccessMode)
	}
	if got := db.tx.execSQL; len(got) != 1 || got[0] != "SET LOCAL statement_timeout = '5s'" {
		t.Errorf("exec = %v, want SET LOCAL statement_timeout = '5s'", got)
	}
	if !reflect.DeepEqual(db.tx.order, []string{"exec", "query"}) {
		t.Errorf("order = %v, want the timeout set before the query", db.tx.order)
	}
	if !db.tx.rolledBack {
		t.Error("transaction was not rolled back")
	}
	want := "SELECT c.language, " +
		"count(*) FILTER (WHERE (c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced) AND (c.phone_e164 IS NOT NULL AND c.sms_opt_in)), " +
		"count(*) FILTER (WHERE (c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced) AND NOT (c.phone_e164 IS NOT NULL AND c.sms_opt_in)), " +
		"count(*) FILTER (WHERE NOT (c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced) AND (c.phone_e164 IS NOT NULL AND c.sms_opt_in)), " +
		"count(*) FILTER (WHERE NOT (c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced) AND NOT (c.phone_e164 IS NOT NULL AND c.sms_opt_in)), " +
		"COALESCE(max(char_length(c.first_name)) FILTER (WHERE ((c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced) OR (c.phone_e164 IS NOT NULL AND c.sms_opt_in))), 0)::int " +
		"FROM dwh.dim_customer c WHERE c.city IS NOT DISTINCT FROM $1 AND c.account_status = 'active' " +
		"GROUP BY c.language ORDER BY c.language"
	if db.tx.querySQL != want {
		t.Errorf("query =\n  %s\nwant\n  %s", db.tx.querySQL, want)
	}
	if !reflect.DeepEqual(db.tx.queryArgs, []any{"İstanbul"}) {
		t.Errorf("args = %v, want [İstanbul]", db.tx.queryArgs)
	}
}

func TestRecipientsQueryShape(t *testing.T) {
	t.Parallel()
	const reach = "(c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced)"
	const sms = "(c.phone_e164 IS NOT NULL AND c.sms_opt_in)"
	head := "SELECT c.customer_id, c.first_name, c.language, " + reach + ", " + sms + ", " +
		"CASE WHEN " + reach + " THEN c.email END, CASE WHEN " + sms + " THEN c.phone_e164 END " +
		"FROM dwh.dim_customer c WHERE c.city IS NOT DISTINCT FROM $1 AND c.account_status = 'active' AND (" + reach + " OR " + sms + ")"

	t.Run("first page", func(t *testing.T) {
		t.Parallel()
		svc, db := testService(t)
		_, err := svc.Recipients(t.Context(), istanbulFilter(), 10, nil)
		if !errors.Is(err, errStop) {
			t.Fatalf("error = %v, want the fake query failure", err)
		}
		if want := head + " ORDER BY c.customer_id LIMIT $2"; db.tx.querySQL != want {
			t.Errorf("query =\n  %s\nwant\n  %s", db.tx.querySQL, want)
		}
		if want := []any{"İstanbul", 11}; !reflect.DeepEqual(db.tx.queryArgs, want) {
			t.Errorf("args = %v, want %v (limit is first+1)", db.tx.queryArgs, want)
		}
		if db.opts.AccessMode != pgx.ReadOnly || len(db.tx.execSQL) != 1 {
			t.Errorf("want a read-only tx with the statement timeout, got %q and %v", db.opts.AccessMode, db.tx.execSQL)
		}
	})
	t.Run("after a cursor", func(t *testing.T) {
		t.Parallel()
		svc, db := testService(t)
		after := EncodeCursor("CUS-000042")
		_, err := svc.Recipients(t.Context(), istanbulFilter(), 5, &after)
		if !errors.Is(err, errStop) {
			t.Fatalf("error = %v, want the fake query failure", err)
		}
		if want := head + " AND c.customer_id > $2 ORDER BY c.customer_id LIMIT $3"; db.tx.querySQL != want {
			t.Errorf("query =\n  %s\nwant\n  %s", db.tx.querySQL, want)
		}
		if want := []any{"İstanbul", "CUS-000042", 6}; !reflect.DeepEqual(db.tx.queryArgs, want) {
			t.Errorf("args = %v, want %v", db.tx.queryArgs, want)
		}
	})
}

func TestRecipientsRejectsBadArgumentsBeforeTheDatabase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		first int
		after *string
	}{
		{"first zero", 0, nil},
		{"first negative", -1, nil},
		{"first over 1000", 1001, nil},
		{"cursor not base64", 10, ptr("!!!")},
		{"cursor empty", 10, ptr("")},
		{"cursor padded base64", 10, ptr("Q1VTLTAwMDAwMQ==")},
		{"cursor NUL byte", 10, ptr(EncodeCursor("a\x00b"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, db := testService(t)
			_, err := svc.Recipients(t.Context(), istanbulFilter(), tt.first, tt.after)
			ae := codeOf(t, err)
			if ae.Code != CodeBadUserInput {
				t.Errorf("code = %s, want %s", ae.Code, CodeBadUserInput)
			}
			if db.begun != 0 {
				t.Error("the database was reached")
			}
		})
	}
	t.Run("1 and 1000 are accepted", func(t *testing.T) {
		t.Parallel()
		for _, first := range []int{1, MaxPageSize} {
			svc, _ := testService(t)
			if _, err := svc.Recipients(t.Context(), istanbulFilter(), first, nil); !errors.Is(err, errStop) {
				t.Errorf("first=%d: error = %v, want to reach the database", first, err)
			}
		}
	})
}

func TestFilterErrorsSkipTheDatabase(t *testing.T) {
	t.Parallel()
	svc, db := testService(t)
	bad := &model.CustomerFilter{LoyaltyTier: &model.StringFilter{Eq: ptr("gold")}}
	if _, err := svc.Preview(t.Context(), bad); codeOf(t, err).Code != CodeUnknownValue {
		t.Errorf("Preview error = %v, want UNKNOWN_VALUE", err)
	}
	if _, err := svc.Recipients(t.Context(), bad, 10, nil); codeOf(t, err).Code != CodeUnknownValue {
		t.Errorf("Recipients error = %v, want UNKNOWN_VALUE", err)
	}
	if db.begun != 0 {
		t.Error("the database was reached")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"CUS-000001", "x", "ü-ğ", "a/b+c=d"} {
		cur := EncodeCursor(id)
		if strings.ContainsAny(cur, "+/=") {
			t.Errorf("cursor %q is not base64url without padding", cur)
		}
		got, err := DecodeCursor(cur)
		if err != nil || got != id {
			t.Errorf("DecodeCursor(EncodeCursor(%q)) = %q, %v", id, got, err)
		}
	}
}

func TestReachabilityRule(t *testing.T) {
	t.Parallel()
	if SQLActive != "c.account_status = 'active'" {
		t.Errorf("SQLActive = %s", SQLActive)
	}
	if SQLEmailOK != "(c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced)" {
		t.Errorf("SQLEmailOK = %s", SQLEmailOK)
	}
	if SQLSMSOK != "(c.phone_e164 IS NOT NULL AND c.sms_opt_in)" {
		t.Errorf("SQLSMSOK = %s", SQLSMSOK)
	}
	if SQLReachable != "("+SQLEmailOK+" OR "+SQLSMSOK+")" {
		t.Errorf("SQLReachable = %s", SQLReachable)
	}
}
