package memberlist

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

// CheckInteg will skip a test if integration testing is not enabled.
func CheckInteg(t *testing.T) {
	if !IsInteg() {
		t.SkipNow()
	}
}

// IsInteg returns a boolean telling you if we're in integ testing mode.
func IsInteg() bool {
	return os.Getenv("INTEG_TESTS") != ""
}

// Tests the memberlist by creating a cluster of 100 nodes
// and checking that we get strong convergence of changes.
func TestMemberlist_Integ(t *testing.T) {
	CheckInteg(t)

	num := 16
	var members []*Memberlist

	secret := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	eventCh := make(chan NodeEvent, num)

	addr := "127.0.0.1"
	for i := 0; i < num; i++ {
		c := DefaultLANConfig()
		c.Name = fmt.Sprintf("%s:%d", addr, 12345+i)
		c.BindAddr = addr
		c.BindPort = 12345 + i
		c.ProbeInterval = 20 * time.Millisecond
		c.ProbeTimeout = 100 * time.Millisecond
		c.GossipInterval = 20 * time.Millisecond
		c.PushPullInterval = 200 * time.Millisecond
		c.SecretKey = secret

		if i == 0 {
			c.Events = &ChannelEventDelegate{eventCh}
		}

		m, err := Create(c)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		members = append(members, m)
		defer m.Shutdown()

		if i > 0 {
			last := members[i-1]
			num, err := m.Join([]string{last.config.Name})
			if num == 0 || err != nil {
				t.Fatalf("unexpected err: %s", err)
			}
		}
	}

	// Wait and print debug info
	breakTimer := time.After(250 * time.Millisecond)
WAIT:
	for {
		select {
		case e := <-eventCh:
			if e.Event == NodeJoin {
				log.Printf("[DEBUG] Node join: %v (%d)", *e.Node, members[0].NumMembers())
			} else {
				log.Printf("[DEBUG] Node leave: %v (%d)", *e.Node, members[0].NumMembers())
			}
		case <-breakTimer:
			break WAIT
		}
	}

	for idx, m := range members {
		got := m.NumMembers()
		if got != num {
			t.Errorf("bad num members at idx %d. Expected %d. Got %d.",
				idx, num, got)
		}
	}
}
