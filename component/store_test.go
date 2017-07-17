package component

import (
	"testing"
	"time"

	"context"

	"github.com/pkg/errors"
)

type testComponent struct{}

func TestStoreRegister(t *testing.T) {
	s := NewStore()

	c := testComponent{}
	cancel, err := s.Register("test", c)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Register("test", c)
	if errors.Cause(err) != existsErr {
		t.Fatal(err)
	}

	cancel()
	cancel, err = s.Register("test", c)
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	if _, err := s.Register("niltest", nil); errors.Cause(err) != nilComponentErr {
		t.Fatal(err)
	}
}

func TestStoreGet(t *testing.T) {
	s := NewStore()

	var c testComponent
	cancel, err := s.Register("test", c)
	if err != nil {
		t.Fatal(err)
	}

	service := s.Get("test")
	if service == nil {
		t.Fatal("expected non-nil service")
	}

	if service != c {
		t.Fatal("got wrong service after get")
	}

	cancel()
	service = s.Get("test")
	if service != nil {
		t.Fatal("expected nil service")
	}
}

func TestStoreWait(t *testing.T) {
	s := NewStore()

	ch := make(chan interface{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ch <- s.Wait(ctx, "test")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// make sure the wait is in place
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		default:
		}

		s.mu.Lock()
		ready := len(s.waiters["test"]) > 0
		s.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// nothing added yet, so there shouldn't be anything in this channel
	select {
	case <-ch:
		t.Fatal("wait returned unexpectedly")
	default:
	}

	var c testComponent
	_, err := s.Register("test", c)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case s := <-ch:
		if s != c {
			t.Fatalf("got unexpected service: %v", s)
		}
	}

	if len(s.waiters["test"]) != 0 {
		t.Fatalf("unpexected waiters: %d", len(s.waiters))
	}
}

func TestComponentTransoform(t *testing.T) {

}
