package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStreamPublishesEventsToSubscribers(t *testing.T) {
	stream := NewStream(WithNow(func() time.Time {
		return time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	}))
	defer stream.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	payload := json.RawMessage(`{"issue_key":"ABC-1"}`)
	if err := stream.Publish(ctx, Event{
		Type:      TypeJiraTicketNew,
		Source:    "active_view",
		DedupeKey: "ABC-1",
		Payload:   payload,
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case event := <-received:
		if event.ID == "" {
			t.Fatal("published event should have an ID")
		}
		if event.Type != TypeJiraTicketNew || event.Source != "active_view" || event.DedupeKey != "ABC-1" {
			t.Fatalf("event = %#v", event)
		}
		if !event.At.Equal(time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)) {
			t.Fatalf("At = %s", event.At)
		}
		if string(event.Payload) != string(payload) {
			t.Fatalf("Payload = %s", event.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestStreamPublishHonorsCanceledContext(t *testing.T) {
	stream := NewStream()
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := stream.Publish(ctx, Event{Type: TypeJiraCacheRefreshRequested})

	if err == nil {
		t.Fatal("expected canceled publish error")
	}
}
