package container

import "testing"

func TestViewSave(t *testing.T) {
	db, err := NewViewDB()
	if err != nil {
		t.Fatal(err)
	}
	c := NewBaseContainer("id", "root")
	if err := c.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
}

func TestViewAll(t *testing.T) {
	var (
		db, _ = NewViewDB()
		one   = NewBaseContainer("id1", "root1")
		two   = NewBaseContainer("id2", "root2")
	)
	one.Pid = 10
	two.Pid = 20
	one.CheckpointTo(db)
	two.CheckpointTo(db)
	all, err := db.Snapshot().All()
	if err != nil {
		t.Fatal(err)
	}
	if l := len(all); l != 2 {
		t.Fatalf("expected 2 items, got %d", l)
	}
	byID := make(map[string]Snapshot)
	for i := range all {
		byID[all[i].ID] = all[i]
	}
	if s, ok := byID["id1"]; !ok || s.Pid != 10 {
		t.Fatalf("expected something different with for id1: %v", s)
	}
	if s, ok := byID["id2"]; !ok || s.Pid != 20 {
		t.Fatalf("expected something different with for id1: %v", s)
	}
}

func TestViewGet(t *testing.T) {
	db, _ := NewViewDB()
	one := NewBaseContainer("id", "root")
	one.ImageID = "some-image-123"
	one.CheckpointTo(db)
	s, err := db.Snapshot().Get("id")
	if err != nil {
		t.Fatal(err)
	}
	if s == nil || s.ImageID != "some-image-123" {
		t.Fatalf("expected something different. Got: %v", s)
	}
}
