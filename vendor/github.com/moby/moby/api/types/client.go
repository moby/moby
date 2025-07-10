package types

import (
	"bufio"
	"context"
	"net"
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

// PluginCreateOptions hold all options to plugin create.
type PluginCreateOptions struct {
	RepoName string
}
