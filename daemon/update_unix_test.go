//go:build !windows

package daemon

import (
	"sync"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/events"
)

// TestUpdateConcurrentRollbackRace exercises Daemon.update with two
// concurrent updates of the same container: one that succeeds, and one
// that fails inside UpdateContainer and rolls back to the backup taken
// at the start of update. Run with -race: the backup copy must be taken
// while holding the container lock, otherwise it races with the field
// writes of the concurrent update, and the rollback can restore a torn
// HostConfig.
func TestUpdateConcurrentRollbackRace(t *testing.T) {
	ctr := &container.Container{
		ID:     "5a4ff6a163ad4533d22d69a2b8960bf7fafdcba06e72d2febdba229008b0bf57",
		Name:   "update_race",
		Root:   t.TempDir(),
		State:  &container.State{},
		Config: &containertypes.Config{},
		HostConfig: &containertypes.HostConfig{
			// CPUPeriod is already set, so an update carrying
			// NanoCPUs is rejected by UpdateContainer and takes
			// the rollback path.
			Resources: containertypes.Resources{CPUPeriod: 100000},
		},
	}

	store := container.NewMemoryStore()
	store.Add(ctr.ID, ctr)
	containersReplica, err := container.NewViewDB()
	if err != nil {
		t.Fatalf("could not create ViewDB: %v", err)
	}
	d := &Daemon{
		containers:        store,
		containersReplica: containersReplica,
		EventsService:     events.New(),
	}
	d.reserveName(ctr.ID, ctr.Name)

	const iterations = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	worker := func(hc *containertypes.HostConfig) {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			// An error is expected for the conflicting update;
			// the point of the test is the absence of a data
			// race, not the outcome of each update.
			_ = d.update(ctr.Name, hc)
		}
	}

	wg.Add(2)
	go worker(&containertypes.HostConfig{Resources: containertypes.Resources{CPUShares: 512}})
	go worker(&containertypes.HostConfig{Resources: containertypes.Resources{NanoCPUs: 500000000}})
	close(start)
	wg.Wait()
}
