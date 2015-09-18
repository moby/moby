package graph

import (
	"sync"
	"testing"

	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/reexec"
)

func init() {
	reexec.Init()
}

func TestPools(t *testing.T) {
	s := &TagStore{
		pullsByKey:      make(map[string]*progressreader.Broadcaster),
		pushesByKey:     make(map[string]*progressreader.Broadcaster),
		pullCountByRepo: make(map[string]int),
		pushCountByRepo: make(map[string]int),
	}

	s.pushPullCond = sync.NewCond(s)

	if _, found := s.acquirePull("test1", "test1", nil); found {
		t.Fatal("Expected pull test1 not to be in progress")
	}
	if _, found := s.acquirePull("test1", "test1", nil); !found {
		t.Fatal("Expected pull test1 to be in progress")
	}
	if _, found := s.acquirePull("test2", "test2", nil); found {
		t.Fatal("Expected pull test2 not to be in progress")
	}

	callbackCalled := false
	releasePullFunc := func() {
		s.releasePull("test1", "test1")
		callbackCalled = true
	}
	if _, found := s.acquirePush("test1", "test1", releasePullFunc); found || !callbackCalled {
		t.Fatalf("Expected pull test1 to be released`")
	}

	callbackCalled = false
	releasePushFunc := func() {
		s.releasePush("test1", "test1")
		callbackCalled = true
	}
	if _, found := s.acquirePull("test1", "test1", releasePushFunc); found || !callbackCalled {
		t.Fatalf("Expected pull test1 to be in progress")
	}
}
