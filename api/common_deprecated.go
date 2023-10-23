package api

import "github.com/docker/docker/api/types/versions"

// Common constants for daemon and client.
const (
	// DefaultVersion of Current REST API.
	//
	// Deprecated: use [versions.DefaultVersion].
	DefaultVersion = versions.DefaultVersion

	// MinVersion represents Minimum REST API version supported.
	//
	// Deprecated: use [versions.MinVersion].
	MinVersion = versions.MinVersion
)
