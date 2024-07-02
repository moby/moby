package plugin

import "context"

// RequestPrivilegeFunc is a function interface that
// clients can supply to retry operations after
// getting an authorization error.
// This function returns the registry authentication
// header value in base 64 format, or an error
// if the privilege request fails.
type RequestPrivilegeFunc func(context.Context) (string, error)

// RemoveOptions holds parameters to remove plugins.
type RemoveOptions struct {
	Force bool
}

// EnableOptions holds parameters to enable plugins.
type EnableOptions struct {
	Timeout int
}

// DisableOptions holds parameters to disable plugins.
type DisableOptions struct {
	Force bool
}

// InstallOptions holds parameters to install a plugin.
type InstallOptions struct {
	Disabled              bool
	AcceptAllPermissions  bool
	RegistryAuth          string // RegistryAuth is the base64 encoded credentials for the registry
	RemoteRef             string // RemoteRef is the plugin name on the registry
	PrivilegeFunc         RequestPrivilegeFunc
	AcceptPermissionsFunc func(context.Context, Privileges) (bool, error)
	Args                  []string
}

// CreateOptions hold all options to plugin create.
type CreateOptions struct {
	RepoName string
}
