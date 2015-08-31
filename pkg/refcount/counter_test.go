package refcount

import (
	"sync"
	"testing"
)

func TestSimpleIncrDecr(t *testing.T) {
	const k = "test"
	c := New()
	c.Incr(k)
	count := c.Count(k)
	if count != 1 {
		t.Fatalf("count should be 1 but received %d", count)
	}
	c.Decr(k)
	count = c.Count(k)
	if count != 0 {
		t.Fatalf("count should be 0 but received %d", count)
	}
}

func TestConcurrentIncr(t *testing.T) {
	const k = "test"
	c := New()
	g := sync.WaitGroup{}
	g.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer g.Done()
			c.Incr(k)
		}()
	}
	g.Wait()
	count := c.Count(k)
	if count != 5 {
		t.Fatalf("count should be 5 but received %d", count)
	}
}

func TestSetGetValue(t *testing.T) {
	const k = "test"
	value := 3
	c := New()
	c.Set(k, value)
	gotit, v := c.Get(k)
	if !gotit {
		t.Fatal("should have received a value for key")
	}
	if v == nil {
		t.Fatal("value should not be nil for key")
	}
	three, ok := v.(int)
	if !ok {
		t.Fatal("wrong value type for key")
	}
	if three != 3 {
		t.Fatalf("expected value to be 3 but received %d", three)
	}
}

func TestGetIncr(t *testing.T) {
	const k = "test"
	value := 3
	c := New()
	c.Set(k, value)
	c.Incr(k)
	gotit, v := c.GetIncr(k)
	if !gotit {
		t.Fatal("should have received a value for key")
	}
	if v == nil {
		t.Fatal("value should not be nil for key")
	}
	three, ok := v.(int)
	if !ok {
		t.Fatal("wrong value type for key")
	}
	if three != 3 {
		t.Fatalf("expected value to be 3 but received %d", three)
	}
	count := c.Count(k)
	if count != 2 {
		t.Fatalf("expected count to be 2 but received %d", count)
	}
}

func TestSetIncr(t *testing.T) {
	const k = "test"
	value := 3
	c := New()
	c.SetIncr(k, value)
	gotit, v := c.Get(k)
	if !gotit {
		t.Fatal("should have received a value for key")
	}
	if v == nil {
		t.Fatal("value should not be nil for key")
	}
	three, ok := v.(int)
	if !ok {
		t.Fatal("wrong value type for key")
	}
	if three != 3 {
		t.Fatalf("expected value to be 3 but received %d", three)
	}
	count := c.Count(k)
	if count != 1 {
		t.Fatalf("expected count to be 1 but received %d", count)
	}
}

func TestDelete(t *testing.T) {
	const k = "test"
	value := 3
	c := New()
	c.Set(k, value)
	gotit, _ := c.Get(k)
	if !gotit {
		t.Fatal("value was not added for key")
	}
	c.Delete(k)
	gotit, _ = c.Get(k)
	if gotit {
		t.Fatal("value still exists for key")
	}
}
