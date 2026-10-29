// Package intake is how Notification takes in what Proposal publishes (D-078): the confirmed and
// cancelled events, the recipient batches and the schedule firings. Three consumers run it:
//
//	EVENTS      evt.proposal.>   envelopes, processed_event dedup (Events)
//	RECIPIENTS  recipients.>     raw batches, dedup by recipient_batch's primary key (Recipients)
//	SCHEDULES   fire.>           raw firings, dedup by the schedule's status (Schedules)
//
// Each handler runs in one database transaction with the platform consumer (internal/platform/consumer)
// and takes the campaign's row lock first, so a batch, a firing and a cancellation of one campaign
// never overlap. The wire shapes are mirrored in wire.go, not imported from Proposal, so the two
// services stay separate; a test compares the mirror with Proposal's types.
package intake
