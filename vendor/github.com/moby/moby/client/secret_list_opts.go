package client

import "github.com/moby/moby/api/types/filters"

// SecretListOptions holds parameters to list secrets
type SecretListOptions struct {
	Filters filters.Args
}
