package cache

import (
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
)

type DescHandler struct {
	Provider       func(session.Group) content.Provider
	Progress       progress.Controller
	SnapshotLabels map[string]string
	Annotations    map[string]string
	Ref            string // string representation of desc origin, can be used as a sync key
}

type DescHandlers map[digest.Digest]*DescHandler

func descHandlersOf(opts ...RefOption) DescHandlers {
	for _, opt := range opts {
		if opt, ok := opt.(DescHandlers); ok {
			return opt
		}
	}
	return nil
}

type DescHandlerKey digest.Digest

type NeedsRemoteProviderError []digest.Digest //nolint:errname

func (m NeedsRemoteProviderError) Error() string {
	return fmt.Sprintf("missing descriptor handlers for lazy blobs %+v", []digest.Digest(m))
}

type ProgressKey struct{}

type Unlazy session.Group

func unlazySessionOf(opts ...RefOption) session.Group {
	for _, opt := range opts {
		if opt, ok := opt.(session.Group); ok {
			return opt
		}
	}
	return nil
}
