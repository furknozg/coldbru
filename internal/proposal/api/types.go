package api

import (
	"encoding/json"

	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// CreateCampaignRequest is the body of POST /campaigns.
type CreateCampaignRequest struct {
	// Brief is what the marketer wants, in their own words. At most 2000 characters.
	Brief string `json:"brief" example:"Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun." minLength:"1" maxLength:"2000" validate:"required"`
}

// CampaignList is the response of GET /campaigns: one page, newest campaign first.
type CampaignList struct {
	Items []flow.CampaignInfo `json:"items" validate:"required"`
	// Limit and Offset echo the page that was asked for. A page with fewer than Limit items is the last.
	Limit  int `json:"limit" example:"20" validate:"required"`
	Offset int `json:"offset" example:"0" validate:"required"`
}

// RejectRequest is the body of POST /variants/{id}/reject.
type RejectRequest struct {
	// Reason tells the model what was wrong; it is used when the slot is regenerated. At most 1000 characters.
	Reason string `json:"reason" example:"Çok resmi, daha samimi olsun." minLength:"1" maxLength:"1000" validate:"required"`
}

// EditRequest is the body of POST /variants/{id}/edit.
type EditRequest struct {
	// Content is the new content of the variant, in the shape of its step: {"dsl"} for the
	// audience; {"mode","primary","rationale"} for the channel plan; for the content step {"text"}
	// of an SMS or {"subject","preheader","headline","paragraphs","cta_label"} of an email;
	// {"send_at","rationale"} for the schedule.
	Content json.RawMessage `json:"content" swaggertype:"object" validate:"required"`
}

// ContextRequest is the body of POST /campaigns/{id}/context.
type ContextRequest struct {
	// Body is the note the marketer adds to the open step. At most 2000 characters.
	Body string `json:"body" example:"Hafta sonu kampanyası olduğunu belirt." minLength:"1" maxLength:"2000" validate:"required"`
}

// ReopenRequest is the body of POST /campaigns/{id}/reopen.
type ReopenRequest struct {
	// Kind is the approved step to revise; it and every later step are replaced.
	Kind string `json:"kind" enums:"audience,channel_plan,content,schedule" example:"channel_plan" validate:"required"`
}

// CancelRequest is the optional body of POST /campaigns/{id}/cancel.
type CancelRequest struct {
	// Reason is why the campaign is cancelled; it is recorded and sent to Notification. At most 1000 characters. Without one, "cancelled by the marketer" is used.
	Reason string `json:"reason,omitempty" example:"Wrong discount in the text." maxLength:"1000"`
}
