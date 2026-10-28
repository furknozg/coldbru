package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// createCampaign godoc
//
//	@Summary		Create a campaign
//	@Description	Stores the brief as a new campaign and returns at once (202). The brief is screened and the audience proposed in the background; follow the progress with GET /campaigns/{id}/events. A blocked brief cancels the campaign.
//	@Tags			campaigns
//	@Accept			json
//	@Produce		json
//	@Param			request	body		api.CreateCampaignRequest	true	"The brief"
//	@Success		202		{object}	flow.CampaignInfo
//	@Header			202		{string}	Location	"Path of the new campaign: /campaigns/{id}"
//	@Failure		400		{object}	api.Problem	"Malformed JSON"
//	@Failure		413		{object}	api.Problem	"Body larger than 64 KiB"
//	@Failure		415		{object}	api.Problem	"Not application/json"
//	@Failure		422		{object}	api.Problem	"Empty or too long brief, unknown field"
//	@Failure		500		{object}	api.Problem
//	@Router			/campaigns [post]
func (h *Handler) createCampaign(w http.ResponseWriter, r *http.Request) {
	var req CreateCampaignRequest
	if !decode(w, r, &req) {
		return
	}
	brief, ok := checkText(w, r, "brief", req.Brief, MaxBriefChars)
	if !ok {
		return
	}
	info, err := h.flow.CreateCampaign(r.Context(), brief)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	w.Header().Set("Location", "/campaigns/"+url.PathEscape(info.ID.String()))
	writeJSON(w, http.StatusAccepted, info)
}

// listCampaigns godoc
//
//	@Summary		List campaigns
//	@Description	Returns one page of campaigns, newest first.
//	@Tags			campaigns
//	@Produce		json
//	@Param			limit	query		int	false	"Page size, 1 to 100"	default(20)	minimum(1)	maximum(100)
//	@Param			offset	query		int	false	"Campaigns to skip"		default(0)	minimum(0)	maximum(100000)
//	@Success		200		{object}	api.CampaignList
//	@Failure		422		{object}	api.Problem	"limit or offset out of range"
//	@Failure		500		{object}	api.Problem
//	@Router			/campaigns [get]
func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset := DefaultListLimit, 0
	var problems []string
	if s := q.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err != nil || n < 1 || n > MaxListLimit {
			problems = append(problems, "limit: must be a whole number from 1 to "+strconv.Itoa(MaxListLimit))
		} else {
			limit = n
		}
	}
	if s := q.Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err != nil || n < 0 || n > MaxListOffset {
			problems = append(problems, "offset: must be a whole number from 0 to "+strconv.Itoa(MaxListOffset))
		} else {
			offset = n
		}
	}
	if len(problems) > 0 {
		fail(w, r, CodeInvalidRequest, "The query parameters are out of range.", problems...)
		return
	}
	items, err := h.flow.ListCampaigns(r.Context(), limit, offset)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if items == nil {
		items = []flow.CampaignInfo{} // an empty page is [], never null
	}
	writeJSON(w, http.StatusOK, CampaignList{Items: items, Limit: limit, Offset: offset})
}

// getCampaign godoc
//
//	@Summary		Get a campaign
//	@Description	Returns the campaign with its current steps and their live variants, the context notes and the estimated cost.
//	@Tags			campaigns
//	@Produce		json
//	@Param			id	path		string	true	"Campaign id"	format(uuid)
//	@Success		200	{object}	flow.CampaignView
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem
//	@Failure		500	{object}	api.Problem
//	@Router			/campaigns/{id} [get]
func (h *Handler) getCampaign(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	view, err := h.flow.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// addContext godoc
//
//	@Summary		Add context to the open step
//	@Description	Records a note on the open step. Its draft and rejected variants are replaced and regenerated with the note (202); approved variants stay.
//	@Tags			campaigns
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Campaign id"	format(uuid)
//	@Param			request	body		api.ContextRequest	true	"The note"
//	@Success		202		{object}	flow.NoteView
//	@Failure		400		{object}	api.Problem	"id is not a UUID, or malformed JSON"
//	@Failure		404		{object}	api.Problem
//	@Failure		409		{object}	api.Problem	"The campaign is not a draft, or has no open step"
//	@Failure		413		{object}	api.Problem
//	@Failure		415		{object}	api.Problem
//	@Failure		422		{object}	api.Problem
//	@Failure		500		{object}	api.Problem
//	@Router			/campaigns/{id}/context [post]
func (h *Handler) addContext(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req ContextRequest
	if !decode(w, r, &req) {
		return
	}
	body, ok := checkText(w, r, "body", req.Body, MaxNoteChars)
	if !ok {
		return
	}
	note, err := h.flow.AddContext(r.Context(), id, body)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusAccepted, note)
}

// reopen godoc
//
//	@Summary		Reopen an approved step
//	@Description	Revises an approved step: it and every later step are replaced, a new version of the step opens and its generation starts (202).
//	@Tags			campaigns
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Campaign id"	format(uuid)
//	@Param			request	body		api.ReopenRequest	true	"The step to reopen"
//	@Success		202		{object}	flow.StepInfo
//	@Failure		400		{object}	api.Problem	"id is not a UUID, or malformed JSON"
//	@Failure		404		{object}	api.Problem	"No such campaign, or it has no step of that kind"
//	@Failure		409		{object}	api.Problem	"The campaign is not a draft, or the step is not approved"
//	@Failure		413		{object}	api.Problem
//	@Failure		415		{object}	api.Problem
//	@Failure		422		{object}	api.Problem	"Unknown step kind"
//	@Failure		500		{object}	api.Problem
//	@Router			/campaigns/{id}/reopen [post]
func (h *Handler) reopen(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req ReopenRequest
	if !decode(w, r, &req) {
		return
	}
	step, err := h.flow.Reopen(r.Context(), id, req.Kind)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusAccepted, step)
}

// regenerateCampaign godoc
//
//	@Summary		Restart a campaign that has no steps
//	@Description	Restarts the screening and the audience proposal of a campaign whose process stopped before the audience step opened (202). Does nothing while the pipeline is running.
//	@Tags			campaigns
//	@Produce		json
//	@Param			id	path		string	true	"Campaign id"	format(uuid)
//	@Success		202	{object}	flow.CampaignView
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem
//	@Failure		409	{object}	api.Problem	"The campaign is not a draft, or already has steps"
//	@Failure		500	{object}	api.Problem
//	@Router			/campaigns/{id}/regenerate [post]
func (h *Handler) regenerateCampaign(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.flow.RegenerateCampaign(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.writeView(w, r, http.StatusAccepted, id)
}

// confirm godoc
//
//	@Summary		Confirm a campaign
//	@Description	Retries the confirmation of a campaign whose schedule step is approved but which is still a draft, for example after the automatic confirmation failed (a Customer outage). It fetches the recipients, assigns each person their channel(s) under the approved plan and, in one transaction, sets the campaign to `scheduled` and queues the confirmed event and the recipient batches for NATS. Returns the updated campaign.
//	@Tags			campaigns
//	@Produce		json
//	@Param			id	path		string	true	"Campaign id"	format(uuid)
//	@Success		200	{object}	flow.CampaignView
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem
//	@Failure		409	{object}	api.Problem	"campaign_not_drafting: already scheduled, cancelled or sent; schedule_not_approved; no_recipients: the plan reaches nobody now"
//	@Failure		500	{object}	api.Problem
//	@Failure		502	{object}	api.Problem	"confirm_failed: Customer or the database failed; nothing changed, try again"
//	@Router			/campaigns/{id}/confirm [post]
func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.flow.Confirm(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.writeView(w, r, http.StatusOK, id)
}

// cancel godoc
//
//	@Summary		Cancel a campaign
//	@Description	Cancels a campaign that is a draft, scheduled or sending. A draft only changes status; a scheduled or sending campaign also queues the cancelled event so that Notification stops sending. The body, and the reason in it, are optional. Returns the updated campaign.
//	@Tags			campaigns
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Campaign id"	format(uuid)
//	@Param			request	body		api.CancelRequest	false	"The reason"
//	@Success		200		{object}	flow.CampaignView
//	@Failure		400		{object}	api.Problem	"id is not a UUID, or malformed JSON"
//	@Failure		404		{object}	api.Problem
//	@Failure		409		{object}	api.Problem	"campaign_not_cancellable: the campaign is completed or already cancelled"
//	@Failure		413		{object}	api.Problem
//	@Failure		415		{object}	api.Problem
//	@Failure		422		{object}	api.Problem	"Reason longer than 1000 characters"
//	@Failure		500		{object}	api.Problem
//	@Router			/campaigns/{id}/cancel [post]
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req CancelRequest
	if !decodeOptional(w, r, &req) {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if n := utf8.RuneCountInString(reason); n > MaxReasonChars {
		fail(w, r, CodeInvalidRequest, "The request body does not match the expected shape.",
			fmt.Sprintf("reason: must be at most %d characters (got %d)", MaxReasonChars, n))
		return
	}
	if err := h.flow.Cancel(r.Context(), id, reason); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.writeView(w, r, http.StatusOK, id)
}

// writeView answers with the campaign view of id.
func (h *Handler) writeView(w http.ResponseWriter, r *http.Request, status int, id uuid.UUID) {
	view, err := h.flow.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, status, view)
}
