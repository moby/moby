package api

import (
	"testing"
)

func TestStatusLeader(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	status := c.Status()

	leader, err := status.Leader()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("Expected leader")
	}
}

func TestStatusPeers(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	status := c.Status()

	peers, err := status.Peers()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) == 0 {
		t.Fatalf("Expected peers ")
	}
}
