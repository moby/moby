package watchapi

import (
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/state"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Watch starts a stream that returns any changes to objects that match
// the specified selectors. When the stream begins, it immediately sends
// an empty message back to the client. It is important to wait for
// this message before taking any actions that depend on an established
// stream of changes for consistency.
func (s *Server) Watch(request *api.WatchRequest, stream api.Watch_WatchServer) error {
	ctx := stream.Context()

	s.mu.Lock()
	pctx := s.pctx
	s.mu.Unlock()
	if pctx == nil {
		return errNotRunning
	}

	watchArgs, err := api.ConvertWatchArgs(request.Entries)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	}

	watchArgs = append(watchArgs, state.EventCommit{})
	watch, cancel, err := store.WatchFrom(s.store, request.ResumeFrom, watchArgs...)
	if err != nil {
		return err
	}
	defer cancel()

	// TODO(aaronl): Send current version in this WatchMessage?
	if err := stream.Send(&api.WatchMessage{}); err != nil {
		return err
	}

	var events []*api.WatchMessage_Event
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pctx.Done():
			return pctx.Err()
		case event := <-watch:
			if commitEvent, ok := event.(state.EventCommit); ok && len(events) > 0 {
				if err := stream.Send(&api.WatchMessage{Events: events, Version: commitEvent.Version}); err != nil {
					return err
				}
				events = nil
			} else if eventMessage := api.WatchMessageEvent(event.(api.Event)); eventMessage != nil {
				if !request.IncludeOldObject {
					eventMessage.OldObject = nil
				}
				events = append(events, eventMessage)
			}
		}
	}
}
