package pubsub

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestSendToOneSub(c *check.C) {
	p := NewPublisher(100*time.Millisecond, 10)
	ss := p.Subscribe()

	p.Publish("hi")

	msg := <-ss
	if msg.(string) != "hi" {
		c.Fatalf("expected message hi but received %v", msg)
	}
}

func (s *DockerSuite) TestSendToMultipleSubs(c *check.C) {
	p := NewPublisher(100*time.Millisecond, 10)
	subs := []chan interface{}{}
	subs = append(subs, p.Subscribe(), p.Subscribe(), p.Subscribe())

	p.Publish("hi")

	for _, sb := range subs {
		msg := <-sb
		if msg.(string) != "hi" {
			c.Fatalf("expected message hi but received %v", msg)
		}
	}
}

func (s *DockerSuite) TestEvictOneSub(c *check.C) {
	p := NewPublisher(100*time.Millisecond, 10)
	s1 := p.Subscribe()
	s2 := p.Subscribe()

	p.Evict(s1)
	p.Publish("hi")
	if _, ok := <-s1; ok {
		c.Fatal("expected s1 to not receive the published message")
	}

	msg := <-s2
	if msg.(string) != "hi" {
		c.Fatalf("expected message hi but received %v", msg)
	}
}

func (s *DockerSuite) TestClosePublisher(c *check.C) {
	p := NewPublisher(100*time.Millisecond, 10)
	subs := []chan interface{}{}
	subs = append(subs, p.Subscribe(), p.Subscribe(), p.Subscribe())
	p.Close()

	for _, sb := range subs {
		if _, ok := <-sb; ok {
			c.Fatal("expected all subscriber channels to be closed")
		}
	}
}

const sampleText = "test"

type testSubscriber struct {
	dataCh chan interface{}
	ch     chan error
}

func (s *testSubscriber) Wait() error {
	return <-s.ch
}

func newTestSubscriber(p *Publisher) *testSubscriber {
	ts := &testSubscriber{
		dataCh: p.Subscribe(),
		ch:     make(chan error),
	}
	go func() {
		for data := range ts.dataCh {
			s, ok := data.(string)
			if !ok {
				ts.ch <- fmt.Errorf("Unexpected type %T", data)
				break
			}
			if s != sampleText {
				ts.ch <- fmt.Errorf("Unexpected text %s", s)
				break
			}
		}
		close(ts.ch)
	}()
	return ts
}

// for testing with -race
func (s *DockerSuite) TestPubSubRace(c *check.C) {
	p := NewPublisher(0, 1024)
	var subs [](*testSubscriber)
	for j := 0; j < 50; j++ {
		subs = append(subs, newTestSubscriber(p))
	}
	for j := 0; j < 1000; j++ {
		p.Publish(sampleText)
	}
	time.AfterFunc(1*time.Second, func() {
		for _, sb := range subs {
			p.Evict(sb.dataCh)
		}
	})
	for _, sb := range subs {
		sb.Wait()
	}
}

func (s *DockerSuite) BenchmarkPubSub(c *check.C) {
	for i := 0; i < c.N; i++ {
		c.StopTimer()
		p := NewPublisher(0, 1024)
		var subs [](*testSubscriber)
		for j := 0; j < 50; j++ {
			subs = append(subs, newTestSubscriber(p))
		}
		c.StartTimer()
		for j := 0; j < 1000; j++ {
			p.Publish(sampleText)
		}
		time.AfterFunc(1*time.Second, func() {
			for _, sb := range subs {
				p.Evict(sb.dataCh)
			}
		})
		for _, sb := range subs {
			if err := sb.Wait(); err != nil {
				c.Fatal(err)
			}
		}
	}
}
