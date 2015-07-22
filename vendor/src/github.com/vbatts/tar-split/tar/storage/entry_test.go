package storage

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestEntries(t *testing.T) {
	e := Entries{
		Entry{
			Type:     SegmentType,
			Payload:  []byte("y'all"),
			Position: 1,
		},
		Entry{
			Type:     SegmentType,
			Payload:  []byte("doin"),
			Position: 3,
		},
		Entry{
			Type:     FileType,
			Name:     "./hurr.txt",
			Payload:  []byte("deadbeef"),
			Position: 2,
		},
		Entry{
			Type:     SegmentType,
			Payload:  []byte("how"),
			Position: 0,
		},
	}
	sort.Sort(e)
	if e[0].Position != 0 {
		t.Errorf("expected Position 0, but got %d", e[0].Position)
	}
}

func TestFile(t *testing.T) {
	f := Entry{
		Type:     FileType,
		Name:     "./hello.txt",
		Size:     100,
		Position: 2,
	}

	buf, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	f1 := Entry{}
	if err = json.Unmarshal(buf, &f1); err != nil {
		t.Fatal(err)
	}

	if f.Name != f1.Name {
		t.Errorf("expected Name %q, got %q", f.Name, f1.Name)
	}
	if f.Size != f1.Size {
		t.Errorf("expected Size %q, got %q", f.Size, f1.Size)
	}
	if f.Position != f1.Position {
		t.Errorf("expected Position %q, got %q", f.Position, f1.Position)
	}
}
