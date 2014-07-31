package server

import "testing"

func TestPools(t *testing.T) {
	srv := &Server{
		pullingPool: make(map[string]chan struct{}),
		pushingPool: make(map[string]chan struct{}),
	}

	if _, err := srv.poolAdd("pull", "test1"); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.poolAdd("pull", "test2"); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.poolAdd("push", "test1"); err == nil || err.Error() != "pull test1 is already in progress" {
		t.Fatalf("Expected `pull test1 is already in progress`")
	}
	if _, err := srv.poolAdd("pull", "test1"); err == nil || err.Error() != "pull test1 is already in progress" {
		t.Fatalf("Expected `pull test1 is already in progress`")
	}
	if _, err := srv.poolAdd("wait", "test3"); err == nil || err.Error() != "Unknown pool type" {
		t.Fatalf("Expected `Unknown pool type`")
	}
	if err := srv.poolRemove("pull", "test2"); err != nil {
		t.Fatal(err)
	}
	if err := srv.poolRemove("pull", "test2"); err != nil {
		t.Fatal(err)
	}
	if err := srv.poolRemove("pull", "test1"); err != nil {
		t.Fatal(err)
	}
	if err := srv.poolRemove("push", "test1"); err != nil {
		t.Fatal(err)
	}
	if err := srv.poolRemove("wait", "test3"); err == nil || err.Error() != "Unknown pool type" {
		t.Fatalf("Expected `Unknown pool type`")
	}
}
