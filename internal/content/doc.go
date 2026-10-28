// Package content is the shared rulebook of campaign messages (D-028, D-071). Proposal uses it to
// validate and measure what the model writes and what the marketer edits; Notification uses it at
// send time to expand and render the same stored content for one recipient.
//
// The model writes a small set of blocks, never markup:
//
//   - SMS is one text. Its length is measured in Unicode characters with {{first_name}} expanded to
//     the longest first name of the audience; its encoding (GSM-7 or UCS-2) and segment count follow
//     from the characters (CheckSMS, MeasureSMS).
//   - An email is subject, preheader, headline, one to four paragraphs and a call-to-action label
//     (CheckEmail). RenderEmail puts the blocks into one fixed table-based HTML template with
//     html/template escaping and builds the plain-text alternative from the same blocks.
//
// Only the variable {{first_name}} exists. Text with any other {{...}}, with a URL or with an HTML
// tag is rejected, and expansion is our own code: model text never goes through a template engine.
// Every rule violation is a Problem (field, rule, got, limit) that can be shown to the marketer or
// fed back to the model.
package content
