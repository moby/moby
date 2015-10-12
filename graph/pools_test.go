package graph

import (
	"testing"

	"github.com/docker/docker/pkg/broadcaster"
	"github.com/docker/docker/pkg/reexec"
)

func init() {
	reexec.Init()
}

func TestPools(t *testing.T) {
	s := &TagStore{
		pullingPool: make(map[string]*broadcaster.Buffered),
		pushingPool: make(map[string]*broadcaster.Buffered),
	}

	if _, found := s.poolAdd("pull", "test1"); found {
		t.Fatal("Expected pull test1 not to be in progress")
	}
	if _, found := s.poolAdd("pull", "test2"); found {
		t.Fatal("Expected pull test2 not to be in progress")
	}
	if _, found := s.poolAdd("push", "test1"); !found {
		t.Fatalf("Expected pull test1 to be in progress`")
	}
	if _, found := s.poolAdd("pull", "test1"); !found {
		t.Fatalf("Expected pull test1 to be in progress`")
	}
	if err := s.poolRemove("pull", "test2"); err != nil {
		t.Fatal(err)
	}
	if err := s.poolRemove("pull", "test2"); err != nil {
		t.Fatal(err)
	}
	if err := s.poolRemove("pull", "test1"); err != nil {
		t.Fatal(err)
	}
	if err := s.poolRemove("push", "test1"); err != nil {
		t.Fatal(err)
	}
}
