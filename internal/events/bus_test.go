package events

import (
	"testing"
	"time"
)

func TestBus_SubscribePublish(t *testing.T) {
	b := NewBus()
	ch := b.Subscribe("pipeline")

	b.Publish("pipeline", Event{Topic: "pipeline", Type: "start", Data: map[string]int{"run_id": 1}})

	select {
	case ev := <-ch:
		if ev.Type != "start" {
			t.Fatalf("expected type start, got %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	b.Unsubscribe(ch)
}

func TestBus_NoOtherTopic(t *testing.T) {
	b := NewBus()
	ch := b.Subscribe("pipeline")
	b.Publish("config", Event{Topic: "config", Type: "reload", Data: nil})

	select {
	case <-ch:
		t.Fatal("should not receive event from other topic")
	case <-time.After(100 * time.Millisecond):
	}

	b.Unsubscribe(ch)
}
