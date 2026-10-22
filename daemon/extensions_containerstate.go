// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"context"
	"slices"
	"time"

	"github.com/containerd/errdefs/pkg/errgrpc"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/filters"
	"github.com/moby/moby/v2/errdefs"
	containerstatev0 "github.com/moby/moby/v2/extpoints/containerstate/v0"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// containerLookupError keeps lookup error classification for in-process callers
// while also carrying a gRPC status for the callback server.
type containerLookupError struct{ error }

func (e containerLookupError) Unwrap() error { return e.error }

func (e containerLookupError) GRPCStatus() *status.Status {
	return status.Convert(errgrpc.ToGRPC(e.error))
}

type containerStateWatcher struct {
	daemon *Daemon
}

func (w containerStateWatcher) Watch(ctx context.Context, req *containerstatev0.WatchRequest) (*containerstatev0.WatchReply, error) {
	if req == nil || req.ContainerID == "" {
		return nil, errdefs.InvalidParameter(status.Error(codes.InvalidArgument, "container ID is required"))
	}
	for _, s := range req.Statuses {
		if s != containerstatev0.StatusRemoved && containertypes.ValidateContainerState(containertypes.ContainerState(s)) != nil {
			return nil, errdefs.InvalidParameter(status.Errorf(codes.InvalidArgument, "invalid status: %q", s))
		}
	}
	select {
	case <-w.daemon.startupDone:
	default:
		return nil, errdefs.Unavailable(status.Error(codes.Unavailable, "daemon startup is not complete"))
	}

	ctr, err := w.daemon.GetContainer(req.ContainerID)
	if err != nil {
		return nil, containerLookupError{err}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Exits and removal wake State.Wait; other status changes only log an
	// event. Both are set up before the state is read for the last time
	// before blocking, so those wakeups are not missed.
	_, evts := w.daemon.SubscribeToEvents(time.Time{}, time.Time{}, filters.NewArgs(
		filters.Arg("type", string(events.ContainerEventType)),
		filters.Arg("container", ctr.ID),
		filters.Arg("event", string(events.ActionStart)),
		filters.Arg("event", string(events.ActionPause)),
		filters.Arg("event", string(events.ActionUnPause)),
	))
	defer w.daemon.UnsubscribeFromEvents(evts)
	var exited <-chan container.StateStatus
	for {
		reply := w.reply(ctr)
		cur := reply.Current.Status
		// A removed container's state never changes again.
		if len(req.Statuses) == 0 || cur == containerstatev0.StatusRemoved || slices.Contains(req.Statuses, cur) {
			return reply, nil
		}
		if exited == nil {
			// A waiter stays registered until the container next stops or
			// is removed, so only register one when about to block.
			exited = ctr.State.Wait(ctx, containertypes.WaitConditionNextExit)
			continue
		}

		var recheck <-chan time.Time
		if cur == containerstatev0.StatusRemoving {
			// Resetting the removal guard, such as after unlinking, sends no
			// notification. Recheck until State provides status-change wakeups.
			recheck = time.After(100 * time.Millisecond)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-recheck:
		case _, ok := <-evts:
			if !ok {
				return nil, errdefs.Unavailable(status.Error(codes.Unavailable, "daemon is shutting down"))
			}
		case <-exited:
			// State.Wait also returns when ctx is done.
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			exited = nil
		}
	}
}

// reply returns the point's view of ctr's state.
func (w containerStateWatcher) reply(ctr *container.Container) *containerstatev0.WatchReply {
	ctr.Lock()
	defer ctr.Unlock()
	// The daemon unregisters a container before it marks it removed.
	// Hold the lock across both observations so removal cannot finish
	// between the membership check and the state snapshot.
	removed := w.daemon.containers.Get(ctr.ID) != ctr
	cs := containerState(ctr.State)
	if removed {
		cs.Status = containerstatev0.StatusRemoved
		// Removal clears the error, but it can be observed just before that.
		if cs.Exit != nil {
			cs.Exit.Error = ""
		}
	}
	return &containerstatev0.WatchReply{Current: cs}
}

// containerState returns the point's view of s.
// The caller must hold the state lock.
func containerState(s *container.State) *containerstatev0.ContainerState {
	status := s.State()
	cs := &containerstatev0.ContainerState{Status: string(status)}
	if s.Health != nil {
		if h := s.Health.Status(); h != "" {
			cs.Health = &containerstatev0.Health{Status: string(h)}
		}
	}
	// Only running and paused containers have not exited since they started;
	// a restarting container reports the exit that triggered the restart.
	running := status == containertypes.StateRunning || status == containertypes.StatePaused
	if !running && !s.FinishedAt.IsZero() {
		cs.Exit = &containerstatev0.Exit{
			ExitCode:  int64(s.ExitCode),
			Error:     s.ErrorMsg,
			OOMKilled: s.OOMKilled,
		}
	}
	return cs
}

var _ containerstatev0.ContainerStateWatcher = containerStateWatcher{}
