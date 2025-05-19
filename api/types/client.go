package types // import "github.com/docker/docker/api/types"

import (
	"bufio"
	"context"
	"net"

	"github.com/docker/docker/api/types/filters"
)

// NewHijackedResponse initializes a [HijackedResponse] type.
func NewHijackedResponse(conn net.Conn, mediaType string) HijackedResponse {
	return HijackedResponse{Conn: conn, Reader: bufio.NewReader(conn), mediaType: mediaType}
}

// HijackedResponse holds connection information for a hijacked request.
type HijackedResponse struct {
	mediaType string
	Conn      net.Conn
	Reader    *bufio.Reader
}

// Close closes the hijacked connection and reader.
func (h *HijackedResponse) Close() {
	h.Conn.Close()
}

// MediaType let client know if HijackedResponse hold a raw or multiplexed stream.
// returns false if HTTP Content-Type is not relevant, and container must be inspected
func (h *HijackedResponse) MediaType() (string, bool) {
	if h.mediaType == "" {
		return "", false
	}
	return h.mediaType, true
}

// CloseWriter is an interface that implements structs
// that close input streams to prevent from writing.
type CloseWriter interface {
	CloseWrite() error
}

// CloseWrite closes a readWriter for writing.
func (h *HijackedResponse) CloseWrite() error {
	if conn, ok := h.Conn.(CloseWriter); ok {
		return conn.CloseWrite()
	}
	return nil
}

// NodeListOptions holds parameters to list nodes with.
type NodeListOptions struct {
	Filters filters.Args
}

// NodeRemoveOptions holds parameters to remove nodes with.
type NodeRemoveOptions struct {
	Force bool
}

// ServiceCreateOptions contains the options to use when creating a service.
type ServiceCreateOptions struct {
	// EncodedRegistryAuth is the encoded registry authorization credentials to
	// use when updating the service.
	//
	// This field follows the format of the X-Registry-Auth header.
	EncodedRegistryAuth string

	// QueryRegistry indicates whether the service update requires
	// contacting a registry. A registry may be contacted to retrieve
	// the image digest and manifest, which in turn can be used to update
	// platform or other information about the service.
	QueryRegistry bool
}

// Values for RegistryAuthFrom in ServiceUpdateOptions
const (
	RegistryAuthFromSpec         = "spec"
	RegistryAuthFromPreviousSpec = "previous-spec"
)

// ServiceUpdateOptions contains the options to be used for updating services.
type ServiceUpdateOptions struct {
	// EncodedRegistryAuth is the encoded registry authorization credentials to
	// use when updating the service.
	//
	// This field follows the format of the X-Registry-Auth header.
	EncodedRegistryAuth string

	// TODO(stevvooe): Consider moving the version parameter of ServiceUpdate
	// into this field. While it does open API users up to racy writes, most
	// users may not need that level of consistency in practice.

	// RegistryAuthFrom specifies where to find the registry authorization
	// credentials if they are not given in EncodedRegistryAuth. Valid
	// values are "spec" and "previous-spec".
	RegistryAuthFrom string

	// Rollback indicates whether a server-side rollback should be
	// performed. When this is set, the provided spec will be ignored.
	// The valid values are "previous" and "none". An empty value is the
	// same as "none".
	Rollback string

	// QueryRegistry indicates whether the service update requires
	// contacting a registry. A registry may be contacted to retrieve
	// the image digest and manifest, which in turn can be used to update
	// platform or other information about the service.
	QueryRegistry bool
}

// TaskListOptions holds parameters to list tasks with.
type TaskListOptions struct {
	Filters filters.Args
}

// PluginRemoveOptions holds parameters to remove plugins.
type PluginRemoveOptions struct {
	Force bool
}

// PluginEnableOptions holds parameters to enable plugins.
type PluginEnableOptions struct {
	Timeout int
}

// PluginDisableOptions holds parameters to disable plugins.
type PluginDisableOptions struct {
	Force bool
}

// PluginInstallOptions holds parameters to install a plugin.
type PluginInstallOptions struct {
	Disabled             bool
	AcceptAllPermissions bool
	RegistryAuth         string // RegistryAuth is the base64 encoded credentials for the registry
	RemoteRef            string // RemoteRef is the plugin name on the registry

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/docker/docker/api/types/registry.RequestAuthConfig].
	PrivilegeFunc         func(context.Context) (string, error)
	AcceptPermissionsFunc func(context.Context, PluginPrivileges) (bool, error)
	Args                  []string
}

// SwarmUnlockKeyResponse contains the response for Engine API:
// GET /swarm/unlockkey
type SwarmUnlockKeyResponse struct {
	// UnlockKey is the unlock key in ASCII-armored format.
	UnlockKey string
}

// PluginCreateOptions hold all options to plugin create.
type PluginCreateOptions struct {
	RepoName string
}
