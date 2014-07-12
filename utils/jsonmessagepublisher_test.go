package utils

import (
	"testing"
	"time"
)

func assertSubscribersCount(t *testing.T, q *JSONMessagePublisher, expected int) {
	if q.SubscribersCount() != expected {
		t.Fatalf("Expected %d registered subscribers, got %d", expected, q.SubscribersCount())
	}
}

func TestJSONMessagePublisherSubscription(t *testing.T) {
	q := NewJSONMessagePublisher()
	l1 := make(chan JSONMessage)
	l2 := make(chan JSONMessage)

	assertSubscribersCount(t, q, 0)
	q.Subscribe(l1)
	assertSubscribersCount(t, q, 1)
	q.Subscribe(l2)
	assertSubscribersCount(t, q, 2)

	q.Unsubscribe(l1)
	q.Unsubscribe(l2)
	assertSubscribersCount(t, q, 0)
}

func TestJSONMessagePublisherPublish(t *testing.T) {
	q := NewJSONMessagePublisher()
	l1 := make(chan JSONMessage)
	l2 := make(chan JSONMessage)

	go func() {
		for {
			select {
			case <-l1:
				close(l1)
				l1 = nil
			case <-l2:
				close(l2)
				l2 = nil
			case <-time.After(1 * time.Second):
				q.Unsubscribe(l1)
				q.Unsubscribe(l2)
				t.Fatal("Timeout waiting for broadcasted message")
			}
		}
	}()

	q.Subscribe(l1)
	q.Subscribe(l2)
	q.Publish(JSONMessage{})
}

func TestJSONMessagePublishTimeout(t *testing.T) {
	q := NewJSONMessagePublisher()
	l := make(chan JSONMessage)
	q.Subscribe(l)

	c := make(chan struct{})
	go func() {
		q.Publish(JSONMessage{})
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("Timeout publishing message")
	}
}
