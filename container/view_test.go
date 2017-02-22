package container

import "testing"

func TestViewSave(t *testing.T) {
	db, err := NewMemDB()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := NewBaseContainer("id", "root").Snapshot()
	if err := db.Save(snapshot); err != nil {
		t.Fatal(err)

	}
}

func TestViewAll(t *testing.T) {
	var (
		db, _ = NewMemDB()
		one   = NewBaseContainer("id1", "root1").Snapshot()
		two   = NewBaseContainer("id2", "root2").Snapshot()
	)
	one.Pid = 10
	two.Pid = 20
	db.Save(one)
	db.Save(two)
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
	db, _ := NewMemDB()
	one := NewBaseContainer("id", "root")
	one.ImageID = "some-image-123"
	db.Save(one.Snapshot())
	s, err := db.Snapshot().Get("id")
	if err != nil {
		t.Fatal(err)
	}
	if s == nil || s.ImageID != "some-image-123" {
		t.Fatalf("expected something different. Got: %v", s)
	}
}
