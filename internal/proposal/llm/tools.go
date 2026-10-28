package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// CodeEmptyAudience is the code of a filter that is valid but selects no reachable customer. It is
// local to this package: the parser and semantic pass codes (D-065) stay unchanged.
const CodeEmptyAudience dsl.Code = "EMPTY_AUDIENCE"

// SegmentTools is what the segment loop needs from the backend: it is how a filter the model
// wrote is checked against the real catalog and the real Customer service. Implementations must be
// safe for concurrent use.
type SegmentTools interface {
	// CountAudience previews the audience of src (the count_audience tool). A filter the model can
	// repair is reported as an error that holds *dsl.Error values (customer.DSLErrors finds them);
	// a zero count is a normal answer. Any other error is a failure of the backend.
	CountAudience(ctx context.Context, src string) (*customer.Preview, error)

	// Validate checks src for propose_segment: parse, catalog, Customer, and a non-empty audience.
	// The filter and the preview are set only when errs is empty. errs are problems the model can
	// repair; err is a failure of the backend (nothing the model can fix).
	Validate(ctx context.Context, src string) (f *compile.Filter, p *customer.Preview, errs []*dsl.Error, err error)
}

// Previewer is the part of *customer.Client that the default tools use.
type Previewer interface {
	Preview(ctx context.Context, f *compile.Filter) (*customer.Preview, error)
}

// CustomerTools is the default SegmentTools: compile.Compile plus the Customer preview (D-067).
type CustomerTools struct{ previewer Previewer }

// NewCustomerTools returns the default tools over a Customer client.
func NewCustomerTools(p Previewer) *CustomerTools { return &CustomerTools{previewer: p} }

// ValidationError is the error CountAudience returns for a filter that did not compile. It joins
// the *dsl.Error values, so customer.DSLErrors reads them back.
type ValidationError struct{ Errors []*dsl.Error }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Errors))
	for i, de := range e.Errors {
		parts[i] = de.Error()
	}
	return strings.Join(parts, "; ")
}

// Unwrap returns the individual errors.
func (e *ValidationError) Unwrap() []error {
	out := make([]error, len(e.Errors))
	for i, de := range e.Errors {
		out[i] = de
	}
	return out
}

// CountAudience compiles src and asks Customer for the audience preview.
func (t *CustomerTools) CountAudience(ctx context.Context, src string) (*customer.Preview, error) {
	f, errs := compile.Compile(src)
	if len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	return t.previewer.Preview(ctx, f)
}

// Validate compiles src, previews it, and treats an audience of zero as an error (D-069).
func (t *CustomerTools) Validate(ctx context.Context, src string) (*compile.Filter, *customer.Preview, []*dsl.Error, error) {
	f, errs := compile.Compile(src)
	if len(errs) > 0 {
		return nil, nil, errs, nil
	}
	p, err := t.previewer.Preview(ctx, f)
	if err != nil {
		if des := customer.DSLErrors(err); len(des) > 0 {
			return nil, nil, des, nil
		}
		return nil, nil, nil, err
	}
	if p.Count == 0 {
		return nil, nil, []*dsl.Error{emptyAudience(src, p)}, nil
	}
	return f, p, nil, nil
}

// emptyAudience is the validation error for a filter that matches nobody reachable. It spans the
// whole source so that the repair prompt has a position.
func emptyAudience(src string, p *customer.Preview) *dsl.Error {
	start := dsl.Pos{Line: 1, Col: 1}
	end := endPos(src)
	msg := "the filter is valid but selects no reachable customer; loosen or remove a condition"
	if p != nil && p.Coverage.Unreachable > 0 {
		msg = fmt.Sprintf("the filter matches %d customers but none can be reached on any channel; loosen or remove a condition", p.Coverage.Unreachable)
	}
	return dsl.NewError(src, CodeEmptyAudience, msg, start, end)
}

// endPos returns the position after the last rune of src.
func endPos(src string) dsl.Pos {
	pos := dsl.Pos{Offset: len(src), Line: 1, Col: 1}
	for _, r := range src {
		if r == '\n' {
			pos.Line++
			pos.Col = 1
		} else {
			pos.Col++
		}
	}
	return pos
}

// RepairableErrors returns the DSL errors inside err, however wrapped or joined: the problems the
// model can fix. It is empty when err is a failure of the backend.
func RepairableErrors(err error) []*dsl.Error { return customer.DSLErrors(err) }

// FormatErrors renders DSL errors as the JSON text of an is_error tool_result (D-069): per error
// the code, the 1-based line and rune column, the message, the text near the problem and the
// suggestions. Models read JSON reliably and tests can decode it.
func FormatErrors(errs []*dsl.Error) string {
	type item struct {
		Code        string   `json:"code"`
		Line        int      `json:"line"`
		Column      int      `json:"column"`
		Message     string   `json:"message"`
		Near        string   `json:"near,omitempty"`
		Suggestions []string `json:"suggestions,omitempty"`
	}
	out := struct {
		Error  string `json:"error"`
		Errors []item `json:"errors"`
		Next   string `json:"next"`
	}{Error: "the DSL was rejected", Next: "Fix every listed problem and call the tool again."}
	for _, e := range errs {
		out.Errors = append(out.Errors, item{
			Code: string(e.Code), Line: e.Start.Line, Column: e.Start.Col,
			Message: e.Message, Near: e.Near, Suggestions: e.Suggestions,
		})
	}
	b, err := marshalNoEscape(out)
	if err != nil {
		return "the DSL was rejected" // unreachable: plain strings and ints
	}
	return b
}

// previewView is what the segment model sees of a preview: sizes and channel coverage only. The
// longest first names (D-071) are for the content step and are left out, so the segment prompt's
// view of count_audience stays as D-069 specified it.
type previewView struct {
	Count      int                `json:"count"`
	Coverage   customer.Coverage  `json:"coverage"`
	ByLanguage []languageViewItem `json:"byLanguage"`
}

type languageViewItem struct {
	Language string            `json:"language"`
	Count    int               `json:"count"`
	Coverage customer.Coverage `json:"coverage"`
}

// FormatPreview renders a preview as the JSON text of a successful count_audience result.
func FormatPreview(p *customer.Preview) string {
	v := previewView{Count: p.Count, Coverage: p.Coverage}
	if p.ByLanguage != nil {
		v.ByLanguage = make([]languageViewItem, len(p.ByLanguage))
		for i, l := range p.ByLanguage {
			v.ByLanguage[i] = languageViewItem{Language: l.Language, Count: l.Count, Coverage: l.Coverage}
		}
	}
	b, err := marshalNoEscape(v)
	if err != nil {
		return "{}"
	}
	return b
}

func marshalNoEscape(v any) (string, error) {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
