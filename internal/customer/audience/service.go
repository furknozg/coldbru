package audience

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

// Query limits (D-062).
const (
	// MaxPageSize is the largest `first` of audienceRecipients.
	MaxPageSize = 1000
	// statementTimeout bounds every audience query.
	statementTimeout = "5s"
)

func itoa(n int) string { return strconv.Itoa(n) }

// Beginner starts transactions; *pgxpool.Pool implements it.
type Beginner interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// Service runs audience queries against customer_db.
type Service struct {
	db       Beginner
	compiler *Compiler
}

// NewService returns a Service over db using the city dictionary.
func NewService(db Beginner, cities *Cities) *Service {
	return &Service{db: db, compiler: &Compiler{Cities: cities}}
}

// withReadTx runs fn in a read-only transaction with a 5 second statement
// timeout (D-062). The transaction is always rolled back: nothing is written.
func (s *Service) withReadTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin read-only transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '"+statementTimeout+"'"); err != nil {
		return fmt.Errorf("set statement_timeout: %w", err)
	}
	return fn(tx)
}

// Preview counts the reachable audience of f and its channel coverage, in total
// and per language, with one query.
func (s *Service) Preview(ctx context.Context, f *model.CustomerFilter) (*model.AudiencePreview, error) {
	c, err := s.compiler.Compile(f)
	if err != nil {
		return nil, err
	}
	out := &model.AudiencePreview{Coverage: &model.ChannelCoverage{}, ByLanguage: []*model.LanguageCoverage{}}
	err = s.withReadTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, previewSQL(c.Where), c.Args...)
		if err != nil {
			return fmt.Errorf("audience preview query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var lang string
			var both, emailOnly, smsOnly, none, maxName int
			if err := rows.Scan(&lang, &both, &emailOnly, &smsOnly, &none, &maxName); err != nil {
				return fmt.Errorf("audience preview scan: %w", err)
			}
			cov := &model.ChannelCoverage{Both: both, EmailOnly: emailOnly, SmsOnly: smsOnly, Unreachable: none}
			out.ByLanguage = append(out.ByLanguage, &model.LanguageCoverage{
				Language: lang, Count: both + emailOnly + smsOnly, Coverage: cov, MaxFirstNameLength: maxName,
			})
			out.MaxFirstNameLength = max(out.MaxFirstNameLength, maxName)
			out.Count += both + emailOnly + smsOnly
			out.Coverage.Both += both
			out.Coverage.EmailOnly += emailOnly
			out.Coverage.SmsOnly += smsOnly
			out.Coverage.Unreachable += none
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Recipients returns one page of reachable customers ordered by customer_id.
// first must be 1..MaxPageSize; after is the previous page's endCursor.
func (s *Service) Recipients(ctx context.Context, f *model.CustomerFilter, first int, after *string) (*model.RecipientConnection, error) {
	if first < 1 || first > MaxPageSize {
		return nil, BadInput("first must be between 1 and %d, got %d", MaxPageSize, first)
	}
	var afterID string
	if after != nil {
		var err error
		if afterID, err = DecodeCursor(*after); err != nil {
			return nil, err
		}
	}
	c, err := s.compiler.Compile(f)
	if err != nil {
		return nil, err
	}
	args := c.Args
	if after != nil {
		args = append(args, afterID)
	}
	args = append(args, first+1) // one extra row tells whether another page exists

	out := &model.RecipientConnection{Nodes: []*model.Recipient{}, PageInfo: &model.PageInfo{}}
	err = s.withReadTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, recipientsSQL(c.Where, len(c.Args), after != nil), args...)
		if err != nil {
			return fmt.Errorf("audience recipients query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var r model.Recipient
			if err := rows.Scan(&r.CustomerID, &r.FirstName, &r.Language,
				&r.EmailEligible, &r.SmsEligible, &r.Email, &r.Phone); err != nil {
				return fmt.Errorf("audience recipients scan: %w", err)
			}
			out.Nodes = append(out.Nodes, &r)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	if len(out.Nodes) > first {
		out.PageInfo.HasNextPage = true
		out.Nodes = out.Nodes[:first]
	}
	if n := len(out.Nodes); n > 0 {
		cur := EncodeCursor(out.Nodes[n-1].CustomerID)
		out.PageInfo.EndCursor = &cur
	}
	return out, nil
}

// EncodeCursor returns the opaque cursor of a customer id: base64url without padding.
func EncodeCursor(customerID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(customerID))
}

// DecodeCursor returns the customer id inside a cursor, or a BAD_USER_INPUT error.
func DecodeCursor(cursor string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(raw) == 0 || !utf8.Valid(raw) || containsNUL(raw) {
		return "", BadInput("after is not a valid cursor")
	}
	return string(raw), nil
}

func containsNUL(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

// IsTimeout reports whether err is a Postgres statement timeout (SQLSTATE 57014).
func IsTimeout(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "57014"
}
