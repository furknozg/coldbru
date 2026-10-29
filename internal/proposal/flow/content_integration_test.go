//go:build integration

package flow_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

var contentPrices = content.Prices{SMSSegment: 40_000, Email: 1_000}

// realContent builds a harness whose content step runs the real ContentGenerator over the scripted
// LLM (D-071).
func realContent(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, 0, func(d *flow.Deps) { d.Content = contentgen.New(d.LLM, contentPrices) })
}

func doc(t *testing.T, channel string, blocks any) string {
	t.Helper()
	m := map[string]any{"sms": nil, "email": nil}
	m[channel] = blocks
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func emailBlocks(subject string) map[string]any {
	return map[string]any{
		"subject": subject, "preheader": "%30 indirim sadece bu hafta", "headline": "Merhaba {{first_name}}!",
		"paragraphs": []string{"Sepetindeki ürünler seni bekliyor.", "%30 indirim fırsatını kaçırma."}, "cta_label": "Sepete dön",
	}
}

func contentRec(outcome llm.Outcome, attempt int, errText string) llm.CallRecord {
	return rec(llm.PurposeContent, outcome, attempt, errText)
}

// sqlMaxName is the longest first name of the reachable sample audience, per language, written
// independently of the services.
func sqlMaxName(t *testing.T, h *harness, lang string) int {
	t.Helper()
	var n int
	err := env(t).Pool.QueryRow(t.Context(), `
		SELECT coalesce(max(char_length(c.first_name)), 0) FROM dwh.dim_customer c
		WHERE c.city = 'İstanbul' AND c.account_status = 'active' AND c.language = $1
		  AND ((c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced) OR (c.phone_e164 IS NOT NULL AND c.sms_opt_in))
		  AND EXISTS (SELECT 1 FROM dwh.fact_cart_item ci WHERE ci.customer_id = c.customer_id AND ci.added_at >= now() - interval '30 days')
		  AND NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND o.order_date >= now() - interval '30 days')`,
		lang).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// The sample brief with the real ContentGenerator (D-071): the audience carries the longest first
// names, every content slot carries encoding, segments and cost, a repair is recorded per attempt,
// Get sums the cost, and an over-long edit is refused.
func TestSampleBriefContentWithTheRealGenerator(t *testing.T) {
	h := realContent(t)
	prev := h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)

	// What the plan will ask for, in the order the flow generates it.
	aud, err := flow.NewAudienceContent(sampleDSL, nil, prev)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := flow.NewPlanContent(llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail}, aud)
	if err != nil {
		t.Fatal(err)
	}
	type slot struct {
		lang, channel string
		recipients    int
	}
	var slots []slot
	for _, l := range plan.ByLanguage {
		if l.Email > 0 {
			slots = append(slots, slot{l.Language, "email", l.Email})
		}
		if l.SMS > 0 {
			slots = append(slots, slot{l.Language, "sms", l.SMS})
		}
	}
	if len(slots) != 3 {
		t.Fatalf("slots = %v, want the three of the sample brief", slots)
	}

	// Script one answer per slot; the first slot gets an over-long answer first, then a repair.
	long := strings.Repeat("a", 170)
	smsText := map[string]string{
		"tr": "Merhaba {{first_name}}, sepetindeki ürünlerde %30 indirim seni bekliyor, hemen alışverişe dön!",
		"en": "Hi {{first_name}}, your cart is waiting with 30% off. Come back and finish your order today!",
	}
	for i, s := range slots {
		var good string
		if s.channel == "sms" {
			good = doc(t, "sms", map[string]any{"text": smsText[s.lang]})
		} else {
			good = doc(t, "email", emailBlocks("Sepetin seni bekliyor, {{first_name}}"))
		}
		if i == 0 {
			bad := doc(t, s.channel, map[string]any{"text": long})
			if s.channel == "email" {
				b := emailBlocks("Sepetin seni bekliyor, {{first_name}}")
				b["subject"] = long
				bad = doc(t, "email", b)
			}
			h.llm.StreamJSON(bad).WithRecords(llm.PurposeContent, contentRec(llm.OutcomeValidationFailed, 1, "too long"))
			h.llm.StreamJSON(good).WithRecords(llm.PurposeContent, contentRec(llm.OutcomeAccepted, 1, ""))
			continue
		}
		h.llm.StreamJSON(good).WithRecords(llm.PurposeContent, contentRec(llm.OutcomeAccepted, 1, ""))
	}

	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	var audience flow.AudienceContent
	decodeInto(t, h.step(camp.ID, flow.KindAudience).Variants[0].Content, &audience)

	// The audience carries the longest first names, equal to a direct SQL max (D-071).
	overall := 0
	for _, l := range audience.ByLanguage {
		if want := sqlMaxName(t, h, l.Language); l.MaxFirstNameLength != want || want == 0 {
			t.Errorf("language %s: max_first_name_length = %d, direct SQL max = %d", l.Language, l.MaxFirstNameLength, want)
		}
		overall = max(overall, l.MaxFirstNameLength)
	}
	if audience.MaxFirstNameLength != overall || overall == 0 {
		t.Errorf("audience max_first_name_length = %d, want the maximum of the languages (%d)", audience.MaxFirstNameLength, overall)
	}

	h.approveAll(camp.ID, flow.KindChannelPlan)
	cs := h.step(camp.ID, flow.KindContent)
	if len(cs.Variants) != 3 || cs.Failed {
		t.Fatalf("content step = %d variants, failed %v %q", len(cs.Variants), cs.Failed, cs.Reason)
	}

	// 1. Each slot carries its measurements, cost and the name length of its own language.
	var wantSMS, wantEmail content.Money
	for _, v := range cs.Variants {
		var recipients int
		for _, s := range slots {
			if s.lang == v.Language && s.channel == v.Channel {
				recipients = s.recipients
			}
		}
		maxName := audience.MaxFirstNameLengthOf(v.Language)
		switch v.Channel {
		case "sms":
			var c content.SMSContent
			decodeInto(t, v.Content, &c)
			info := content.MeasureSMS(content.ExpandMax(smsText[v.Language], maxName))
			cost := contentPrices.EstimateSMS(info.Segments, recipients)
			wantSMS += cost
			if c.Text != smsText[v.Language] || c.Encoding != info.Encoding || c.Segments != info.Segments || c.RenderedLength != info.Length || c.EstimatedCostUSD != cost.String() {
				t.Errorf("%s/sms = %+v, want %+v costing %s", v.Language, c, info, cost)
			}
			if v.Language == "tr" && (c.Encoding != content.UCS2 || c.Segments < 1) {
				t.Errorf("the Turkish sms must be UCS-2: %+v", c)
			}
		case "email":
			var c content.EmailContent
			decodeInto(t, v.Content, &c)
			cost := contentPrices.EstimateEmail(recipients)
			wantEmail += cost
			if c.EstimatedCostUSD != cost.String() || c.MaxRenderedLengths.Subject != len([]rune("Sepetin seni bekliyor, "))+maxName || len(c.Paragraphs) != 2 {
				t.Errorf("%s/email = %+v, want cost %s and subject length %d", v.Language, c, cost, len([]rune("Sepetin seni bekliyor, "))+maxName)
			}
		}
	}

	// 2. The first slot was repaired: two model calls, numbered 1 and 2, the accepted one names the variant.
	rows, err := h.q.ListLLMCalls(t.Context(), camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	requireTokenEvents(t, h, camp.ID)
	var contentRows []store.LlmCall
	for _, r := range rows {
		if r.Purpose == "content" {
			contentRows = append(contentRows, r)
		}
	}
	if len(contentRows) != 4 {
		t.Fatalf("%d content llm_call rows, want 4 (3 slots, one repaired)", len(contentRows))
	}
	repaired, accepted := 0, 0
	for _, r := range contentRows {
		switch {
		case r.Outcome == "validation_failed" && r.Attempt == 1 && r.VariantID == nil:
			repaired++
		case r.Outcome == "accepted" && r.VariantID == nil:
			t.Errorf("an accepted content row names no variant: %+v", r)
		case r.Outcome == "accepted":
			accepted++
		}
	}
	if repaired != 1 || accepted != 3 {
		t.Errorf("%d failed first attempts and %d accepted rows recorded, want 1 and 3", repaired, accepted)
	}
	calls := h.llm.StreamCalls()
	if len(calls) != 4 {
		t.Fatalf("%d model calls, want 4", len(calls))
	}
	if !strings.Contains(string(calls[1].UserJSON), `"repair":{"attempt":2`) || strings.Contains(string(calls[0].UserJSON), `"repair"`) {
		t.Errorf("repair data: first call %s, second call %s", calls[0].UserJSON, calls[1].UserJSON)
	}

	// 3. Get sums the live variants per channel and in total.
	cost := h.get(camp.ID).Cost
	if cost.SMSUSD != wantSMS.String() || cost.EmailUSD != wantEmail.String() || cost.TotalUSD != (wantSMS+wantEmail).String() {
		t.Errorf("cost = %+v, want sms %s email %s", cost, wantSMS, wantEmail)
	}
	if wantSMS == 0 || wantEmail == 0 {
		t.Errorf("sms %s / email %s: the sample brief has both channels", wantSMS, wantEmail)
	}

	// 4. Editing: an over-long SMS is refused and changes nothing; a valid one is recomputed.
	var sms flow.VariantView
	for _, v := range cs.Variants {
		if v.Channel == "sms" {
			sms = v
		}
	}
	_, err = h.svc.Edit(t.Context(), sms.ID, json.RawMessage(fmt.Sprintf(`{"text":%q}`, strings.Repeat("b", 161))))
	var ice *flow.InvalidContentError
	if !errors.Is(err, flow.ErrInvalidContent) || !errors.As(err, &ice) || len(ice.Problems) != 1 ||
		!strings.Contains(ice.Problems[0], "sms.text") || !strings.Contains(ice.Problems[0], "161") {
		t.Fatalf("over-long edit: err = %v", err)
	}
	if got := variantStatus(t, h, sms.ID); got != flow.VariantStatusDraft {
		t.Errorf("the refused edit changed the variant to %s", got)
	}
	// 160 characters in the expanded text: the placeholder counts as the longest name.
	maxName := audience.MaxFirstNameLengthOf(sms.Language)
	exact := strings.Repeat("c", 160-maxName) + "{{first_name}}"
	if _, err := h.svc.Edit(t.Context(), sms.ID, json.RawMessage(fmt.Sprintf(`{"text":%q}`, exact+"d"))); !errors.Is(err, flow.ErrInvalidContent) {
		t.Errorf("161 characters with the longest name: err = %v", err)
	}
	edited, err := h.svc.Edit(t.Context(), sms.ID, json.RawMessage(fmt.Sprintf(`{"text":%q}`, exact)))
	if err != nil {
		t.Fatalf("a 160-character edit was refused: %v", err)
	}
	var ec content.SMSContent
	decodeInto(t, edited.Content, &ec)
	if ec.RenderedLength != 160 || ec.Encoding != content.GSM7 || ec.Segments != 1 || ec.EstimatedCostUSD == "" {
		t.Errorf("edited content = %+v, want 160 characters, GSM-7, 1 segment, a recomputed cost", ec)
	}
	if got := variantStatus(t, h, sms.ID); got != flow.VariantStatusSuperseded {
		t.Errorf("the replaced variant is %s, want superseded", got)
	}
	var recipients int
	for _, s := range slots {
		if s.lang == sms.Language && s.channel == "sms" {
			recipients = s.recipients
		}
	}
	wantSMS = 0
	for _, v := range h.step(camp.ID, flow.KindContent).Variants {
		var c struct {
			Cost string `json:"estimated_cost_usd"`
		}
		decodeInto(t, v.Content, &c)
		if v.Channel == "sms" {
			m, _ := content.ParseMoney(c.Cost)
			wantSMS += m
		}
	}
	if got := h.get(camp.ID).Cost.SMSUSD; got != wantSMS.String() || ec.EstimatedCostUSD != contentPrices.EstimateSMS(1, recipients).String() {
		t.Errorf("sms cost after the edit = %s (variant %s), want %s", got, ec.EstimatedCostUSD, wantSMS)
	}
}
