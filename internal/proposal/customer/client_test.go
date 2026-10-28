package customer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

const testToken = "proposal-test-token"

func mustFilter(t *testing.T, src string) *compile.Filter {
	t.Helper()
	f, errs := compile.Compile(src)
	if len(errs) > 0 {
		t.Fatalf("Compile(%q): %v", src, errs)
	}
	return f
}

// fake is a test server and the client wired to it. Sleeping is recorded, not performed, and the
// jitter is the identity, so delays are exactly the backoff values.
type fake struct {
	*Client
	srv    *httptest.Server
	calls  atomic.Int32
	sleeps []time.Duration
}

func newFake(t *testing.T, h http.HandlerFunc) *fake {
	t.Helper()
	fk := &fake{}
	fk.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fk.calls.Add(1)
		h(w, r)
	}))
	t.Cleanup(fk.srv.Close)
	c, err := New(Config{URL: fk.srv.URL, Token: testToken, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	c.sleep = func(ctx context.Context, d time.Duration) error {
		fk.sleeps = append(fk.sleeps, d)
		return ctx.Err()
	}
	c.jitter = func(d time.Duration) time.Duration { return d }
	fk.Client = c
	return fk
}

// hang blocks until the client gives up. The request body is drained first: the server only
// notices a closed connection once it has read the body.
func hang(r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	select {
	case <-r.Context().Done():
	case <-time.After(10 * time.Second):
	}
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

const previewBody = `{"data":{"audiencePreview":{"count":46,"coverage":{"both":20,"emailOnly":13,"smsOnly":13,"unreachable":3},
"byLanguage":[{"language":"en","count":10,"coverage":{"both":4,"emailOnly":3,"smsOnly":3,"unreachable":1}},
{"language":"tr","count":36,"coverage":{"both":16,"emailOnly":10,"smsOnly":10,"unreachable":2}}]}}}`

func TestQueriesMatchSchema(t *testing.T) {
	validateQuery(t, previewQuery)
	validateQuery(t, recipientsQuery)
}

func TestPreviewDecodesAndSendsRequest(t *testing.T) {
	var gotAuth, gotCT, gotMethod string
	var gotBody request
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotCT, gotMethod = r.Header.Get("Authorization"), r.Header.Get("Content-Type"), r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("request body: %v", err)
		}
		writeJSON(w, 200, previewBody)
	})
	f := mustFilter(t, `city === "İstanbul"`)
	p, err := fk.Preview(t.Context(), f)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	want := &Preview{
		Count:    46,
		Coverage: Coverage{Both: 20, EmailOnly: 13, SmsOnly: 13, Unreachable: 3},
		ByLanguage: []LanguageCoverage{
			{Language: "en", Count: 10, Coverage: Coverage{Both: 4, EmailOnly: 3, SmsOnly: 3, Unreachable: 1}},
			{Language: "tr", Count: 36, Coverage: Coverage{Both: 16, EmailOnly: 10, SmsOnly: 10, Unreachable: 2}},
		},
	}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("Preview = %+v, want %+v", p, want)
	}
	if gotAuth != "Bearer "+testToken {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotCT != "application/json" || gotMethod != http.MethodPost {
		t.Errorf("method %s content type %q", gotMethod, gotCT)
	}
	if gotBody.Query != previewQuery || gotBody.OperationName != "AudiencePreview" {
		t.Errorf("query/operation = %q / %q", gotBody.Query, gotBody.OperationName)
	}
	wantVars := map[string]any{"filter": map[string]any{"city": map[string]any{"eq": "İstanbul"}}}
	if !reflect.DeepEqual(gotBody.Variables, wantVars) {
		t.Errorf("variables = %v, want %v", gotBody.Variables, wantVars)
	}
	if fk.calls.Load() != 1 || len(fk.sleeps) != 0 {
		t.Errorf("calls = %d, sleeps = %v", fk.calls.Load(), fk.sleeps)
	}
}

func TestRecipientsRequestAndDecode(t *testing.T) {
	var vars []map[string]any
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		var req request
		_ = json.NewDecoder(r.Body).Decode(&req)
		vars = append(vars, req.Variables)
		if req.Query != recipientsQuery || req.OperationName != "AudienceRecipients" {
			t.Errorf("query/operation = %q / %q", req.Query, req.OperationName)
		}
		writeJSON(w, 200, `{"data":{"audienceRecipients":{"nodes":[
{"customerId":"CUS-000001","firstName":"Ayşe","language":"tr","emailEligible":true,"smsEligible":false,"email":"a@example.com","phone":null}],
"pageInfo":{"hasNextPage":true,"endCursor":"c1"}}}}`)
	})
	f := mustFilter(t, `city === "İstanbul"`)
	page, err := fk.Recipients(t.Context(), f, 10, "")
	if err != nil {
		t.Fatalf("Recipients: %v", err)
	}
	want := &RecipientPage{
		Nodes:       []Recipient{{CustomerID: "CUS-000001", FirstName: "Ayşe", Language: "tr", EmailEligible: true, Email: "a@example.com"}},
		HasNextPage: true,
		EndCursor:   "c1",
	}
	if !reflect.DeepEqual(page, want) {
		t.Errorf("page = %+v, want %+v", page, want)
	}
	if _, err := fk.Recipients(t.Context(), f, 5, "c1"); err != nil {
		t.Fatal(err)
	}
	if v := vars[0]; v["first"] != float64(10) || v["after"] != nil {
		t.Errorf("first call vars = %v, want first 10 and after null", v)
	}
	if v := vars[1]; v["first"] != float64(5) || v["after"] != "c1" {
		t.Errorf("second call vars = %v, want first 5 and after c1", v)
	}
}

func TestRecipientsRejectsBadPageSize(t *testing.T) {
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { t.Error("request must not be sent") })
	f := mustFilter(t, `city === "Ankara"`)
	for _, n := range []int{0, -1, MaxPageSize + 1} {
		if _, err := fk.Recipients(t.Context(), f, n, ""); err == nil {
			t.Errorf("first = %d: want an error", n)
		}
	}
}

func TestNilFilter(t *testing.T) {
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { t.Error("request must not be sent") })
	if _, err := fk.Preview(t.Context(), nil); err == nil {
		t.Error("want an error for a nil filter")
	}
}

func TestEachRecipientPageWalksAllPages(t *testing.T) {
	// 2500 recipients in pages of 1000; the cursor is the index of the next page.
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		var req request
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Variables["first"] != float64(MaxPageSize) {
			t.Errorf("first = %v, want %d", req.Variables["first"], MaxPageSize)
		}
		start := 0
		if after, ok := req.Variables["after"].(string); ok {
			fmt.Sscanf(after, "p%d", &start)
		}
		end := min(start+MaxPageSize, 2500)
		var nodes []string
		for i := start; i < end; i++ {
			nodes = append(nodes, fmt.Sprintf(`{"customerId":"CUS-%06d","firstName":"n","language":"tr","emailEligible":true,"smsEligible":false,"email":"e","phone":null}`, i))
		}
		cursor := "null"
		if end < 2500 {
			cursor = fmt.Sprintf(`"p%d"`, end)
		}
		writeJSON(w, 200, fmt.Sprintf(`{"data":{"audienceRecipients":{"nodes":[%s],"pageInfo":{"hasNextPage":%t,"endCursor":%s}}}}`,
			strings.Join(nodes, ","), end < 2500, cursor))
	})
	all, err := fk.AllRecipients(t.Context(), mustFilter(t, `city === "Ankara"`))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2500 || all[0].CustomerID != "CUS-000000" || all[2499].CustomerID != "CUS-002499" {
		t.Errorf("got %d recipients, first %q last %q", len(all), all[0].CustomerID, all[len(all)-1].CustomerID)
	}
	if fk.calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", fk.calls.Load())
	}
}

func TestEachRecipientPageStopsOnCallbackError(t *testing.T) {
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"data":{"audienceRecipients":{"nodes":[{"customerId":"CUS-1","firstName":"n","language":"tr","emailEligible":true,"smsEligible":false}],"pageInfo":{"hasNextPage":true,"endCursor":"x"}}}}`)
	})
	boom := errors.New("boom")
	err := fk.EachRecipientPage(t.Context(), mustFilter(t, `city === "Ankara"`), func([]Recipient) error { return boom })
	if !errors.Is(err, boom) || fk.calls.Load() != 1 {
		t.Errorf("err = %v, calls = %d", err, fk.calls.Load())
	}
}

func TestEachRecipientPageRejectsStuckCursor(t *testing.T) {
	for name, cursor := range map[string]string{"null": "null", "repeated": `"same"`} {
		t.Run(name, func(t *testing.T) {
			fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, 200, `{"data":{"audienceRecipients":{"nodes":[],"pageInfo":{"hasNextPage":true,"endCursor":`+cursor+`}}}}`)
			})
			err := fk.EachRecipientPage(t.Context(), mustFilter(t, `city === "Ankara"`), func([]Recipient) error { return nil })
			if err == nil || !strings.Contains(err.Error(), "cursor") {
				t.Errorf("err = %v, want a cursor error", err)
			}
			if n := fk.calls.Load(); n > 2 {
				t.Errorf("calls = %d, the loop did not stop", n)
			}
		})
	}
}

func TestRetryOn503ThenSuccess(t *testing.T) {
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {})
	fk.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fk.calls.Add(1) < 3 {
			writeJSON(w, 503, `unavailable`)
			return
		}
		writeJSON(w, 200, previewBody)
	})
	p, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`))
	if err != nil || p.Count != 46 {
		t.Fatalf("Preview = %+v, %v", p, err)
	}
	if got := fk.calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
	if want := []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}; !reflect.DeepEqual(fk.sleeps, want) {
		t.Errorf("backoff = %v, want %v", fk.sleeps, want)
	}
}

func TestRetryStatusesAndAttemptCap(t *testing.T) {
	for _, status := range []int{502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, status, `down`) })
			_, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`))
			var se *StatusError
			if !errors.As(err, &se) || se.Status != status || se.Body != "down" {
				t.Fatalf("err = %v, want a StatusError %d", err, status)
			}
			if !strings.Contains(err.Error(), "3 attempts") {
				t.Errorf("err = %q, want it to mention the attempts", err)
			}
			if got := fk.calls.Load(); got != maxAttempts {
				t.Errorf("calls = %d, want %d", got, maxAttempts)
			}
			if len(fk.sleeps) != maxAttempts-1 {
				t.Errorf("sleeps = %v", fk.sleeps)
			}
		})
	}
}

func TestRetryOnTransportError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close() // the client sees EOF
			return
		}
		writeJSON(w, 200, previewBody)
	}))
	defer srv.Close()
	c, _ := New(Config{URL: srv.URL, Token: testToken})
	c.sleep = func(context.Context, time.Duration) error { return nil }
	if _, err := c.Preview(t.Context(), mustFilter(t, `city === "Ankara"`)); err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestRetryOnConnectionRefusedGivesUpAfterThree(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens any more
	c, _ := New(Config{URL: "http://" + addr, Token: testToken})
	var sleeps int
	c.sleep = func(context.Context, time.Duration) error { sleeps++; return nil }
	_, err = c.Preview(t.Context(), mustFilter(t, `city === "Ankara"`))
	if err == nil || !strings.Contains(err.Error(), "3 attempts") {
		t.Fatalf("err = %v", err)
	}
	if sleeps != maxAttempts-1 {
		t.Errorf("sleeps = %d, want %d", sleeps, maxAttempts-1)
	}
}

func TestNoRetry(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		check  func(t *testing.T, err error)
	}{
		{"400 plain", 400, `bad request`, func(t *testing.T, err error) {
			var se *StatusError
			if !errors.As(err, &se) || se.Status != 400 {
				t.Errorf("err = %v, want StatusError 400", err)
			}
		}},
		{"404", 404, ``, func(t *testing.T, err error) {
			var se *StatusError
			if !errors.As(err, &se) || se.Status != 404 {
				t.Errorf("err = %v, want StatusError 404", err)
			}
		}},
		{"500 plain", 500, `oops`, func(t *testing.T, err error) {
			var se *StatusError
			if !errors.As(err, &se) || se.Status != 500 {
				t.Errorf("err = %v, want StatusError 500", err)
			}
		}},
		{"401", 401, `{"error":"unauthorized"}`, func(t *testing.T, err error) {
			if !IsUnauthorized(err) {
				t.Errorf("err = %v, want unauthorized", err)
			}
		}},
		{"graphql error with 200", 200, `{"data":null,"errors":[{"message":"internal error","extensions":{"code":"INTERNAL"}}]}`, func(t *testing.T, err error) {
			var ce *Error
			if !errors.As(err, &ce) || ce.Code != CodeInternal {
				t.Errorf("err = %v, want INTERNAL", err)
			}
		}},
		{"graphql error with 422", 422, `{"errors":[{"message":"unknown field","extensions":{"code":"GRAPHQL_VALIDATION_FAILED"}}],"data":null}`, func(t *testing.T, err error) {
			var ce *Error
			if !errors.As(err, &ce) || ce.Code != "GRAPHQL_VALIDATION_FAILED" {
				t.Errorf("err = %v, want the GraphQL error", err)
			}
		}},
		{"graphql error with 503 body is still retried", 0, ``, nil},
	}
	for _, tt := range tests {
		if tt.check == nil {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, tt.status, tt.body) })
			_, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`))
			if err == nil {
				t.Fatal("want an error")
			}
			tt.check(t, err)
			if fk.calls.Load() != 1 || len(fk.sleeps) != 0 {
				t.Errorf("calls = %d, sleeps = %v, want a single attempt", fk.calls.Load(), fk.sleeps)
			}
		})
	}
}

func TestBadSuccessBodies(t *testing.T) {
	for name, body := range map[string]string{
		"not json":   `<html>`,
		"no data":    `{}`,
		"null data":  `{"data":null}`,
		"wrong data": `{"data":{"audiencePreview":{"count":"many"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, body) })
			if _, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`)); err == nil {
				t.Error("want an error")
			}
			if fk.calls.Load() != 1 {
				t.Errorf("calls = %d, want 1", fk.calls.Load())
			}
		})
	}
}

func TestOversizedResponseIsRejected(t *testing.T) {
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write(make([]byte, maxBody+10))
	})
	_, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`))
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("err = %v", err)
	}
	if fk.calls.Load() != 1 {
		t.Errorf("calls = %d, an oversized response must not be retried", fk.calls.Load())
	}
}

func TestPerAttemptTimeout(t *testing.T) {
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		hang(r)
	})
	fk.timeout = 30 * time.Millisecond
	start := time.Now()
	_, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want a deadline error", err)
	}
	if got := fk.calls.Load(); got != maxAttempts {
		t.Errorf("calls = %d, want %d: a timed-out attempt is retried", got, maxAttempts)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("took %s", time.Since(start))
	}
}

func TestTimeoutAppliesPerAttemptNotInTotal(t *testing.T) {
	// Two attempts of 80 ms each succeed under a 100 ms limit; the third is fast.
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {})
	fk.timeout = 100 * time.Millisecond
	fk.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fk.calls.Add(1) < 3 {
			writeJSON(w, 503, `x`)
			time.Sleep(80 * time.Millisecond)
			return
		}
		writeJSON(w, 200, previewBody)
	})
	if _, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`)); err != nil {
		t.Fatal(err)
	}
}

func TestContextCancelStopsRetrying(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		cancel()
		hang(r)
	})
	_, err := fk.Preview(ctx, mustFilter(t, `city === "Ankara"`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if fk.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", fk.calls.Load())
	}
}

func TestContextCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 503, `x`) })
	fk.sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return ctx.Err()
	}
	_, err := fk.Preview(ctx, mustFilter(t, `city === "Ankara"`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if fk.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", fk.calls.Load())
	}
}

func TestContextDeadlineBeforeCall(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, previewBody) })
	if _, err := fk.Preview(ctx, mustFilter(t, `city === "Ankara"`)); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestBackoffGrowsAndJitterStaysInRange(t *testing.T) {
	if backoff(1) != 200*time.Millisecond || backoff(2) != 400*time.Millisecond {
		t.Errorf("backoff = %s, %s", backoff(1), backoff(2))
	}
	for range 200 {
		for _, d := range []time.Duration{backoff(1), backoff(2)} {
			if got := equalJitter(d); got < d/2 || got > d {
				t.Fatalf("equalJitter(%s) = %s, want %s..%s", d, got, d/2, d)
			}
		}
	}
}

func TestSleepCtx(t *testing.T) {
	if err := sleepCtx(t.Context(), time.Millisecond); err != nil {
		t.Errorf("sleepCtx = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("sleepCtx = %v, want context.Canceled", err)
	}
}

// customerError builds the JSON of a Customer GraphQL error response.
func customerError(code, msg, filterPath, extra string) string {
	ext := fmt.Sprintf(`"code":%q`, code)
	if filterPath != "" {
		ext += `,"filterPath":` + filterPath
	}
	if extra != "" {
		ext += "," + extra
	}
	return fmt.Sprintf(`{"errors":[{"message":%q,"path":["audiencePreview"],"extensions":{%s}}],"data":null}`, msg, ext)
}

func TestUnknownValueBecomesDSLError(t *testing.T) {
	src := `retired === false &&
city === "Istnbul"`
	body := customerError("UNKNOWN_VALUE", `"Istnbul" is not a known value of city; did you mean "İstanbul"?`,
		`["filter","and",1,"city","eq"]`, `"field":"city","value":"Istnbul","suggestions":["İstanbul","İzmit"]`)
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, body) })
	_, err := fk.Preview(t.Context(), mustFilter(t, src))
	var de *dsl.Error
	if !errors.As(err, &de) {
		t.Fatalf("err = %T %v, want *dsl.Error", err, err)
	}
	if de.Code != dsl.CodeUnknownValue {
		t.Errorf("code = %s", de.Code)
	}
	if de.Message != `"Istnbul" is not a known value of city; did you mean "İstanbul"?` {
		t.Errorf("message = %q", de.Message)
	}
	if de.Start.Line != 2 || de.Start.Col != 10 || de.End.Line != 2 || de.End.Col != 19 {
		t.Errorf("range = %d:%d-%d:%d, want 2:10-2:19", de.Start.Line, de.Start.Col, de.End.Line, de.End.Col)
	}
	if de.Near != `"Istnbul"` {
		t.Errorf("near = %q", de.Near)
	}
	if !reflect.DeepEqual(de.Suggestions, []string{"İstanbul", "İzmit"}) {
		t.Errorf("suggestions = %v", de.Suggestions)
	}
	if got := DSLErrors(err); len(got) != 1 || got[0] != de {
		t.Errorf("DSLErrors = %v", got)
	}
	if fk.calls.Load() != 1 {
		t.Errorf("calls = %d, a GraphQL error is not retried", fk.calls.Load())
	}
}

func TestUnknownValueInListPointsAtTheElement(t *testing.T) {
	src := `vehicleMake in ["Ford", "Audi", "Fordd"]`
	body := customerError("UNKNOWN_VALUE", "unknown", `["filter","vehicleMake","in",2]`, `"suggestions":["Ford"]`)
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, body) })
	_, err := fk.Preview(t.Context(), mustFilter(t, src))
	de := DSLErrors(err)
	if len(de) != 1 || de[0].Near != `"Fordd"` || de[0].Start.Col != 33 {
		t.Fatalf("DSLErrors = %+v", de)
	}
}

func TestComplexityLimitBecomesDSLError(t *testing.T) {
	body := customerError("COMPLEXITY_LIMIT", "filter has more than 100 condition nodes", `["filter","and"]`, "")
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, body) })
	_, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara" && totalOrders > 1`))
	de := DSLErrors(err)
	if len(de) != 1 || de[0].Code != dsl.CodeComplexityLimit || de[0].Start.Col != 1 || de[0].Suggestions != nil {
		t.Fatalf("DSLErrors = %+v", de)
	}
}

func TestBadUserInputKeepsSpan(t *testing.T) {
	src := `lastActiveDate >= ago("30d")`
	body := customerError("BAD_USER_INPUT", "ago too large", `["filter","lastActiveDate","gte","ago"]`, "")
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, body) })
	_, err := fk.Preview(t.Context(), mustFilter(t, src))
	var ce *Error
	if !errors.As(err, &ce) {
		t.Fatalf("err = %T %v, want *Error", err, err)
	}
	if ce.Code != CodeBadUserInput || ce.Message != "ago too large" {
		t.Errorf("error = %+v", ce)
	}
	if ce.Span == nil || ce.Span.Start.Col != 19 || ce.Span.End.Col != 29 {
		t.Errorf("span = %+v, want cols 19-29", ce.Span)
	}
	if !reflect.DeepEqual(ce.Path, []any{"audiencePreview"}) || len(ce.FilterPath) != 4 {
		t.Errorf("paths = %v / %v", ce.Path, ce.FilterPath)
	}
	if ce.Extensions["code"] != CodeBadUserInput {
		t.Errorf("extensions = %v", ce.Extensions)
	}
	if got := DSLErrors(err); len(got) != 0 {
		t.Errorf("DSLErrors = %v, want none", got)
	}
}

func TestOtherCustomerErrors(t *testing.T) {
	tests := []struct {
		name, body, code string
	}{
		{"forbidden", customerError("FORBIDDEN", "missing scope", "", ""), CodeForbidden},
		{"query timeout", customerError("QUERY_TIMEOUT", "query timed out; narrow the request", "", ""), CodeQueryTimeout},
		{"internal", customerError("INTERNAL", "internal error", "", ""), CodeInternal},
		{"unknown value without a filter path", customerError("UNKNOWN_VALUE", "x", "", ""), CodeUnknownValue},
		{"unknown value with an unmappable path", customerError("UNKNOWN_VALUE", "x", `["other","city"]`, ""), CodeUnknownValue},
		{"no code at all", `{"errors":[{"message":"boom"}],"data":null}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, tt.body) })
			_, err := fk.Preview(t.Context(), mustFilter(t, `city === "Ankara"`))
			var ce *Error
			if !errors.As(err, &ce) || ce.Code != tt.code {
				t.Fatalf("err = %T %v, want *Error %q", err, err, tt.code)
			}
			if ce.Span != nil {
				t.Errorf("span = %+v, want none", ce.Span)
			}
			if len(DSLErrors(err)) != 0 {
				t.Error("want no dsl errors")
			}
			if fk.calls.Load() != 1 {
				t.Errorf("calls = %d", fk.calls.Load())
			}
		})
	}
}

func TestSeveralErrorsAreJoined(t *testing.T) {
	src := `city === "Istnbul" && vehicleMake === "gold"`
	body := `{"data":null,"errors":[
{"message":"a","extensions":{"code":"UNKNOWN_VALUE","filterPath":["filter","and",0,"city","eq"],"suggestions":["İstanbul"]}},
{"message":"b","extensions":{"code":"FORBIDDEN"}},
{"message":"c","extensions":{"code":"UNKNOWN_VALUE","filterPath":["filter","and",1,"vehicleMake","eq"],"suggestions":["Gold"]}}]}`
	fk := newFake(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, body) })
	_, err := fk.Preview(t.Context(), mustFilter(t, src))
	des := DSLErrors(err)
	if len(des) != 2 || des[0].Near != `"Istnbul"` || des[1].Near != `"gold"` {
		t.Fatalf("DSLErrors = %+v", des)
	}
	var ce *Error
	if !errors.As(err, &ce) || ce.Code != CodeForbidden {
		t.Errorf("errors.As(*Error) = %v", ce)
	}
}

func TestDSLErrorsWalksWrapping(t *testing.T) {
	inner := &dsl.Error{Code: dsl.CodeUnknownValue}
	wrapped := fmt.Errorf("repair: %w", errors.Join(errors.New("x"), fmt.Errorf("again: %w", inner)))
	if got := DSLErrors(wrapped); len(got) != 1 || got[0] != inner {
		t.Errorf("DSLErrors = %v", got)
	}
	if DSLErrors(nil) != nil || DSLErrors(errors.New("plain")) != nil {
		t.Error("want nil for errors without dsl errors")
	}
}

func TestErrorStrings(t *testing.T) {
	if got := (&Error{Code: "X", Message: "m"}).Error(); got != "customer: X: m" {
		t.Errorf("got %q", got)
	}
	if got := (&Error{Message: "m"}).Error(); got != "customer: m" {
		t.Errorf("got %q", got)
	}
	if got := (&StatusError{Status: 418}).Error(); !strings.Contains(got, "418") {
		t.Errorf("got %q", got)
	}
	if got := (&UnauthorizedError{}).Error(); !strings.Contains(got, KeyToken) {
		t.Errorf("got %q", got)
	}
	long := snippet([]byte(strings.Repeat("x ", 500)))
	if len(long) > errBodyLen+len("…") {
		t.Errorf("snippet length %d", len(long))
	}
}

func TestNewValidatesConfig(t *testing.T) {
	_, err := New(Config{})
	if err == nil || !strings.Contains(err.Error(), KeyURL) || !strings.Contains(err.Error(), KeyToken) {
		t.Errorf("err = %v, want both keys named", err)
	}
	c, err := New(Config{URL: "http://x", Token: "t"})
	if err != nil || c.timeout != DefaultTimeout {
		t.Errorf("timeout = %v, err = %v; want the default", c.timeout, err)
	}
	custom := &http.Client{}
	c, _ = New(Config{URL: "http://x", Token: "t", Timeout: time.Second}, WithHTTPClient(custom))
	if c.http != custom || c.timeout != time.Second {
		t.Error("options and timeout not applied")
	}
}

func TestConfigStringHidesToken(t *testing.T) {
	s := Config{URL: "http://x", Token: "super-secret"}.String()
	if strings.Contains(s, "super-secret") || !strings.Contains(s, "http://x") {
		t.Errorf("String() = %q", s)
	}
	if got := fmt.Sprintf("%v", Config{URL: "u", Token: "super-secret"}); strings.Contains(got, "super-secret") {
		t.Errorf("%%v leaks the token: %q", got)
	}
}
