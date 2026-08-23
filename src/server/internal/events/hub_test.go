package events

import (
	"testing"
	"time"
)

func TestReplayAndGap(t *testing.T) {
	h := NewHub()
	first := h.Publish("source.state", map[string]string{"state": "online"})
	second := h.Publish("source.state", map[string]string{"state": "stale"})
	sub := h.Subscribe(first.Sequence)
	defer sub.Close()
	select {
	case e := <-sub.C:
		if e.Sequence != second.Sequence || e.Type != "source.state" {
			t.Fatalf("replay %#v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("replay timeout")
	}
	for i := 0; i < 1100; i++ {
		h.Publish("connection.open", map[string]int{"i": i})
	}
	gap := h.Subscribe(1)
	defer gap.Close()
	select {
	case e := <-gap.C:
		if e.Type != "resync" {
			t.Fatalf("gap %#v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("gap timeout")
	}
}

func TestSlowSubscriberIsDisconnectedWithoutBlockingPublisher(t *testing.T) {
	h := NewHub()
	sub := h.Subscribe(0)
	for i := 0; i < 100; i++ {
		h.Publish("connection.open", i)
	}
	select {
	case e, ok := <-sub.C:
		if !ok || e.Type != "resync" {
			t.Fatalf("slow subscriber event %#v open=%v", e, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("slow subscriber not closed")
	}
	select {
	case _, ok := <-sub.C:
		if ok {
			t.Fatal("slow subscriber channel still open")
		}
	case <-time.After(time.Second):
		t.Fatal("slow subscriber close timeout")
	}
}
