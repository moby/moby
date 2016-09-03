package container

import (
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestNewMemoryStore(c *check.C) {
	ms := NewMemoryStore()
	m, ok := ms.(*memoryStore)
	if !ok {
		c.Fatalf("store is not a memory store %v", ms)
	}
	if m.s == nil {
		c.Fatal("expected store map to not be nil")
	}
}

func (s *DockerSuite) TestAddContainers(c *check.C) {
	ms := NewMemoryStore()
	ms.Add("id", NewBaseContainer("id", "root"))
	if ms.Size() != 1 {
		c.Fatalf("expected store size 1, got %v", ms.Size())
	}
}

func (s *DockerSuite) TestGetContainer(c *check.C) {
	ms := NewMemoryStore()
	ms.Add("id", NewBaseContainer("id", "root"))
	id := ms.Get("id")
	if id == nil {
		c.Fatal("expected container to not be nil")
	}
}

func (s *DockerSuite) TestDeleteContainer(c *check.C) {
	ms := NewMemoryStore()
	ms.Add("id", NewBaseContainer("id", "root"))
	ms.Delete("id")
	if id := ms.Get("id"); id != nil {
		c.Fatalf("expected container to be nil after removal, got %v", id)
	}

	if ms.Size() != 0 {
		c.Fatalf("expected store size to be 0, got %v", ms.Size())
	}
}

func (s *DockerSuite) TestListContainers(c *check.C) {
	ms := NewMemoryStore()

	cont := NewBaseContainer("id", "root")
	cont.Created = time.Now()
	cont2 := NewBaseContainer("id2", "root")
	cont2.Created = time.Now().Add(24 * time.Hour)

	ms.Add("id", cont)
	ms.Add("id2", cont2)

	list := ms.List()
	if len(list) != 2 {
		c.Fatalf("expected list size 2, got %v", len(list))
	}
	if list[0].ID != "id2" {
		c.Fatalf("expected older container to be first, got %v", list[0].ID)
	}
}

func (s *DockerSuite) TestFirstContainer(c *check.C) {
	ms := NewMemoryStore()

	ms.Add("id", NewBaseContainer("id", "root"))
	ms.Add("id2", NewBaseContainer("id2", "root"))

	first := ms.First(func(cont *Container) bool {
		return cont.ID == "id2"
	})

	if first == nil {
		c.Fatal("expected container to not be nil")
	}
	if first.ID != "id2" {
		c.Fatalf("expected id2, got %v", first)
	}
}

func (s *DockerSuite) TestApplyAllContainer(c *check.C) {
	ms := NewMemoryStore()

	ms.Add("id", NewBaseContainer("id", "root"))
	ms.Add("id2", NewBaseContainer("id2", "root"))

	ms.ApplyAll(func(cont *Container) {
		if cont.ID == "id2" {
			cont.ID = "newID"
		}
	})

	cont := ms.Get("id2")
	if cont == nil {
		c.Fatal("expected container to not be nil")
	}
	if cont.ID != "newID" {
		c.Fatalf("expected newID, got %v", cont)
	}
}
