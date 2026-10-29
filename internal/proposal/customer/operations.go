package customer

import (
	"context"
	"errors"
	"fmt"

	"github.com/furknozg/coldbru/internal/proposal/compile"
)

// The two operations of Proposal and their fixed selections (D-041, docs/coldbru-gqdsl.md §7). The
// text is validated against the Customer schema by a test, so schema drift fails the build.
const (
	previewOperation = "AudiencePreview"
	previewQuery     = `query AudiencePreview($filter: CustomerFilter!) {
  audiencePreview(filter: $filter) {
    count
    maxFirstNameLength
    coverage { both emailOnly smsOnly unreachable }
    byLanguage { language count maxFirstNameLength coverage { both emailOnly smsOnly unreachable } }
    byLocale { language timezone count maxFirstNameLength coverage { both emailOnly smsOnly unreachable } }
  }
}`

	recipientsOperation = "AudienceRecipients"
	recipientsQuery     = `query AudienceRecipients($filter: CustomerFilter!, $first: Int!, $after: String) {
  audienceRecipients(filter: $filter, first: $first, after: $after) {
    nodes { customerId firstName language timezone emailEligible smsEligible email phone }
    pageInfo { hasNextPage endCursor }
  }
}`
)

// MaxPageSize is the largest page Customer returns (D-058).
const MaxPageSize = 1000

// Coverage counts customers by the channels they can be reached on (D-010, D-058).
type Coverage struct {
	Both        int `json:"both"`
	EmailOnly   int `json:"emailOnly"`
	SmsOnly     int `json:"smsOnly"`
	Unreachable int `json:"unreachable"`
}

// LanguageCoverage is the audience figures of one language (D-009).
type LanguageCoverage struct {
	Language string   `json:"language"`
	Count    int      `json:"count"`
	Coverage Coverage `json:"coverage"`
	// MaxFirstNameLength is the longest first name, in Unicode characters, among the reachable
	// customers of this language; 0 when there are none (D-071).
	MaxFirstNameLength int `json:"maxFirstNameLength"`
}

// LocaleCoverage is the audience figures of one (language, timezone) pair; schedule slots follow
// these pairs (D-076). Pairs whose customers are all unreachable are listed with Count 0.
type LocaleCoverage struct {
	Language string `json:"language"`
	// Timezone is the IANA time zone of the customers, for example Europe/Istanbul.
	Timezone string   `json:"timezone"`
	Count    int      `json:"count"`
	Coverage Coverage `json:"coverage"`
	// MaxFirstNameLength is the longest first name, in Unicode characters, among the reachable
	// customers of this pair; 0 when there are none.
	MaxFirstNameLength int `json:"maxFirstNameLength"`
}

// Preview is the size and channel reach of an audience. Count includes only customers reachable on
// at least one channel; Coverage.Unreachable matched the filter but are not in Count (D-058).
type Preview struct {
	Count      int                `json:"count"`
	Coverage   Coverage           `json:"coverage"`
	ByLanguage []LanguageCoverage `json:"byLanguage"`
	// ByLocale splits the same figures by language and time zone, ordered by language then time
	// zone (D-076).
	ByLocale []LocaleCoverage `json:"byLocale"`
	// MaxFirstNameLength is the longest first name, in Unicode characters, among the reachable
	// customers counted in Count; 0 when there are none (D-071).
	MaxFirstNameLength int `json:"maxFirstNameLength"`
}

// Recipient is one reachable customer. Email is set only when EmailEligible and Phone only when
// SMSEligible (D-058).
type Recipient struct {
	CustomerID string `json:"customerId"`
	FirstName  string `json:"firstName"`
	Language   string `json:"language"`
	// Timezone is the IANA time zone of the customer (D-076).
	Timezone      string `json:"timezone"`
	EmailEligible bool   `json:"emailEligible"`
	SMSEligible   bool   `json:"smsEligible"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
}

// RecipientPage is one page of recipients ordered by customer id.
type RecipientPage struct {
	Nodes       []Recipient `json:"nodes"`
	HasNextPage bool        `json:"hasNextPage"`
	// EndCursor is the value for the next call's after; empty when there is none.
	EndCursor string `json:"endCursor"`
}

// Preview returns the size and channel reach of the audience f selects.
func (c *Client) Preview(ctx context.Context, f *compile.Filter) (*Preview, error) {
	var data struct {
		AudiencePreview Preview `json:"audiencePreview"`
	}
	if err := c.execute(ctx, f, previewQuery, previewOperation, map[string]any{}, &data); err != nil {
		return nil, err
	}
	return &data.AudiencePreview, nil
}

// Recipients returns one page of up to first recipients of the audience f selects, after the
// cursor (empty for the first page). first must be 1 to MaxPageSize.
func (c *Client) Recipients(ctx context.Context, f *compile.Filter, first int, after string) (*RecipientPage, error) {
	if first < 1 || first > MaxPageSize {
		return nil, fmt.Errorf("customer: page size %d out of range 1..%d", first, MaxPageSize)
	}
	vars := map[string]any{"first": first, "after": nil}
	if after != "" {
		vars["after"] = after
	}
	var data struct {
		AudienceRecipients struct {
			Nodes    []Recipient `json:"nodes"`
			PageInfo struct {
				HasNextPage bool    `json:"hasNextPage"`
				EndCursor   *string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"audienceRecipients"`
	}
	if err := c.execute(ctx, f, recipientsQuery, recipientsOperation, vars, &data); err != nil {
		return nil, err
	}
	page := &RecipientPage{Nodes: data.AudienceRecipients.Nodes, HasNextPage: data.AudienceRecipients.PageInfo.HasNextPage}
	if cur := data.AudienceRecipients.PageInfo.EndCursor; cur != nil {
		page.EndCursor = *cur
	}
	return page, nil
}

// EachRecipientPage calls fn with every page of the audience, MaxPageSize recipients at a time,
// until the last page or until fn or a request fails. The pages are not a snapshot: Customer
// evaluates the filter on every call.
func (c *Client) EachRecipientPage(ctx context.Context, f *compile.Filter, fn func(page []Recipient) error) error {
	after := ""
	for {
		page, err := c.Recipients(ctx, f, MaxPageSize, after)
		if err != nil {
			return err
		}
		if len(page.Nodes) > 0 {
			if err := fn(page.Nodes); err != nil {
				return err
			}
		}
		if !page.HasNextPage {
			return nil
		}
		if page.EndCursor == "" || page.EndCursor == after {
			return errors.New("customer: next page reported without a new cursor")
		}
		after = page.EndCursor
	}
}

// AllRecipients collects every recipient of the audience in customer id order.
func (c *Client) AllRecipients(ctx context.Context, f *compile.Filter) ([]Recipient, error) {
	var all []Recipient
	err := c.EachRecipientPage(ctx, f, func(page []Recipient) error {
		all = append(all, page...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}
