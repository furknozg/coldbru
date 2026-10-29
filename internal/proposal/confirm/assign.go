package confirm

import (
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Assign returns the messages one person gets under the approved plan (D-010), in the order
// email, sms:
//
//	single(c)           one entry on c when the person is eligible on c, none otherwise;
//	primary_fallback(p) one entry on p when eligible on p, else one on the other channel when
//	                    eligible there, else none;
//	both                one entry per eligible channel.
//
// The contact is the email address for email and the phone number for sms. A channel whose
// contact is empty counts as not eligible: the batch contract needs a contact.
func Assign(mode llm.PlanMode, primary llm.Channel, r customer.Recipient) []flow.BatchRecipient {
	email := r.EmailEligible && r.Email != ""
	sms := r.SMSEligible && r.Phone != ""
	use := func(c llm.Channel) bool {
		if c == llm.ChannelEmail {
			return email
		}
		return sms
	}
	var channels []llm.Channel
	switch mode {
	case llm.ModeSingle:
		if use(primary) {
			channels = []llm.Channel{primary}
		}
	case llm.ModePrimaryFallback:
		other := llm.ChannelEmail
		if primary == llm.ChannelEmail {
			other = llm.ChannelSMS
		}
		switch {
		case use(primary):
			channels = []llm.Channel{primary}
		case use(other):
			channels = []llm.Channel{other}
		}
	case llm.ModeBoth:
		if email {
			channels = append(channels, llm.ChannelEmail)
		}
		if sms {
			channels = append(channels, llm.ChannelSMS)
		}
	}
	out := make([]flow.BatchRecipient, 0, len(channels))
	for _, c := range channels {
		contact := r.Phone
		if c == llm.ChannelEmail {
			contact = r.Email
		}
		out = append(out, flow.BatchRecipient{
			CustomerID: r.CustomerID, FirstName: r.FirstName, Language: r.Language, Timezone: r.Timezone, Channel: string(c), Contact: contact,
		})
	}
	return out
}

// Batches splits recipients into messages of at most flow.MaxBatchRecipients, in order. batch_no
// counts from 1 and every batch carries batch_count. No recipients means no batches.
func Batches(campaignID string, recipients []flow.BatchRecipient) []flow.RecipientBatch {
	n := (len(recipients) + flow.MaxBatchRecipients - 1) / flow.MaxBatchRecipients
	out := make([]flow.RecipientBatch, 0, n)
	for i := 0; i < len(recipients); i += flow.MaxBatchRecipients {
		end := min(i+flow.MaxBatchRecipients, len(recipients))
		out = append(out, flow.RecipientBatch{
			CampaignID: campaignID, BatchNo: len(out) + 1, BatchCount: n, Recipients: recipients[i:end:end],
		})
	}
	return out
}

// Count tallies recipients per channel; Total is the number of entries, so a person on two
// channels counts twice.
func Count(recipients []flow.BatchRecipient) flow.Recipients {
	var c flow.Recipients
	for _, r := range recipients {
		switch r.Channel {
		case string(llm.ChannelEmail):
			c.Email++
		case string(llm.ChannelSMS):
			c.SMS++
		}
	}
	c.Total = c.Email + c.SMS
	return c
}
