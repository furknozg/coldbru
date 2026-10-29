package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// Names of the two events that do not come from the flow.
const (
	// EventSnapshot carries the whole campaign view.
	EventSnapshot = "snapshot"
	// EventPing keeps an idle stream alive.
	EventPing = "ping"
)

// PingEvent is the data of a ping event.
type PingEvent struct {
	// At is the time the ping was sent.
	At time.Time `json:"at" validate:"required" format:"date-time"`
}

// EventCatalog documents the event stream of GET /campaigns/{id}/events: one property per event,
// named like the SSE event, holding the schema of that event's data line. It is never sent as a
// whole.
type EventCatalog struct {
	Snapshot          flow.CampaignView      `json:"snapshot" validate:"required"`
	ScreenResult      flow.ScreenResult      `json:"screen_result" validate:"required"`
	CampaignCancelled flow.CampaignCancelled `json:"campaign_cancelled" validate:"required"`
	CampaignConfirmed flow.CampaignConfirmed `json:"campaign_confirmed" validate:"required"`
	CampaignStatus    flow.CampaignStatus    `json:"campaign_status" validate:"required"`
	StepOpened        flow.StepOpened        `json:"step_opened" validate:"required"`
	StepApproved      flow.StepApproved      `json:"step_approved" validate:"required"`
	StepSuperseded    flow.StepSuperseded    `json:"step_superseded" validate:"required"`
	GenerationStarted flow.GenerationStarted `json:"generation_started" validate:"required"`
	GenerationFailed  flow.GenerationFailed  `json:"generation_failed" validate:"required"`
	VariantCreated    flow.VariantCreated    `json:"variant_created" validate:"required"`
	VariantApproved   flow.VariantApproved   `json:"variant_approved" validate:"required"`
	VariantRejected   flow.VariantRejected   `json:"variant_rejected" validate:"required"`
	VariantSuperseded flow.VariantSuperseded `json:"variant_superseded" validate:"required"`
	ContentDelta      flow.ContentDelta      `json:"content_delta" validate:"required"`
	ContentReset      flow.ContentReset      `json:"content_reset" validate:"required"`
	ContextAdded      flow.ContextAdded      `json:"context_added" validate:"required"`
	Ping              PingEvent              `json:"ping" validate:"required"`
}

// events godoc
//
//	@Summary		Live events of a campaign (Server-Sent Events)
//	@Description	Opens a text/event-stream. The first event is `snapshot`: the whole campaign view, the same JSON as GET /campaigns/{id}. After it come the events of the flow, each as `event: <name>` and `data: <JSON>`, in the order they happened; the data of every event is documented as a property of api.EventCatalog. A `ping` event is sent every 15 seconds.
//	@Description
//	@Description	The server subscribes before it takes the snapshot, so no event between the two is missed; an event that happened just before the snapshot may arrive again after it, so apply events idempotently (by variant id, step id and note id). There is no replay: a client that reconnects gets a new snapshot. When the server notices that it could not keep up with the events of a slow client it sends a fresh `snapshot` and carries on; always replace your state with a snapshot.
//	@Description
//	@Description	Events of the content step: `content_delta` carries a fragment of the text being written and its `attempt`; `content_reset` announces that attempt 2 or 3 starts because the previous answer broke a rule, and the preview of that slot must be cleared. The variant is final only with `variant_created`.
//	@Description
//	@Description	Lifecycle events: `campaign_confirmed` (the schedule was confirmed and the campaign is `scheduled`; carries the recipient counts per channel and the number of recipient batches), `campaign_cancelled` (the campaign was cancelled by a screening block or by the marketer) and `campaign_status` (Notification reported progress: the campaign is now `sending`, when the first send time arrived, or `completed`, when every message is final; D-080).
//	@Description
//	@Description	The response is not JSON; the schema below only documents the data of each event.
//	@Tags			events
//	@Produce		text/event-stream
//	@Param			id	path		string	true	"Campaign id"	format(uuid)
//	@Success		200	{object}	api.EventCatalog	"Stream of events; each event's data is the property of the same name"
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem	"No such campaign (answered before the stream starts)"
//	@Failure		500	{object}	api.Problem
//	@Router			/campaigns/{id}/events [get]
func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// Subscribe first: an event published between the subscription and the snapshot is delivered
	// (and may be a duplicate of what the snapshot already shows), none is lost.
	sub := h.flow.Hub().Subscribe(id)
	defer sub.Close()
	// Events dropped from here on may have happened after the snapshot is read, or before it is
	// sent: counting them from now makes the next event trigger a fresh snapshot in either case.
	seenDrops := sub.Dropped()

	view, err := h.flow.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	rc := http.NewResponseController(w)
	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-cache")
	hd.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	send := func(event string, data any) error {
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
			return err
		}
		return rc.Flush()
	}
	if err := send(EventSnapshot, view); err != nil {
		h.streamEnded(r, id, err)
		return
	}

	tick, stop := h.ticker(h.pingEvery)
	defer stop()
	// resnapshot sends a fresh snapshot when the subscription dropped events since the last one.
	resnapshot := func() error {
		if d := sub.Dropped(); d != seenDrops {
			seenDrops = d
			v, err := h.flow.Get(r.Context(), id)
			if err != nil {
				return err
			}
			return send(EventSnapshot, v)
		}
		return nil
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-h.shutdown:
			return
		case ev, ok := <-sub.C:
			if !ok {
				return
			}
			if err := resnapshot(); err != nil {
				h.streamEnded(r, id, err)
				return
			}
			if err := send(ev.Type(), ev); err != nil {
				h.streamEnded(r, id, err)
				return
			}
		case <-tick:
			if err := resnapshot(); err != nil {
				h.streamEnded(r, id, err)
				return
			}
			if err := send(EventPing, PingEvent{At: time.Now().UTC()}); err != nil {
				h.streamEnded(r, id, err)
				return
			}
		}
	}
}

// streamEnded logs why a stream stopped. A client that went away is routine; anything else is not.
func (h *Handler) streamEnded(r *http.Request, id uuid.UUID, err error) {
	if r.Context().Err() != nil {
		h.log.Debug("api: event stream closed by the client", "campaign", id, "error", err)
		return
	}
	h.log.Warn("api: event stream failed", "campaign", id, "error", err)
}
