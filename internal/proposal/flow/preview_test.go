package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/flow"
)

func TestSampleFirstName(t *testing.T) {
	t.Parallel()
	tests := map[string]string{"tr": flow.SampleFirstNameTR, "en": flow.SampleFirstNameEN, "de": flow.SampleFirstNameEN, "": flow.SampleFirstNameEN}
	for lang, want := range tests {
		if got := flow.SampleFirstName(lang); got != want {
			t.Errorf("SampleFirstName(%q) = %q, want %q", lang, got, want)
		}
	}
	if flow.SampleFirstNameTR != "Ayşe" || flow.SampleFirstNameEN != "Alex" {
		t.Errorf("sample names = %q, %q: D-090 fixes them", flow.SampleFirstNameTR, flow.SampleFirstNameEN)
	}
}

func TestEmailPreviewJSON(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(flow.EmailPreview{Subject: "s", HTML: "<p>", Text: "t"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 3 || m["subject"] != "s" || m["html"] != "<p>" || m["text"] != "t" {
		t.Errorf("json = %s, want the keys subject, html and text", b)
	}
}
