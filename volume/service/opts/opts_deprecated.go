package opts

import volume "github.com/docker/docker/api/types/backend/volume"

// CreateOption is used to pass options in when creating a volume
// Deprecated: Use [volume.CreateOption] instead
type CreateOption = volume.CreateOption

// CreateConfig is the set of config options that can be set when creating
// a volume
// Deprecated: Use [volume.CreateConfig] instead
type CreateConfig = volume.CreateConfig

// WithCreateLabel creates a CreateOption which adds a label with the given key/value pair
//
// Deprecated: Use [volume.WithCreateLabel] instead
var WithCreateLabel = volume.WithCreateLabel

// WithCreateLabels creates a CreateOption which sets the labels to the
// passed in value
//
// Deprecated: Use [volume.WithCreateLabels] instead
var WithCreateLabels = volume.WithCreateLabels

// WithCreateOptions creates a CreateOption which sets the options passed
// to the volume driver when creating a volume to the options passed in.
//
// Deprecated: Use [volume.WithCreateOptions] instead
var WithCreateOptions = volume.WithCreateOptions

// WithCreateReference creates a CreateOption which sets a reference to use
// when creating a volume. This ensures that the volume is created with a reference
// already attached to it to prevent race conditions with Create and volume cleanup.
//
// Deprecated: Use [volume.WithCreateReference] instead
var WithCreateReference = volume.WithCreateReference

// GetConfig is used with `GetOption` to set options for the volumes service's
// `Get` implementation.
//
// Deprecated: Use [volume.GetConfig] instead
type GetConfig = volume.GetConfig

// GetOption is passed to the service `Get` add extra details on the get request
// Deprecated: Use [volume.GetOption] instead
type GetOption = volume.GetOption

// WithGetDriver provides the driver to get the volume from
// If no driver is provided to `Get`, first the available metadata is checked
// to see which driver it belongs to, if that is not available all drivers are
// probed to find the volume.
//
// Deprecated: Use [volume.WithGetDriver] instead
var WithGetDriver = volume.WithGetDriver

// WithGetReference indicates to `Get` to increment the reference count for the
// retrieved volume with the provided reference ID.
//
// Deprecated: Use [volume.WithGetReference] instead
var WithGetReference = volume.WithGetReference

// WithGetResolveStatus indicates to `Get` to also fetch the volume status.
// This can cause significant overhead in the volume lookup.
//
// Deprecated: Use [volume.WithGetResolveStatus] instead
var WithGetResolveStatus = volume.WithGetResolveStatus

// RemoveConfig is used by `RemoveOption` to store config options for remove
//
// Deprecated: Use [volume.RemoveConfig] instead
type RemoveConfig = volume.RemoveConfig

// RemoveOption is used to pass options to the volumes service `Remove` implementation
//
// Deprecated: Use [volume.RemoveOption] instead
type RemoveOption = volume.RemoveOption

// WithPurgeOnError is an option passed to `Remove` which will purge all cached
// data about a volume even if there was an error while attempting to remove the
// volume.
//
// Deprecated: Use [volume.WithPurgeOnError] instead
var WithPurgeOnError = volume.WithPurgeOnError
