package api

import "time"

// Event Redirect publishes (D-035, D-107): every click on a short link. Analytics mirrors this
// shape in internal/analytics/intake; a contract test there fails when the two drift apart.

// Event type (envelope "type", subject evt.redirect.clicked.v1), producer name and version.
const (
	EventProducer     = "redirect"
	EventTypeClicked  = "redirect.clicked"
	EventClickVersion = 1
)

// ClickedData is the data of evt.redirect.clicked.v1. It is written in the transaction that
// inserts the click row. ClickID is that row's id (UUIDv7); First is true for the first click of
// the delivery, so counting First events gives the unique clickers.
type ClickedData struct {
	ClickID    string    `json:"click_id"`
	DeliveryID string    `json:"delivery_id"`
	CampaignID string    `json:"campaign_id"`
	Channel    string    `json:"channel"`
	Language   string    `json:"language"`
	Timezone   string    `json:"timezone"`
	ClickedAt  time.Time `json:"clicked_at"`
	First      bool      `json:"first"`
}
