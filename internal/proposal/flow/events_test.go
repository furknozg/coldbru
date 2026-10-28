package flow

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func ev(c uuid.UUID, text string) Event {
	return ContentDelta{Meta: Meta{CampaignID: c, At: time.Unix(0, 0)}, Text: text}
}

func drain(s *Subscription) []string {
	var out []string
	for {
		select {
		case e, ok := <-s.C:
			if !ok {
				return out
			}
			out = append(out, e.(ContentDelta).Text)
		default:
			return out
		}
	}
}

func TestHubFansOutPerCampaign(t *testing.T) {
	t.Parallel()
	h := NewHub(8)
	a, b := uuid.New(), uuid.New()
	s1, s2, other := h.Subscribe(a), h.Subscribe(a), h.Subscribe(b)
	for _, txt := range []string{"1", "2", "3"} {
		h.Publish(ev(a, txt))
	}
	h.Publish(ev(b, "x"))
	for i, s := range []*Subscription{s1, s2} {
		if got := drain(s); len(got) != 3 || got[0] != "1" || got[2] != "3" {
			t.Errorf("subscriber %d got %v, want 1 2 3 in order", i, got)
		}
	}
	if got := drain(other); len(got) != 1 || got[0] != "x" {
		t.Errorf("the other campaign's subscriber got %v", got)
	}
	if h.Subscribers(a) != 2 || h.Subscribers(b) != 1 {
		t.Errorf("subscribers = %d, %d", h.Subscribers(a), h.Subscribers(b))
	}
	// Events carry their identity.
	e := ev(a, "t")
	if e.Campaign() != a || e.Type() != TypeContentDelta || !e.When().Equal(time.Unix(0, 0)) {
		t.Errorf("event = %+v", e)
	}
}

func TestHubSlowSubscriberDropsWithoutBlocking(t *testing.T) {
	t.Parallel()
	h := NewHub(2)
	c := uuid.New()
	slow, fast := h.Subscribe(c), h.Subscribe(c)
	var got []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range fast.C {
			got = append(got, e.(ContentDelta).Text)
		}
	}()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := 0; i < 5; i++ {
			h.Publish(ev(c, string(rune('a'+i))))
			time.Sleep(5 * time.Millisecond) // lets the fast consumer keep up
		}
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a slow subscriber")
	}
	fast.Close()
	<-done
	if len(got) != 5 {
		t.Errorf("fast subscriber got %v, want all 5", got)
	}
	// The slow one never read: it kept the first two and missed three.
	if kept := drain(slow); len(kept) != 2 || kept[0] != "a" || kept[1] != "b" {
		t.Errorf("slow subscriber kept %v, want a b", kept)
	}
	if slow.Dropped() != 3 || fast.Dropped() != 0 || h.Dropped() != 3 {
		t.Errorf("dropped = slow %d, fast %d, hub %d; want 3, 0, 3", slow.Dropped(), fast.Dropped(), h.Dropped())
	}
}

func TestHubUnsubscribe(t *testing.T) {
	t.Parallel()
	h := NewHub(4)
	c := uuid.New()
	s1, s2 := h.Subscribe(c), h.Subscribe(c)
	s1.Close()
	s1.Close() // idempotent
	if _, ok := <-s1.C; ok {
		t.Error("C is still open after Close")
	}
	h.Publish(ev(c, "after"))
	if got := drain(s2); len(got) != 1 {
		t.Errorf("the remaining subscriber got %v", got)
	}
	if h.Subscribers(c) != 1 || h.Dropped() != 0 {
		t.Errorf("subscribers = %d, dropped = %d", h.Subscribers(c), h.Dropped())
	}
	s2.Close()
	if h.Subscribers(c) != 0 {
		t.Error("the campaign entry stays after the last Close")
	}
	h.Publish(ev(c, "nobody")) // no subscribers: a no-op
}

func TestHubConcurrentPublishAndClose(t *testing.T) {
	t.Parallel()
	h := NewHub(1)
	c := uuid.New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s := h.Subscribe(c)
			for range 3 {
				select {
				case <-s.C:
				default:
				}
			}
			s.Close()
		}()
		go func() {
			defer wg.Done()
			for range 200 {
				h.Publish(ev(c, "x"))
			}
		}()
	}
	wg.Wait()
}

func TestNewHubDefaultBuffer(t *testing.T) {
	t.Parallel()
	h := NewHub(0)
	c := uuid.New()
	s := h.Subscribe(c)
	for range DefaultHubBuffer + 10 {
		h.Publish(ev(c, "x"))
	}
	if len(drain(s)) != DefaultHubBuffer || s.Dropped() != 10 {
		t.Errorf("dropped = %d", s.Dropped())
	}
}

func TestEventTypes(t *testing.T) {
	t.Parallel()
	m := Meta{}
	want := []struct {
		e    Event
		name string
	}{
		{ScreenResult{Meta: m}, TypeScreenResult}, {CampaignCancelled{Meta: m}, TypeCampaignCancelled}, {StepOpened{Meta: m}, TypeStepOpened},
		{StepApproved{Meta: m}, TypeStepApproved}, {StepSuperseded{Meta: m}, TypeStepSuperseded}, {GenerationStarted{Meta: m}, TypeGenerationStarted},
		{GenerationFailed{Meta: m}, TypeGenerationFailed}, {VariantCreated{Meta: m}, TypeVariantCreated}, {VariantApproved{Meta: m}, TypeVariantApproved},
		{VariantRejected{Meta: m}, TypeVariantRejected}, {VariantSuperseded{Meta: m}, TypeVariantSuperseded}, {ContentDelta{Meta: m}, TypeContentDelta},
		{ContextAdded{Meta: m}, TypeContextAdded}, {ContentReset{Meta: m}, TypeContentReset},
	}
	seen := map[string]bool{}
	for _, w := range want {
		if w.e.Type() != w.name || w.name == "" || seen[w.name] {
			t.Errorf("%T type = %q (want %q, unique)", w.e, w.e.Type(), w.name)
		}
		seen[w.name] = true
	}
}

func TestDeltaSinkMarksRepairAttempts(t *testing.T) {
	t.Parallel()
	camp, step := uuid.New(), uuid.New()
	s := &Service{hub: NewHub(0), clock: ClockFunc(func() time.Time { return time.Unix(0, 0) })}
	sub := s.hub.Subscribe(camp)
	defer sub.Close()
	slot := Slot{Language: "tr", Channel: "sms"}
	sink := s.deltaSink(camp, step, slot)

	// Attempt 1 streams, attempt 2 is announced by an empty fragment before its first one, attempt 3
	// starts straight with text. A repeated call of an attempt resets nothing.
	sink(1, "")
	sink(1, "a")
	sink(1, "b")
	sink(2, "")
	sink(2, "c")
	sink(2, "d")
	sink(3, "e")

	var got []string
	for {
		select {
		case e := <-sub.C:
			switch v := e.(type) {
			case ContentDelta:
				if v.StepID != step || v.Slot != slot {
					t.Errorf("delta %+v: wrong step or slot", v)
				}
				got = append(got, v.Text+string(rune('0'+v.Attempt)))
			case ContentReset:
				if v.StepID != step || v.Slot != slot {
					t.Errorf("reset %+v: wrong step or slot", v)
				}
				got = append(got, "reset"+string(rune('0'+v.Attempt)))
			default:
				t.Fatalf("unexpected event %T", e)
			}
			continue
		default:
		}
		break
	}
	want := []string{"a1", "b1", "reset2", "c2", "d2", "reset3", "e3"}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %q, want %q", i, got[i], want[i])
		}
	}
}
