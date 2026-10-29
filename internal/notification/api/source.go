package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/notification/store"
)

// ErrNotFound: the campaign is not known to Notification.
var ErrNotFound = errors.New("api: campaign not found")

// Source reads the live state of a campaign. *PoolSource implements it; tests use a fake.
type Source interface {
	// Summary returns the status and delivery counts of the campaign, or ErrNotFound.
	Summary(ctx context.Context, campaignID uuid.UUID) (DeliverySummary, error)
}

// PoolSource reads the state from notification_db: the campaign row and the per-channel counts,
// served by delivery_campaign_channel_status_idx (D-036, D-077).
type PoolSource struct {
	q *store.Queries
}

// NewPoolSource returns a Source over pool.
func NewPoolSource(pool *pgxpool.Pool) *PoolSource { return &PoolSource{q: store.New(pool)} }

// Summary reads the status first and the counts second. A campaign that completes in between shows
// a state that is older than its counts, which the next read corrects; the other order could show a
// completed campaign with counts that were read before its last delivery settled.
func (s *PoolSource) Summary(ctx context.Context, campaignID uuid.UUID) (DeliverySummary, error) {
	c, err := s.q.GetSendCampaign(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DeliverySummary{}, ErrNotFound
	}
	if err != nil {
		return DeliverySummary{}, fmt.Errorf("read campaign: %w", err)
	}
	rows, err := s.q.CountDeliveriesByStatus(ctx, campaignID)
	if err != nil {
		return DeliverySummary{}, fmt.Errorf("count deliveries: %w", err)
	}
	out := DeliverySummary{CampaignID: campaignID, Status: c.Status, Counts: make([]ChannelCount, 0, len(rows)), UpdatedAt: time.Now().UTC()}
	for _, r := range rows {
		out.Counts = append(out.Counts, ChannelCount{Channel: r.Channel, Status: r.Status, Count: r.Total})
		out.Total += r.Total
	}
	return out, nil
}
