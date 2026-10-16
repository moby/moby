// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/container"
	daemonevents "github.com/moby/moby/v2/daemon/events"
	containerstatev0 "github.com/moby/moby/v2/extpoints/containerstate/v0"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

func newContainerStateTestDaemon(t *testing.T, state *container.State) (*Daemon, *container.Container) {
	t.Helper()

	ctr := &container.Container{ID: "container-state-test-id", Config: &containertypes.Config{}, State: state}
	store := container.NewMemoryStore()
	store.Add(ctr.ID, ctr)
	replica, err := container.NewViewDB()
	assert.NilError(t, err)
	assert.NilError(t, replica.Save(ctr))

	d := &Daemon{
		containers:        store,
		containersReplica: replica,
		EventsService:     daemonevents.New(),
		startupDone:       make(chan struct{}),
	}
	close(d.startupDone)
	return d, ctr
}

type containerStateWaitResult struct {
	reply *containerstatev0.WatchReply
	err   error
}

func TestContainerStateReplyLocksRemovalSnapshot(t *testing.T) {
	t.Parallel()

	d, ctr := newContainerStateTestDaemon(t, &container.State{RemovalInProgress: true, Dead: true})
	d.containers = removalDuringStateLookupStore{d.containers}

	reply := containerStateWatcher{daemon: d}.reply(ctr)
	assert.Equal(t, reply.Current.Status, containerstatev0.StatusRemoving)
}

type removalDuringStateLookupStore struct{ container.Store }

func (s removalDuringStateLookupStore) Get(id string) *container.Container {
	ctr := s.Store.Get(id)
	// Complete removal after the membership lookup if the snapshot leaves
	// that window unlocked. The reply must not combine stale membership
	// with the state after SetRemoved clears RemovalInProgress.
	if ctr != nil && ctr.TryLock() {
		ctr.Unlock()
		s.Store.Delete(id)
		ctr.State.SetRemoved()
	}
	return ctr
}

func startContainerStateWatch(ctx context.Context, d *Daemon, id string, statuses ...string) <-chan containerStateWaitResult {
	waited := make(chan containerStateWaitResult, 1)
	go func() {
		reply, err := containerStateWatcher{daemon: d}.Watch(ctx, &containerstatev0.WatchRequest{
			ContainerID: id,
			Statuses:    statuses,
		})
		waited <- containerStateWaitResult{reply, err}
	}()
	return waited
}

func TestContainerStateWatchSkipsRestart(t *testing.T) {
	t.Parallel()

	d, ctr := newContainerStateTestDaemon(t, &container.State{Running: true, StartedAt: time.Now()})

	// Start waiting while the container is between restart-policy attempts.
	ctr.Lock()
	ctr.State.SetRestarting(&container.ExitStatus{ExitCode: 23})
	ctr.Unlock()
	waited := startContainerStateWatch(t.Context(), d, ctr.ID, containerstatev0.StatusExited)
	assertWaiting := func() {
		select {
		case <-waited:
			t.Fatal("Watch returned before the final exit")
		case <-time.After(20 * time.Millisecond):
		}
	}
	assertWaiting()

	ctr.Lock()
	ctr.State.SetRunning(nil, nil, time.Now())
	d.LogContainerEvent(ctr, events.ActionStart)
	ctr.Unlock()
	assertWaiting()

	// The final exit logs no event when the restart manager gives up.
	ctr.Lock()
	ctr.State.SetError(errors.New("final exit error"))
	ctr.State.SetStopped(&container.ExitStatus{ExitCode: 42})
	ctr.Unlock()

	select {
	case res := <-waited:
		assert.NilError(t, res.err)
		assert.DeepEqual(t, res.reply.Current, &containerstatev0.ContainerState{
			Status: containerstatev0.StatusExited,
			Exit:   &containerstatev0.Exit{ExitCode: 42, Error: "final exit error"},
		})
	case <-time.After(3 * time.Second):
		t.Fatal("Watch did not return after the final exit")
	}
}

func TestContainerStateWatchEndsAfterRemovalGuardReset(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		d, ctr := newContainerStateTestDaemon(t, &container.State{
			StartedAt:  time.Now().Add(-time.Minute),
			FinishedAt: time.Now(),
			ExitCode:   42,
		})
		ctr.State.SetRemovalInProgress()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		nextExit := ctr.State.Wait(ctx, containertypes.WaitConditionNextExit)
		waited := startContainerStateWatch(ctx, d, ctr.ID, containerstatev0.StatusExited)

		// Let the watch read "removing" and block before the guard is reset,
		// as happens when link removal finishes without a container exit.
		synctest.Wait()
		ctr.State.ResetRemovalInProgress()

		select {
		case res := <-waited:
			assert.NilError(t, res.err)
			assert.DeepEqual(t, res.reply.Current, &containerstatev0.ContainerState{
				Status: containerstatev0.StatusExited,
				Exit:   &containerstatev0.Exit{ExitCode: 42},
			})
		case <-time.After(time.Second):
			t.Fatal("Watch did not return after the removal guard was reset")
		}
		select {
		case <-nextExit:
			t.Fatal("resetting the removal guard notified a next-exit waiter")
		default:
		}
	})
}

func TestContainerStateWatchWakesOnEvent(t *testing.T) {
	t.Parallel()

	d, ctr := newContainerStateTestDaemon(t, &container.State{Running: true, StartedAt: time.Now()})
	waited := startContainerStateWatch(t.Context(), d, ctr.ID, containerstatev0.StatusPaused)

	// Pausing does not wake State.Wait, only the event does.
	ctr.Lock()
	ctr.State.Paused = true
	d.LogContainerEvent(ctr, events.ActionPause)
	ctr.Unlock()

	select {
	case res := <-waited:
		assert.NilError(t, res.err)
		assert.DeepEqual(t, res.reply.Current, &containerstatev0.ContainerState{Status: containerstatev0.StatusPaused})
	case <-time.After(3 * time.Second):
		t.Fatal("Watch did not return after the pause event")
	}
}

func TestContainerStateWatchEndsOnRemoval(t *testing.T) {
	t.Parallel()

	health := &container.Health{}
	health.SetStatus(containertypes.Healthy)
	d, ctr := newContainerStateTestDaemon(t, &container.State{Running: true, StartedAt: time.Now(), Health: health})
	var waits []<-chan containerStateWaitResult
	for _, statuses := range [][]string{
		// Removal ends a watch that does not list "removed".
		{containerstatev0.StatusPaused},
		{containerstatev0.StatusRemoved},
	} {
		waits = append(waits, startContainerStateWatch(t.Context(), d, ctr.ID, statuses...))
	}
	// Removal must happen after the watches found the container.
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if d.EventsService.SubscribersCount() < len(waits) {
			return poll.Continue("watches did not start")
		}
		return poll.Success()
	})
	ctr.Lock()
	ctr.State.OOMKilled = true
	ctr.State.SetError(errors.New("exit error"))
	ctr.State.SetStopped(&container.ExitStatus{ExitCode: 7})
	ctr.Unlock()
	health.SetStatus(containertypes.Unhealthy)
	d.containers.Delete(ctr.ID)
	d.containersReplica.Delete(ctr)
	ctr.State.SetRemoved()

	for _, waited := range waits {
		select {
		case res := <-waited:
			assert.NilError(t, res.err)
			// Removal clears the recorded error.
			assert.DeepEqual(t, res.reply.Current, &containerstatev0.ContainerState{
				Status: containerstatev0.StatusRemoved,
				Health: &containerstatev0.Health{Status: containerstatev0.HealthUnhealthy},
				Exit:   &containerstatev0.Exit{ExitCode: 7, OOMKilled: true},
			})
		case <-time.After(3 * time.Second):
			t.Fatal("Watch was not ended by removal")
		}
	}

	_, err := containerStateWatcher{daemon: d}.Watch(t.Context(), &containerstatev0.WatchRequest{ContainerID: ctr.ID})
	assert.Assert(t, cerrdefs.IsNotFound(err))
}

func TestContainerStateWatchReturnsImmediately(t *testing.T) {
	t.Parallel()

	d, ctr := newContainerStateTestDaemon(t, &container.State{Running: true, StartedAt: time.Now()})
	for _, statuses := range [][]string{
		nil,
		{containerstatev0.StatusExited, containerstatev0.StatusRunning},
	} {
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		reply, err := containerStateWatcher{daemon: d}.Watch(ctx, &containerstatev0.WatchRequest{
			ContainerID: ctr.ID,
			Statuses:    statuses,
		})
		cancel()
		assert.NilError(t, err)
		assert.Equal(t, reply.Current.Status, containerstatev0.StatusRunning)
	}
}

func TestContainerStateWatchErrors(t *testing.T) {
	t.Parallel()

	d, _ := newContainerStateTestDaemon(t, &container.State{})
	_, err := containerStateWatcher{daemon: d}.Watch(t.Context(), &containerstatev0.WatchRequest{ContainerID: "unknown-container"})
	assert.Assert(t, cerrdefs.IsNotFound(err))
	assert.Equal(t, status.Code(err), codes.NotFound)

	_, err = containerStateWatcher{daemon: d}.Watch(t.Context(), &containerstatev0.WatchRequest{ContainerID: "any", Statuses: []string{"exitted"}})
	assert.Assert(t, cerrdefs.IsInvalidArgument(err))
	assert.Equal(t, status.Code(err), codes.InvalidArgument)

	starting := &Daemon{containers: container.NewMemoryStore(), startupDone: make(chan struct{})}
	_, err = containerStateWatcher{daemon: starting}.Watch(t.Context(), &containerstatev0.WatchRequest{ContainerID: "any"})
	assert.Assert(t, cerrdefs.IsUnavailable(err))
	assert.Equal(t, status.Code(err), codes.Unavailable)
}
