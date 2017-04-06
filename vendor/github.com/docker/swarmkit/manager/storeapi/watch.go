package storeapi

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
)

// Watch starts a stream that returns any changes to objects that match
// the specified selectors. When the stream begins, it immediately sends
// an empty message back to the client. It is important to wait for
// this message before taking any actions that depend on an established
// stream of changes for consistency.
func (s *Server) Watch(request *api.WatchRequest, stream api.Store_WatchServer) error {
	ctx := stream.Context()

	watchArgs, err := api.ConvertWatchArgs(request.Entries)
	if err != nil {
		return grpc.Errorf(codes.InvalidArgument, "%s", err.Error())
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
