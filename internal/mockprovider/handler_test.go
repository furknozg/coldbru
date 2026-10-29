package mockprovider

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestIdempotencyReplay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		body       any
		forwarded  func(*fakeForwarder) int
	}{
		{"sms", "/v1/sms", goodSMS(), func(f *fakeForwarder) int { return len(f.sms) }},
		{"email", "/v1/email", goodEmail(), func(f *fakeForwarder) int { return len(f.emails) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rg := newRig(t, quietConfig())
			first := rg.do(t, tc.path, "key-1", tc.body)
			if first.Code != http.StatusAccepted {
				t.Fatalf("first status = %d, body %s", first.Code, first.Body)
			}
			second := rg.do(t, tc.path, "key-1", tc.body)
			if second.Code != http.StatusAccepted || second.Body.String() != first.Body.String() {
				t.Errorf("replay = %d %q, want 202 %q", second.Code, second.Body, first.Body)
			}
			if got := tc.forwarded(rg.fwd); got != 1 {
				t.Errorf("forwarded %d times, want 1", got)
			}
			if got := rg.metric(tc.name, OutcomeReplayed); got != 1 {
				t.Errorf("replayed = %v, want 1", got)
			}
			// Another key is another message.
			other := rg.do(t, tc.path, "key-2", tc.body)
			if messageID(t, other) == messageID(t, first) {
				t.Error("a new key got the old message_id")
			}
		})
	}
}

func TestIdempotencyKeyWithAnotherBodyIs422(t *testing.T) {
	t.Parallel()
	rg := newRig(t, quietConfig())
	rg.do(t, "/v1/sms", "key-1", goodSMS())
	other := goodSMS()
	other.Text = "başka bir metin"
	rec := rg.do(t, "/v1/sms", "key-1", other)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if p := problemOf(t, rec); p.Code != CodeKeyConflict {
		t.Errorf("code = %q, want %q", p.Code, CodeKeyConflict)
	}
	if rg.fwd.count() != 1 {
		t.Errorf("forwarded %d, want 1", rg.fwd.count())
	}
	// The same key on the other channel is a different message: keys are scoped by channel.
	if rec := rg.do(t, "/v1/email", "key-1", goodEmail()); rec.Code != http.StatusAccepted {
		t.Errorf("email with an SMS key = %d, want 202", rec.Code)
	}
}

func TestReplaySkipsRateLimitAndFaults(t *testing.T) {
	t.Parallel()
	cfg := quietConfig()
	cfg.RateLimit, cfg.Burst = 1, 1
	rg := newRig(t, cfg)
	first := rg.do(t, "/v1/sms", "k", goodSMS())
	if first.Code != 202 {
		t.Fatalf("first = %d", first.Code)
	}
	// The bucket is empty, a new key is limited, but the stored key is still answered.
	if rec := rg.do(t, "/v1/sms", "k2", goodSMS()); rec.Code != 429 {
		t.Fatalf("new key = %d, want 429", rec.Code)
	}
	if rec := rg.do(t, "/v1/sms", "k", goodSMS()); rec.Code != 202 || rec.Body.String() != first.Body.String() {
		t.Errorf("replay under an empty bucket = %d %q", rec.Code, rec.Body)
	}
}

func TestStoreExpiresWithTheClock(t *testing.T) {
	t.Parallel()
	cfg := quietConfig()
	cfg.IdempotencyTTL = 24 * time.Hour
	rg := newRig(t, cfg)
	first := rg.do(t, "/v1/sms", "k", goodSMS())
	rg.do(t, "/v1/sms", "k2", goodSMS())
	if rg.h.Stored() != 2 {
		t.Fatalf("stored = %d, want 2", rg.h.Stored())
	}

	rg.clk.advance(24*time.Hour - time.Second)
	if n := rg.h.Sweep(); n != 0 || rg.h.Stored() != 2 {
		t.Errorf("sweep before the TTL removed %d, stored %d", n, rg.h.Stored())
	}
	if rec := rg.do(t, "/v1/sms", "k", goodSMS()); rec.Body.String() != first.Body.String() {
		t.Error("the record was gone before its TTL")
	}

	rg.clk.advance(2 * time.Second) // "k" was stored 24h+1s ago now; "k2" the same
	if n := rg.h.Sweep(); n != 2 || rg.h.Stored() != 0 {
		t.Errorf("sweep after the TTL removed %d, stored %d, want 2 and 0", n, rg.h.Stored())
	}
	again := rg.do(t, "/v1/sms", "k", goodSMS())
	if again.Code != 202 || messageID(t, again) == messageID(t, first) {
		t.Errorf("after the TTL the key must be a new message: %d %s", again.Code, again.Body)
	}
	if got := len(rg.fwd.sms); got != 3 {
		t.Errorf("forwarded %d, want 3 (k, k2, k again)", got)
	}
}

func TestExpiredRecordIsIgnoredBeforeTheSweep(t *testing.T) {
	t.Parallel()
	rg := newRig(t, quietConfig())
	first := rg.do(t, "/v1/sms", "k", goodSMS())
	rg.clk.advance(25 * time.Hour)
	// Different body under the same key: no conflict, because the record is dead.
	other := goodSMS()
	other.Text = "yeni"
	rec := rg.do(t, "/v1/sms", "k", other)
	if rec.Code != 202 || messageID(t, rec) == messageID(t, first) {
		t.Errorf("= %d %s", rec.Code, rec.Body)
	}
}

func TestRateLimitPerChannel(t *testing.T) {
	t.Parallel()
	cfg := quietConfig()
	cfg.RateLimit, cfg.Burst = 110, Burst
	rg := newRig(t, cfg)

	// 50 requests at one instant: only the burst passes.
	var ok, limited int
	for i := range 50 {
		rec := rg.do(t, "/v1/sms", "burst-"+strconv.Itoa(i), goodSMS())
		switch rec.Code {
		case 202:
			ok++
		case 429:
			limited++
			if got := rec.Header().Get("Retry-After"); got != "1" {
				t.Fatalf("Retry-After = %q, want 1", got)
			}
			if p := problemOf(t, rec); p.Code != CodeRateLimited {
				t.Fatalf("code = %q", p.Code)
			}
		default:
			t.Fatalf("status %d", rec.Code)
		}
	}
	if ok != Burst || limited != 50-Burst {
		t.Errorf("accepted %d and limited %d, want %d and %d", ok, limited, Burst, 50-Burst)
	}
	if got := rg.metric(ChannelSMS, OutcomeRateLimited); got != float64(limited) {
		t.Errorf("rate_limited metric = %v, want %d", got, limited)
	}
	// The email bucket is separate.
	if rec := rg.do(t, "/v1/email", "e-1", goodEmail()); rec.Code != 202 {
		t.Errorf("email while sms is limited = %d, want 202", rec.Code)
	}
	// Limited requests were neither forwarded nor stored.
	if got := len(rg.fwd.sms); got != Burst {
		t.Errorf("forwarded %d sms, want %d", got, Burst)
	}

	// 200 requests/s for one second: about 110 tokens plus the burst get through.
	rg2 := newRig(t, cfg)
	accepted := 0
	for i := range 200 {
		if rg2.do(t, "/v1/sms", "s-"+strconv.Itoa(i), goodSMS()).Code == 202 {
			accepted++
		}
		rg2.clk.advance(5 * time.Millisecond)
	}
	if accepted < 112 || accepted > 122 {
		t.Errorf("accepted %d of 200 requests in one second, want about 120 (110/s + burst 10)", accepted)
	}
}

func TestRateLimitedRequestIsRetriedWithTheSameKey(t *testing.T) {
	t.Parallel()
	cfg := quietConfig()
	cfg.RateLimit, cfg.Burst = 10, 1
	rg := newRig(t, cfg)
	rg.do(t, "/v1/sms", "a", goodSMS())
	if rec := rg.do(t, "/v1/sms", "b", goodSMS()); rec.Code != 429 {
		t.Fatalf("= %d, want 429", rec.Code)
	}
	rg.clk.advance(time.Second)
	if rec := rg.do(t, "/v1/sms", "b", goodSMS()); rec.Code != 202 {
		t.Errorf("retry of a limited key = %d, want 202 (429 is not stored)", rec.Code)
	}
}

func TestFaultInjectionWithAFixedSeed(t *testing.T) {
	t.Parallel()
	const n = 20000
	run := func() (counts map[int]int, outcomes []int) {
		cfg := quietConfig()
		cfg.ErrorRate, cfg.Rate429, cfg.TimeoutRate = 0.05, 0.02, 0.01
		cfg.Seed, cfg.HasSeed = 42, true
		rg := newRig(t, cfg)
		counts = map[int]int{}
		for i := range n {
			rec := rg.do(t, "/v1/sms", "k-"+strconv.Itoa(i), goodSMS())
			counts[rec.Code]++
			outcomes = append(outcomes, rec.Code)
		}
		if got, want := rg.metric(ChannelSMS, OutcomeInjected5xx), float64(counts[502]+counts[503]); got != want {
			t.Errorf("injected_5xx metric = %v, want %v", got, want)
		}
		if got := rg.metric(ChannelSMS, OutcomeInjected429); got != float64(counts[429]) {
			t.Errorf("injected_429 metric = %v, want %v", got, counts[429])
		}
		return counts, outcomes
	}
	counts, first := run()

	within := func(name string, got int, p float64) {
		mean := p * n
		tol := 5 * math.Sqrt(n*p*(1-p))
		if math.Abs(float64(got)-mean) > tol {
			t.Errorf("%s = %d, want %.0f ± %.0f", name, got, mean, tol)
		}
	}
	within("5xx", counts[502]+counts[503], 0.05)
	within("429", counts[429], 0.02)
	// The timeout requests answer 202 here (the wait is a no-op), as do the clean ones.
	within("202", counts[202], 0.93)
	if counts[502] == 0 || counts[503] == 0 {
		t.Errorf("both 502 and 503 should occur: %v", counts)
	}

	_, second := run()
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("same seed, different answer at request %d: %d vs %d", i, first[i], second[i])
		}
	}
}

func TestInjectedFailuresAreNotForwardedOrStored(t *testing.T) {
	t.Parallel()
	t.Run("5xx", func(t *testing.T) {
		t.Parallel()
		cfg := quietConfig()
		cfg.ErrorRate = 1
		rg := newRig(t, cfg)
		for i := range 40 {
			rec := rg.do(t, "/v1/email", "k"+strconv.Itoa(i), goodEmail())
			if rec.Code != 502 && rec.Code != 503 {
				t.Fatalf("status = %d", rec.Code)
			}
			problemOf(t, rec)
		}
		if rg.fwd.count() != 0 || rg.h.Stored() != 0 {
			t.Errorf("forwarded %d, stored %d, want 0 and 0", rg.fwd.count(), rg.h.Stored())
		}
	})
	t.Run("429", func(t *testing.T) {
		t.Parallel()
		cfg := quietConfig()
		cfg.Rate429 = 1
		rg := newRig(t, cfg)
		seen := map[string]bool{}
		for i := range 60 {
			rec := rg.do(t, "/v1/sms", "k"+strconv.Itoa(i), goodSMS())
			if rec.Code != 429 {
				t.Fatalf("status = %d", rec.Code)
			}
			ra, err := strconv.Atoi(rec.Header().Get("Retry-After"))
			if err != nil || ra < 1 || ra > 3 {
				t.Fatalf("Retry-After = %q, want 1..3", rec.Header().Get("Retry-After"))
			}
			seen[rec.Header().Get("Retry-After")] = true
		}
		if len(seen) != 3 {
			t.Errorf("Retry-After values seen: %v, want 1, 2 and 3", seen)
		}
		if rg.fwd.count() != 0 || rg.h.Stored() != 0 {
			t.Errorf("forwarded %d, stored %d, want 0 and 0", rg.fwd.count(), rg.h.Stored())
		}
	})
}

func TestLatencyIsUniformInTheConfiguredRange(t *testing.T) {
	t.Parallel()
	cfg := quietConfig()
	cfg.LatencyMin, cfg.LatencyMax = 50*time.Millisecond, 300*time.Millisecond
	rg := newRig(t, cfg)
	for i := range 300 {
		rg.do(t, "/v1/sms", "k"+strconv.Itoa(i), goodSMS())
	}
	lo, hi := time.Hour, time.Duration(0)
	var sum time.Duration
	for _, d := range rg.sleeps {
		lo, hi, sum = min(lo, d), max(hi, d), sum+d
	}
	if len(rg.sleeps) != 300 || lo < 50*time.Millisecond || hi > 300*time.Millisecond {
		t.Fatalf("%d sleeps in [%s, %s], want 300 in [50ms, 300ms]", len(rg.sleeps), lo, hi)
	}
	if mean := sum / 300; mean < 150*time.Millisecond || mean > 200*time.Millisecond {
		t.Errorf("mean latency %s, want about 175ms", mean)
	}
}

func TestTimeoutStoresBeforeDelaying(t *testing.T) {
	t.Parallel()
	cfg := quietConfig()
	cfg.TimeoutRate, cfg.TimeoutDelay = 1, 12*time.Second
	rg := newRig(t, cfg)

	var storedAtDelay, forwardedAtDelay int
	var retried *httpResult
	rg.onSleep = func(d time.Duration) {
		if d != 12*time.Second {
			return
		}
		storedAtDelay, forwardedAtDelay = rg.h.Stored(), rg.fwd.count()
		// While the first answer is held back, the client's retry with the same key must not wait
		// for it: the key is free and the stored 202 comes back.
		rec := rg.do(t, "/v1/sms", "k", goodSMS())
		retried = &httpResult{rec.Code, rec.Body.String()}
	}
	first := rg.do(t, "/v1/sms", "k", goodSMS())
	if first.Code != 202 {
		t.Fatalf("held answer = %d, want 202", first.Code)
	}
	if storedAtDelay != 1 || forwardedAtDelay != 1 {
		t.Errorf("at the delay: stored %d, forwarded %d, want 1 and 1", storedAtDelay, forwardedAtDelay)
	}
	if retried == nil || retried.code != 202 || retried.body != first.Body.String() {
		t.Errorf("retry during the delay = %+v, want 202 %q", retried, first.Body)
	}
	if rg.fwd.count() != 1 {
		t.Errorf("forwarded %d, want 1", rg.fwd.count())
	}
	if rg.metric(ChannelSMS, OutcomeTimeout) != 1 {
		t.Errorf("timeout metric = %v, want 1", rg.metric(ChannelSMS, OutcomeTimeout))
	}
}

type httpResult struct {
	code int
	body string
}

func TestClientLeavingDuringTheHoldKeepsTheMessageStored(t *testing.T) {
	t.Parallel()
	cfg := quietConfig()
	cfg.TimeoutRate = 1
	rg := newRig(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	rg.onSleep = func(d time.Duration) {
		if d == cfg.TimeoutDelay {
			cancel() // the client's 10 s timeout fires
		}
	}
	rec := rg.doCtx(t, ctx, "/v1/email", "k", goodEmail())
	if rec.Code != statusClientClosed {
		t.Errorf("status = %d, want %d", rec.Code, statusClientClosed)
	}
	// The retry gets the stored 202 and nothing is sent twice.
	retry := rg.do(t, "/v1/email", "k", goodEmail())
	if retry.Code != 202 || len(rg.fwd.emails) != 1 {
		t.Errorf("retry = %d, forwarded %d", retry.Code, len(rg.fwd.emails))
	}
}

func TestValidationErrors(t *testing.T) {
	t.Parallel()
	rg := newRig(t, quietConfig())
	long := string(make([]byte, 200))
	for _, tc := range []struct {
		name, path, key string
		body            any
		status          int
		code            string
	}{
		{"no key", "/v1/sms", "", goodSMS(), 400, CodeMissingKey},
		{"key with a space", "/v1/sms", "a b", goodSMS(), 400, CodeInvalidKey},
		{"key with a newline", "/v1/sms", "a\nb", goodSMS(), 400, CodeInvalidKey},
		{"key too long", "/v1/sms", long, goodSMS(), 400, CodeInvalidKey},
		{"not json", "/v1/sms", "k", "to=+905551112233", 400, CodeInvalidJSON},
		{"empty body", "/v1/sms", "k", "", 400, CodeInvalidJSON},
		{"two objects", "/v1/sms", "k", `{"to":"+905551112233","text":"a"}{}`, 400, CodeInvalidJSON},
		{"unknown field", "/v1/sms", "k", `{"to":"+905551112233","text":"a","from":"x"}`, 400, CodeInvalidJSON},
		{"wrong type", "/v1/sms", "k", `{"to":5,"text":"a"}`, 400, CodeInvalidJSON},
		{"sms no plus", "/v1/sms", "k", SMSRequest{To: "905551112233", Text: "a"}, 422, CodeValidation},
		{"sms letters", "/v1/sms", "k", SMSRequest{To: "+90555abc", Text: "a"}, 422, CodeValidation},
		{"sms too short", "/v1/sms", "k", SMSRequest{To: "+12345", Text: "a"}, 422, CodeValidation},
		{"sms empty to", "/v1/sms", "k", SMSRequest{Text: "a"}, 422, CodeValidation},
		{"sms empty text", "/v1/sms", "k", SMSRequest{To: "+905551112233", Text: "  "}, 422, CodeValidation},
		{"email bad address", "/v1/email", "k", EmailRequest{To: "not-an-address", Subject: "s", HTML: "h", Text: "t"}, 422, CodeValidation},
		{"email display name", "/v1/email", "k", EmailRequest{To: "Ayşe <ayse@example.com>", Subject: "s", HTML: "h", Text: "t"}, 422, CodeValidation},
		{"email empty subject", "/v1/email", "k", EmailRequest{To: "a@example.com", HTML: "h", Text: "t"}, 422, CodeValidation},
		{"email newline in subject", "/v1/email", "k", EmailRequest{To: "a@example.com", Subject: "s\r\nBcc: x@example.com", HTML: "h", Text: "t"}, 422, CodeValidation},
		{"email empty html", "/v1/email", "k", EmailRequest{To: "a@example.com", Subject: "s", Text: "t"}, 422, CodeValidation},
		{"email empty text", "/v1/email", "k", EmailRequest{To: "a@example.com", Subject: "s", HTML: "h"}, 422, CodeValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := rg.do(t, tc.path, tc.key, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.status, rec.Body)
			}
			if p := problemOf(t, rec); p.Code != tc.code {
				t.Errorf("code = %q, want %q", p.Code, tc.code)
			}
		})
	}
	if rg.fwd.count() != 0 || rg.h.Stored() != 0 {
		t.Errorf("forwarded %d, stored %d after only invalid requests", rg.fwd.count(), rg.h.Stored())
	}
	if rg.metric(ChannelSMS, OutcomeInvalid)+rg.metric(ChannelEmail, OutcomeInvalid) != 20 {
		t.Errorf("invalid metric = %v + %v, want 20 in all", rg.metric(ChannelSMS, OutcomeInvalid), rg.metric(ChannelEmail, OutcomeInvalid))
	}
}

func TestBodyAbove1MiBIs413(t *testing.T) {
	t.Parallel()
	rg := newRig(t, quietConfig())
	rec := rg.do(t, "/v1/sms", "k", `{"to":"+905551112233","text":"`+string(make([]byte, 0))+stringOf('a', 1<<20+10)+`"}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
	if p := problemOf(t, rec); p.Code != CodeTooLarge {
		t.Errorf("code = %q", p.Code)
	}
}

func stringOf(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func TestForwardFailureIs503AndNotStored(t *testing.T) {
	t.Parallel()
	rg := newRig(t, quietConfig())
	rg.fwd.setFail(errInbox)
	rec := rg.do(t, "/v1/email", "k", goodEmail())
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if p := problemOf(t, rec); p.Code != CodeForwardFailed {
		t.Errorf("code = %q", p.Code)
	}
	if rg.h.Stored() != 0 {
		t.Errorf("stored %d records, want 0", rg.h.Stored())
	}
	if rg.metric(ChannelEmail, OutcomeForwardFailed) != 1 {
		t.Errorf("forward_failed = %v", rg.metric(ChannelEmail, OutcomeForwardFailed))
	}

	// The inbox is back: the retry with the same key is forwarded now.
	rg.fwd.setFail(nil)
	retry := rg.do(t, "/v1/email", "k", goodEmail())
	if retry.Code != 202 || len(rg.fwd.emails) != 1 {
		t.Errorf("retry = %d, forwarded %d, want 202 and 1", retry.Code, len(rg.fwd.emails))
	}
}

func TestConcurrentRequestsWithOneKeyForwardOnce(t *testing.T) {
	t.Parallel()
	rg := newRig(t, quietConfig())
	rg.fwd.delay = 20 * time.Millisecond
	const n = 25
	ids := make([]string, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := rg.do(t, "/v1/sms", "same", goodSMS())
			codes[i] = rec.Code
			if rec.Code == 202 {
				ids[i] = messageID(t, rec)
			}
		}()
	}
	wg.Wait()
	for i := range n {
		if codes[i] != 202 || ids[i] != ids[0] {
			t.Fatalf("request %d: %d %q, want 202 %q", i, codes[i], ids[i], ids[0])
		}
	}
	if got := len(rg.fwd.sms); got != 1 {
		t.Errorf("forwarded %d times, want 1", got)
	}
}

func TestForwardedFieldsCarryTheMessageAndKey(t *testing.T) {
	t.Parallel()
	rg := newRig(t, quietConfig())
	smsRec := rg.do(t, "/v1/sms", "sms-key", goodSMS())
	emailRec := rg.do(t, "/v1/email", "mail-key", goodEmail())
	s := rg.fwd.sms[0]
	if s.To != "+905551112233" || s.Text != "Merhaba Ayşe" || s.IdempotencyKey != "sms-key" || s.MessageID != messageID(t, smsRec) {
		t.Errorf("sms = %+v", s)
	}
	e := rg.fwd.emails[0]
	if e.To != "ayse@example.com" || e.Subject != "Sepetin seni bekliyor" || e.IdempotencyKey != "mail-key" ||
		e.MessageID != messageID(t, emailRec) || e.From != DefaultEmailFrom || e.Date.IsZero() {
		t.Errorf("email = %+v", e)
	}
	if got := rg.metric(ChannelSMS, OutcomeAccepted); got != 1 {
		t.Errorf("accepted = %v", got)
	}
}
