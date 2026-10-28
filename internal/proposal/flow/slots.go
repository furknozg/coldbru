package flow

import (
	"sort"

	"github.com/furknozg/coldbru/internal/proposal/store"
)

func sortSlots(s []Slot) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Language != s[j].Language {
			return s[i].Language < s[j].Language
		}
		return s[i].Channel < s[j].Channel
	})
}

// slotOf returns the slot a variant occupies.
func slotOf(v store.ProposalVariant) Slot {
	return Slot{Language: v.Language.String, Channel: v.Channel.String}
}

// filled is the set of slots that hold a draft or approved variant.
func filled(live []store.ProposalVariant) map[Slot]bool {
	m := map[Slot]bool{}
	for _, v := range live {
		if v.Status == VariantStatusDraft || v.Status == VariantStatusApproved {
			m[slotOf(v)] = true
		}
	}
	return m
}

// missingSlots are the expected slots without a draft or approved variant, in expected order.
func missingSlots(expected []Slot, live []store.ProposalVariant) []Slot {
	return missingFrom(expected, filled(live))
}

// missingFrom are the expected slots not in have.
func missingFrom(expected []Slot, have map[Slot]bool) []Slot {
	var out []Slot
	for _, s := range expected {
		if !have[s] {
			out = append(out, s)
		}
	}
	return out
}

// stepComplete reports whether every expected slot is approved and no variant of the step is
// still a draft or rejected: the condition under which the step is approved (D-009).
func stepComplete(expected []Slot, live []store.ProposalVariant) bool {
	approved := map[Slot]bool{}
	for _, v := range live {
		switch v.Status {
		case VariantStatusApproved:
			approved[slotOf(v)] = true
		case VariantStatusDraft, VariantStatusRejected:
			return false
		}
	}
	for _, s := range expected {
		if !approved[s] {
			return false
		}
	}
	return len(expected) > 0
}
