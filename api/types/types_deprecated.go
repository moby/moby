package types

import (
	"context"

	"github.com/docker/docker/api/types/common"
	"github.com/docker/docker/api/types/container"
)

// IDResponse Response to an API call that returns just an Id.
//
// Deprecated: use either [container.CommitResponse] or [container.ExecCreateResponse]. It will be removed in the next release.
type IDResponse = common.IDResponse

// ContainerJSONBase contains response of Engine API GET "/containers/{name:.*}/json"
// for API version 1.18 and older.
//
// Deprecated: use [container.InspectResponse] or [container.ContainerJSONBase]. It will be removed in the next release.
type ContainerJSONBase = container.ContainerJSONBase

// ContainerJSON is the response for the GET "/containers/{name:.*}/json"
// endpoint.
//
// Deprecated: use [container.InspectResponse]. It will be removed in the next release.
type ContainerJSON = container.InspectResponse

// Container contains response of Engine API:
// GET "/containers/json"
//
// Deprecated: use [container.Summary].
type Container = container.Summary

// RequestPrivilegeFunc is a function interface that clients can supply to
// retry operations after getting an authorization error.
// This function returns the registry authentication header value in base64
// format, or an error if the privilege request fails.
//
// Deprecated: moved to [github.com/docker/docker/api/types/registry.RequestAuthConfig].
type RequestPrivilegeFunc func(context.Context) (string, error)
