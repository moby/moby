package pubsub

import (
	"testing"
	"time"
)

func TestSendToOneSub(t *testing.T) {
	p := NewPublisher(100*time.Millisecond, 10)
	c := p.Subscribe()

	p.Publish("hi")

	msg := <-c
	if msg.(string) != "hi" {
		t.Fatalf("expected message hi but received %v", msg)
	}
}

func TestSendToMultipleSubs(t *testing.T) {
	p := NewPublisher(100*time.Millisecond, 10)
	subs := []chan interface{}{}
	subs = append(subs, p.Subscribe(), p.Subscribe(), p.Subscribe())

	p.Publish("hi")

	for _, c := range subs {
		msg := <-c
		if msg.(string) != "hi" {
			t.Fatalf("expected message hi but received %v", msg)
		}
	}
}

func TestEvictOneSub(t *testing.T) {
	p := NewPublisher(100*time.Millisecond, 10)
	s1 := p.Subscribe()
	s2 := p.Subscribe()

	p.Evict(s1)
	p.Publish("hi")
	if _, ok := <-s1; ok {
		t.Fatal("expected s1 to not receive the published message")
	}

	msg := <-s2
	if msg.(string) != "hi" {
		t.Fatalf("expected message hi but received %v", msg)
	}
}

func TestClosePublisher(t *testing.T) {
	p := NewPublisher(100*time.Millisecond, 10)
	subs := []chan interface{}{}
	subs = append(subs, p.Subscribe(), p.Subscribe(), p.Subscribe())
	p.Close()

	for _, c := range subs {
		if _, ok := <-c; ok {
			t.Fatal("expected all subscriber channels to be closed")
		}
	}
}
