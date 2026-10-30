package api

import (
	"bytes"
	"net/http"
)

// approve godoc
//
//	@Summary		Approve a variant
//	@Description	Approves a draft variant. When it was the last slot of its step the step is approved and the next one opens and starts generating; approving the schedule step confirms the campaign. Returns the updated campaign.
//	@Tags			variants
//	@Produce		json
//	@Param			id	path		string	true	"Variant id"	format(uuid)
//	@Success		200	{object}	flow.CampaignView
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem
//	@Failure		409	{object}	api.Problem	"The campaign is not a draft, the step is not open, or the variant is not a draft"
//	@Failure		500	{object}	api.Problem
//	@Failure		502	{object}	api.Problem	"The schedule was approved but confirming the campaign failed; the approval stands"
//	@Router			/variants/{id}/approve [post]
func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	camp, err := h.flow.CampaignOfVariant(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := h.flow.Approve(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.writeView(w, r, http.StatusOK, camp)
}

// reject godoc
//
//	@Summary		Reject a variant
//	@Description	Rejects a draft variant with a reason. Only its slot is regenerated, with the reason given to the model (202). Returns the updated campaign.
//	@Tags			variants
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Variant id"	format(uuid)
//	@Param			request	body		api.RejectRequest	true	"The reason"
//	@Success		202		{object}	flow.CampaignView
//	@Failure		400		{object}	api.Problem	"id is not a UUID, or malformed JSON"
//	@Failure		404		{object}	api.Problem
//	@Failure		409		{object}	api.Problem	"The campaign is not a draft, the step is not open, or the variant is not a draft"
//	@Failure		413		{object}	api.Problem
//	@Failure		415		{object}	api.Problem
//	@Failure		422		{object}	api.Problem	"Missing or too long reason"
//	@Failure		500		{object}	api.Problem
//	@Router			/variants/{id}/reject [post]
func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req RejectRequest
	if !decode(w, r, &req) {
		return
	}
	reason, ok := checkText(w, r, "reason", req.Reason, MaxReasonChars)
	if !ok {
		return
	}
	camp, err := h.flow.CampaignOfVariant(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := h.flow.Reject(r.Context(), id, reason); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.writeView(w, r, http.StatusAccepted, camp)
}

// edit godoc
//
//	@Summary		Edit a variant by hand
//	@Description	Replaces a draft variant with the edited content, which passes the validators generated content passes. The edit becomes a new draft variant in the same slot and the old one is replaced. Invalid content is a 422 whose problems list every broken rule.
//	@Tags			variants
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Variant id"	format(uuid)
//	@Param			request	body		api.EditRequest		true	"The new content"
//	@Success		200		{object}	flow.VariantView
//	@Failure		400		{object}	api.Problem	"id is not a UUID, or malformed JSON"
//	@Failure		404		{object}	api.Problem
//	@Failure		409		{object}	api.Problem	"The step is not open, or the variant is not a draft"
//	@Failure		413		{object}	api.Problem
//	@Failure		415		{object}	api.Problem
//	@Failure		422		{object}	api.Problem	"The content failed validation; problems lists why"
//	@Failure		500		{object}	api.Problem
//	@Router			/variants/{id}/edit [post]
func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req EditRequest
	if !decode(w, r, &req) {
		return
	}
	if len(req.Content) == 0 {
		fail(w, r, CodeInvalidRequest, "The request body does not match the expected shape.", "content: is required")
		return
	}
	v, err := h.flow.Edit(r.Context(), id, req.Content)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// regenerateStep godoc
//
//	@Summary		Regenerate an open step
//	@Description	Retries the generation of an open step, after a failure or a restart (202). Slots that have a draft or approved variant are left alone. Returns the campaign.
//	@Tags			steps
//	@Produce		json
//	@Param			id	path		string	true	"Step id"	format(uuid)
//	@Success		202	{object}	flow.CampaignView
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem
//	@Failure		409	{object}	api.Problem	"The step is not open, or the campaign is not a draft"
//	@Failure		500	{object}	api.Problem
//	@Router			/steps/{id}/regenerate [post]
func (h *Handler) regenerateStep(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	camp, err := h.flow.CampaignOfStep(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := h.flow.Regenerate(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.writeView(w, r, http.StatusAccepted, camp)
}

// answer godoc
//
//	@Summary		Answer a question of the agent
//	@Description	Records the marketer's answer to a clarifying question the agent asked (a chat message of kind `question`), and generates the step again with the question and the answer as context (202). Returns the updated campaign. A question takes one answer; the options of the question are suggestions, free text is accepted.
//	@Tags			messages
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Message id of the question"	format(uuid)
//	@Param			request	body		api.AnswerRequest	true	"The answer"
//	@Success		202		{object}	flow.CampaignView
//	@Failure		400		{object}	api.Problem	"id is not a UUID, or malformed JSON"
//	@Failure		404		{object}	api.Problem	"No such message"
//	@Failure		409		{object}	api.Problem	"message_not_question; question_already_answered; step_not_open: the step of the question is no longer open; campaign_not_drafting"
//	@Failure		413		{object}	api.Problem
//	@Failure		415		{object}	api.Problem
//	@Failure		422		{object}	api.Problem	"Missing or too long answer"
//	@Failure		500		{object}	api.Problem
//	@Router			/messages/{id}/answer [post]
func (h *Handler) answer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req AnswerRequest
	if !decode(w, r, &req) {
		return
	}
	answer, ok := checkText(w, r, "answer", req.Answer, MaxAnswerChars)
	if !ok {
		return
	}
	msg, err := h.flow.Answer(r.Context(), id, answer)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	h.writeView(w, r, http.StatusAccepted, msg.CampaignID)
}

// preview godoc
//
//	@Summary		Preview an email variant
//	@Description	Renders an email variant of the content step the way it is sent (D-090): the stored blocks, or, when the body carries `content`, unsaved edited blocks that pass the validators of the edit endpoint. `{{first_name}}` is replaced by a sample name of the variant's language ("Ayşe" for tr, "Alex" otherwise) and the button links to the campaign's CTA URL. Nothing is stored and no variant is created; the variant may be in any state.
//	@Tags			variants
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Variant id"	format(uuid)
//	@Param			request	body		api.PreviewRequest	false	"Unsaved edited content; omit the body or `content` to render the stored one"
//	@Success		200		{object}	flow.EmailPreview
//	@Failure		400		{object}	api.Problem	"id is not a UUID, or malformed JSON"
//	@Failure		404		{object}	api.Problem
//	@Failure		409		{object}	api.Problem	"variant_not_email: the variant is not an email of the content step"
//	@Failure		413		{object}	api.Problem
//	@Failure		415		{object}	api.Problem
//	@Failure		422		{object}	api.Problem	"The edited content failed validation; problems lists why"
//	@Failure		500		{object}	api.Problem
//	@Router			/variants/{id}/preview [post]
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req PreviewRequest
	if !decodeOptional(w, r, &req) {
		return
	}
	if string(bytes.TrimSpace(req.Content)) == "null" {
		req.Content = nil
	}
	p, err := h.flow.PreviewEmail(r.Context(), id, req.Content)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
