package distribution

import (
	"testing"
)

func TestPools(t *testing.T) {
	p := NewPool()

	if _, found := p.add("test1"); found {
		t.Fatal("Expected pull test1 not to be in progress")
	}
	if _, found := p.add("test2"); found {
		t.Fatal("Expected pull test2 not to be in progress")
	}
	if _, found := p.add("test1"); !found {
		t.Fatalf("Expected pull test1 to be in progress`")
	}
	if err := p.remove("test2"); err != nil {
		t.Fatal(err)
	}
	if err := p.remove("test2"); err != nil {
		t.Fatal(err)
	}
	if err := p.remove("test1"); err != nil {
		t.Fatal(err)
	}
}
