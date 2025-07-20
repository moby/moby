package types

import (
	"context"

	"github.com/docker/docker/api/types/common"
)

// IDResponse Response to an API call that returns just an Id.
//
// Deprecated: use either [container.CommitResponse] or [container.ExecCreateResponse]. It will be removed in the next release.
type IDResponse = common.IDResponse

// RequestPrivilegeFunc is a function interface that clients can supply to
// retry operations after getting an authorization error.
// This function returns the registry authentication header value in base64
// format, or an error if the privilege request fails.
//
// Deprecated: moved to [github.com/docker/docker/api/types/registry.RequestAuthConfig].
type RequestPrivilegeFunc func(context.Context) (string, error)
