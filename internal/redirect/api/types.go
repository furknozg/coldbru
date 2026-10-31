package api

import "github.com/google/uuid"

// MaxLinksPerRequest bounds one POST /links; Notification sends one recipient batch (500) at a time.
const MaxLinksPerRequest = 1000

// Link request limits.
const (
	maxTimezoneLen  = 64
	maxTargetURLLen = 2048
)

// LinkRequest asks for the short link of one delivery.
type LinkRequest struct {
	// DeliveryID is Notification's delivery id and the idempotency key: asking again for the same
	// delivery returns the same code.
	DeliveryID uuid.UUID `json:"delivery_id" validate:"required" format:"uuid"`
	CampaignID uuid.UUID `json:"campaign_id" validate:"required" format:"uuid"`
	Channel    string    `json:"channel" validate:"required" enums:"sms,email"`
	Language   string    `json:"language" validate:"required" enums:"tr,en"`
	Timezone   string    `json:"timezone" validate:"required" example:"Europe/Istanbul"`
	// TargetURL is where a click is sent: the campaign's call-to-action link with utm_campaign. An
	// absolute http or https URL without credentials.
	TargetURL string `json:"target_url" validate:"required" example:"https://shop.example.com/?utm_campaign=019a0e6e"`
}

// CreateLinksRequest is the body of POST /links.
type CreateLinksRequest struct {
	Links []LinkRequest `json:"links" validate:"required"`
}

// LinkResult is the code of one delivery.
type LinkResult struct {
	DeliveryID uuid.UUID `json:"delivery_id" validate:"required" format:"uuid"`
	// Code is 10 base62 characters; the link is <TRACKING_BASE_URL>/r/<code>.
	Code string `json:"code" validate:"required" example:"k3JdQ9aB7x"`
}

// CreateLinksResponse is the body of the answer to POST /links: one result per requested link, in
// the order of the request.
type CreateLinksResponse struct {
	Links []LinkResult `json:"links" validate:"required"`
}
