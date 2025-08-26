package swarmbackend

import "github.com/moby/moby/api/types/filters"

type ConfigListOptions struct {
	Filters filters.Args
}

type NodeListOptions struct {
	Filters filters.Args
}

type TaskListOptions struct {
	Filters filters.Args
}

type UpdateFlags struct {
	RotateWorkerToken      bool
	RotateManagerToken     bool
	RotateManagerUnlockKey bool
}

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
}

type ServiceListOptions struct {
	Filters filters.Args

	// Status indicates whether the server should include the service task
	// count of running and desired tasks.
	Status bool
}

// SecretListOptions holds parameters to list secrets
type SecretListOptions struct {
	Filters filters.Args
}
