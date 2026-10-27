// Package consent reads and changes the per-channel consent of customers
// (D-064). The eligibility rule is the audience reachability rule (D-062); this
// package reuses its SQL so the two cannot drift apart.
package consent

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

// Channels (D-064).
const (
	ChannelEmail = "email"
	ChannelSMS   = "sms"
)

// Sources an update may record: a closed list (D-064). signup_form and
// account_deletion exist only in the seed.
var Sources = []string{"preference_centre", "support_request", "unsubscribe_link", "admin_tool"}

// Limits and fixed values (D-064).
const (
	// MaxIDs is the largest customerIds list of consentStatus.
	MaxIDs = 1000
	// statementTimeout bounds every consent transaction, as in the audience queries (D-062).
	statementTimeout = "5s"
	// idLockKey is the pg_advisory_xact_lock key that serializes consent id allocation.
	idLockKey int64 = 6_004_064
	// idPrefix and idDigits form consent ids: CNS-002220. A larger number simply gets more digits.
	idPrefix = "CNS-"
	idDigits = 6
)

// Service reads and writes consent in customer_db.
type Service struct {
	db audience.Beginner
}

// NewService returns a Service over db.
func NewService(db audience.Beginner) *Service { return &Service{db: db} }

// channelSQL returns the dim_customer opt-in column and the full eligibility
// condition of channel, or a BAD_USER_INPUT error. Both come from constants, so
// they are safe to put into SQL.
func channelSQL(channel string) (column, eligible string, err error) {
	switch channel {
	case ChannelEmail:
		return "email_opt_in", audience.SQLActive + " AND " + audience.SQLEmailOK, nil
	case ChannelSMS:
		return "sms_opt_in", audience.SQLActive + " AND " + audience.SQLSMSOK, nil
	}
	return "", "", audience.BadInput("channel must be %q or %q, got %q", ChannelEmail, ChannelSMS, channel)
}

// beginTx starts a transaction with the 5 second statement timeout.
func (s *Service) beginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '"+statementTimeout+"'"); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set statement_timeout: %w", err)
	}
	return tx, nil
}

// Status returns one row per requested id, in request order. An unknown id is
// not opted in and not eligible (fail-closed, D-064).
func (s *Service) Status(ctx context.Context, ids []string, channel string) ([]*model.ConsentStatus, error) {
	if _, _, err := channelSQL(channel); err != nil {
		return nil, err
	}
	if len(ids) > MaxIDs {
		return nil, audience.BadInput("customerIds holds at most %d ids, got %d", MaxIDs, len(ids))
	}
	if len(ids) == 0 {
		return []*model.ConsentStatus{}, nil
	}
	tx, err := s.beginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return statusRows(ctx, tx, ids, channel)
}

// statusRows runs the status query inside tx.
func statusRows(ctx context.Context, tx pgx.Tx, ids []string, channel string) ([]*model.ConsentStatus, error) {
	column, eligible, err := channelSQL(channel)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx,
		"SELECT c.customer_id, c."+column+", "+eligible+" FROM dwh.dim_customer c WHERE c.customer_id = ANY($1)", ids)
	if err != nil {
		return nil, fmt.Errorf("consent status query: %w", err)
	}
	defer rows.Close()
	found := make(map[string]*model.ConsentStatus, len(ids))
	for rows.Next() {
		st := &model.ConsentStatus{Channel: channel}
		if err := rows.Scan(&st.CustomerID, &st.OptedIn, &st.Eligible); err != nil {
			return nil, fmt.Errorf("consent status scan: %w", err)
		}
		found[st.CustomerID] = st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("consent status rows: %w", err)
	}
	out := make([]*model.ConsentStatus, len(ids))
	for i, id := range ids {
		if st, ok := found[id]; ok {
			cp := *st // a repeated id gets its own row
			out[i] = &cp
			continue
		}
		out[i] = &model.ConsentStatus{CustomerID: id, Channel: channel}
	}
	return out, nil
}

// Update records an opt-in or opt-out and returns the new status (D-064). The
// customer row is locked; if the flag already has the wanted value nothing is
// written. Otherwise one fact_consent row is inserted and the flag and
// consent_updated_at change in the same transaction, so the flag always equals
// the latest fact_consent row of the channel (D-005).
func (s *Service) Update(ctx context.Context, in model.UpdateConsentInput) (*model.ConsentStatus, error) {
	if _, _, err := channelSQL(in.Channel); err != nil {
		return nil, err
	}
	if !slices.Contains(Sources, in.Source) {
		return nil, audience.BadInput("source must be one of %v, got %q", Sources, in.Source)
	}
	if in.CustomerID == "" {
		return nil, audience.BadInput("customerId must not be empty")
	}
	tx, err := s.beginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var accountStatus string
	var emailIn, smsIn bool
	err = tx.QueryRow(ctx,
		"SELECT account_status, email_opt_in, sms_opt_in FROM dwh.dim_customer WHERE customer_id = $1 FOR UPDATE",
		in.CustomerID).Scan(&accountStatus, &emailIn, &smsIn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, audience.NotFound("customer %q does not exist", in.CustomerID)
	}
	if err != nil {
		return nil, fmt.Errorf("lock customer: %w", err)
	}
	if accountStatus == "deleted" {
		return nil, audience.BadInput("customer %q is deleted; consent cannot change", in.CustomerID)
	}
	current := emailIn
	if in.Channel == ChannelSMS {
		current = smsIn
	}
	if current != in.OptedIn {
		if err := writeChange(ctx, tx, in); err != nil {
			return nil, err
		}
	}
	rows, err := statusRows(ctx, tx, []string{in.CustomerID}, in.Channel)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit consent change: %w", err)
	}
	return rows[0], nil
}

// writeChange inserts the fact_consent row and flips the flag. The advisory
// lock makes the max+1 id allocation safe against concurrent updates of other
// customers; it is released at commit or rollback.
func writeChange(ctx context.Context, tx pgx.Tx, in model.UpdateConsentInput) error {
	column, _, err := channelSQL(in.Channel)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", idLockKey); err != nil {
		return fmt.Errorf("lock consent ids: %w", err)
	}
	var next int64
	if err := tx.QueryRow(ctx,
		`SELECT coalesce(max(substring(consent_id from '^CNS-([0-9]+)$')::bigint), 0) + 1 FROM dwh.fact_consent`,
	).Scan(&next); err != nil {
		return fmt.Errorf("next consent id: %w", err)
	}
	status := "opted_out"
	if in.OptedIn {
		status = "opted_in"
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO dwh.fact_consent (consent_id, customer_id, channel, consent_status, source, changed_at)
		 VALUES ($1, $2, $3, $4, $5, now())`,
		ConsentID(next), in.CustomerID, in.Channel, status, in.Source); err != nil {
		return fmt.Errorf("insert consent row: %w", err)
	}
	if _, err := tx.Exec(ctx,
		"UPDATE dwh.dim_customer SET "+column+" = $1, consent_updated_at = now() WHERE customer_id = $2",
		in.OptedIn, in.CustomerID); err != nil {
		return fmt.Errorf("update customer flag: %w", err)
	}
	return nil
}

// ConsentID formats consent id number n: CNS-002220, wider when n needs it.
func ConsentID(n int64) string { return fmt.Sprintf("%s%0*d", idPrefix, idDigits, n) }
