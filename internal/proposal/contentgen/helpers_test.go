package contentgen_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/anthropic"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

const sampleBrief = "Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun."

// demoPrices are the defaults of D-071: 0.04 per SMS segment, 0.001 per email.
var demoPrices = content.Prices{SMSSegment: 40_000, Email: 1_000}

func audience() flow.AudienceContent {
	return flow.AudienceContent{
		DSL: `city === "İstanbul"`, Count: 46, MaxFirstNameLength: 9,
		Coverage: flow.Coverage{Both: 20, EmailOnly: 13, SMSOnly: 13, Unreachable: 3},
		ByLanguage: []flow.LanguageAudience{
			{Language: "en", Count: 16, MaxFirstNameLength: 7},
			{Language: "tr", Count: 30, MaxFirstNameLength: 9},
		},
	}
}

func request(language, channel string, recipients int) flow.ContentRequest {
	return flow.ContentRequest{
		CampaignID: uuid.New(), StepID: uuid.New(), Language: language, Channel: channel, Brief: sampleBrief,
		ContextNotes: []string{"Mention the weekend."}, Audience: audience(),
		Plan:       flow.PlanContent{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "email reaches 33"},
		Recipients: recipients,
	}
}

func smsDoc(text string) string {
	b, _ := json.Marshal(map[string]any{"sms": map[string]any{"text": text}, "email": nil})
	return string(b)
}

func emailMap(subject string) map[string]any {
	return map[string]any{
		"subject": subject, "preheader": "%30 indirim sadece bu hafta", "headline": "Merhaba {{first_name}}!",
		"paragraphs": []string{"Sepetindeki ürünler seni bekliyor.", "%30 indirim fırsatını kaçırma."}, "cta_label": "Sepete dön",
	}
}

func emailDoc(subject string) string {
	b, _ := json.Marshal(map[string]any{"sms": nil, "email": emailMap(subject)})
	return string(b)
}

func newGenerator(l llm.LLM) *contentgen.Generator { return contentgen.New(l, demoPrices) }

// serverGenerator connects the real adapter to the scripted Messages server.
func serverGenerator(t *testing.T, srv *llmtest.MessagesServer) *contentgen.Generator {
	t.Helper()
	c, err := anthropic.New(anthropic.Config{APIKey: "test", BaseURL: srv.URL}, option.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	return newGenerator(c)
}

// sse is a streamed answer whose single text block is doc.
func sse(doc string) llmtest.Response {
	return llmtest.SSE(llmtest.Message{Blocks: []llmtest.Block{llmtest.Text(doc)}, Usage: llmtest.Usage{Input: 900, Output: 120}})
}

func decode(t *testing.T, raw []byte, into any) {
	t.Helper()
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, raw)
	}
}

func rep(s string, n int) string { return strings.Repeat(s, n) }
