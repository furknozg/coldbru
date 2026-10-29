package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// events godoc
//
//	@Summary		Live delivery counts of a campaign (Server-Sent Events)
//	@Description	Opens a text/event-stream. The first event is `snapshot`: the same JSON as GET /campaigns/{id}/deliveries. The server then reads the counts every second and sends a new `snapshot` only when the status or a count changed. A `ping` event is sent every 15 seconds.
//	@Description
//	@Description	When the campaign is `completed` or `cancelled`, the server sends the final `snapshot` followed by `done` ({"status"}) and closes the stream; a campaign that is already finished when the client connects gets its snapshot and `done` at once. There is no replay: a client that reconnects gets a new snapshot. A snapshot replaces the client's state.
//	@Description
//	@Description	The response is not JSON; the schema below only documents the data of each event.
//	@Tags			deliveries
//	@Produce		text/event-stream
//	@Param			id	path		string	true	"Campaign id"	format(uuid)
//	@Success		200	{object}	api.EventCatalog	"Stream of events; each event's data is the property of the same name"
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem	"No such campaign (answered before the stream starts)"
//	@Failure		500	{object}	api.Problem
//	@Router			/campaigns/{id}/deliveries/events [get]
func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	last, err := h.src.Summary(r.Context(), id)
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
	// sendState sends a snapshot and, when the campaign is over, done. It reports whether the stream
	// is finished.
	sendState := func(s DeliverySummary) (bool, error) {
		if err := send(EventSnapshot, s); err != nil {
			return false, err
		}
		if !terminal(s.Status) {
			return false, nil
		}
		return true, send(EventDone, DoneEvent{Status: s.Status})
	}

	if end, err := sendState(last); err != nil || end {
		if err != nil {
			h.streamEnded(r, id, err)
		}
		return
	}

	poll, stopPoll := h.ticker(h.pollEvery)
	defer stopPoll()
	ping, stopPing := h.ticker(h.pingEvery)
	defer stopPing()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-h.shutdown:
			return
		case <-poll:
			s, err := h.src.Summary(r.Context(), id)
			if err != nil {
				// A failed read is retried at the next tick; the client keeps its last snapshot.
				if r.Context().Err() == nil {
					h.log.Warn("api: delivery poll failed", "campaign", id, "error", err)
				}
				continue
			}
			if sameState(last, s) {
				continue
			}
			last = s
			end, err := sendState(s)
			if err != nil {
				h.streamEnded(r, id, err)
				return
			}
			if end {
				return
			}
		case <-ping:
			if err := send(EventPing, PingEvent{At: h.now().UTC()}); err != nil {
				h.streamEnded(r, id, err)
				return
			}
		}
	}
}

// streamEnded logs why a stream stopped. A client that went away is routine; anything else is not.
func (h *Handler) streamEnded(r *http.Request, id uuid.UUID, err error) {
	if r.Context().Err() != nil {
		h.log.Debug("api: delivery stream closed by the client", "campaign", id, "error", err)
		return
	}
	h.log.Warn("api: delivery stream failed", "campaign", id, "error", err)
}
