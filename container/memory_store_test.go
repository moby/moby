package container

import (
	"testing"
	"time"
)

func TestNewMemoryStore(t *testing.T) {
	s := NewMemoryStore()
	m, ok := s.(*memoryStore)
	if !ok {
		t.Fatalf("store is not a memory store %v", s)
	}
	if m.s == nil {
		t.Fatal("expected store map to not be nil")
	}
}

func TestAddContainers(t *testing.T) {
	s := NewMemoryStore()
	s.Add("id", NewBaseContainer("id", "root"))
	if s.Size() != 1 {
		t.Fatalf("expected store size 1, got %v", s.Size())
	}
}

func TestGetContainer(t *testing.T) {
	s := NewMemoryStore()
	s.Add("id", NewBaseContainer("id", "root"))
	c := s.Get("id")
	if c == nil {
		t.Fatal("expected container to not be nil")
	}
}

func TestDeleteContainer(t *testing.T) {
	s := NewMemoryStore()
	s.Add("id", NewBaseContainer("id", "root"))
	s.Delete("id")
	if c := s.Get("id"); c != nil {
		t.Fatalf("expected container to be nil after removal, got %v", c)
	}

	if s.Size() != 0 {
		t.Fatalf("expected store size to be 0, got %v", s.Size())
	}
}

func TestListContainers(t *testing.T) {
	s := NewMemoryStore()

	cont := NewBaseContainer("id", "root")
	cont.Created = time.Now()
	cont2 := NewBaseContainer("id2", "root")
	cont2.Created = time.Now().Add(24 * time.Hour)

	s.Add("id", cont)
	s.Add("id2", cont2)

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected list size 2, got %v", len(list))
	}
	if list[0].ID != "id2" {
		t.Fatalf("expected older container to be first, got %v", list[0].ID)
	}
}

func TestFirstContainer(t *testing.T) {
	s := NewMemoryStore()

	s.Add("id", NewBaseContainer("id", "root"))
	s.Add("id2", NewBaseContainer("id2", "root"))

	first := s.First(func(cont *Container) bool {
		return cont.ID == "id2"
	})

	if first == nil {
		t.Fatal("expected container to not be nil")
	}
	if first.ID != "id2" {
		t.Fatalf("expected id2, got %v", first)
	}
}

func TestApplyAllContainer(t *testing.T) {
	s := NewMemoryStore()

	s.Add("id", NewBaseContainer("id", "root"))
	s.Add("id2", NewBaseContainer("id2", "root"))

	s.ApplyAll(func(cont *Container) error {
		if cont.ID == "id2" {
			cont.ID = "newID"
		}
		return nil
	})

	cont := s.Get("id2")
	if cont == nil {
		t.Fatal("expected container to not be nil")
	}
	if cont.ID != "newID" {
		t.Fatalf("expected newID, got %v", cont)
	}
}

func TestReduceAll(t *testing.T) {
	s := NewMemoryStore()

	c1 := NewBaseContainer("id", "root")
	c1.Created = time.Now()
	c1.SetRunning(1234)

	s.Add("id", c1)

	var copies []*Container
	all := func(c *Container) bool {
		return true
	}
	store := func(c *Container) error {
		copies = append(copies, c)
		return nil
	}

	if err := s.ReduceAll(all, store); err != nil {
		t.Fatal(err)
	}

	if len(copies) != 1 {
		t.Fatalf("expected 1 copies, got %v", len(copies))
	}

	cp1 := copies[0]

	if cp1 == c1 {
		t.Fatalf("expected container structure to be a copy, not original pointer")
	}
	if cp1.Created != c1.Created {
		t.Fatalf("expected same created times, original %v, got %v", c1.Created, cp1.Created)
	}
	if !cp1.IsRunning() {
		t.Fatalf("expected state to be running, got %s", cp1.StateString())
	}
	if cp1.Pid != 1234 {
		t.Fatalf("expected PID 1234, got %v", cp1.Pid)
	}
	if cp1.State == c1.State {
		t.Fatalf("expected state structure to be a copy, not original pointer")
	}
}

func TestReduceOne(t *testing.T) {
	s := NewMemoryStore()

	c1 := NewBaseContainer("id", "root")
	c1.Created = time.Now()
	c1.SetRunning(1234)

	s.Add("id", c1)
	var checked bool
	store := func(cp1 *Container) error {
		checked = true
		if cp1 == c1 {
			t.Fatalf("expected container structure to be a copy, not original pointer")
		}
		if cp1.Created != c1.Created {
			t.Fatalf("expected same created times, original %v, got %v", c1.Created, cp1.Created)
		}
		if !cp1.IsRunning() {
			t.Fatalf("expected state to be running, got %s", cp1.StateString())
		}
		if cp1.Pid != 1234 {
			t.Fatalf("expected PID 1234, got %v", cp1.Pid)
		}
		if cp1.State == c1.State {
			t.Fatalf("expected state structure to be a copy, not original pointer")
		}
		return nil
	}

	if err := s.ReduceOne("id", store); err != nil {
		t.Fatal(err)
	}

	if !checked {
		t.Fatal("expected checked container copy, got false")
	}

	if err := s.ReduceOne("unknown", store); err != nil {
		t.Fatal(err)
	}
}
