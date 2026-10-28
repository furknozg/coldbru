// Package confirm is Proposal's Confirmer (D-070, D-074): when the schedule step of a campaign is
// approved it fetches the recipients from Customer, assigns each person the channel(s) of the
// approved channel plan (D-010), and, in one transaction, sets the campaign `scheduled` and writes
// the evt.proposal.confirmed.v1 event and the RECIPIENTS batches to the outbox. The outbox Relay
// publishes them (D-006, D-054). The recipient list is never stored in proposal_db; it exists only
// in the payloads of the outbox rows.
package confirm
