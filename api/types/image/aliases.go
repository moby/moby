package image

import (
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// DeleteResponse delete response
type DeleteResponse = image.DeleteResponse

// Metadata contains engine-local data about the image.
type Metadata = image.Metadata

// PruneReport contains the response for Engine API:
// POST "/images/prune"
type PruneReport = image.PruneReport

// LoadResponse returns information to the client about a load process.
//
// TODO(thaJeztah): remove this type, and just use an io.ReadCloser
//
// This type was added in https://github.com/moby/moby/pull/18878, related
// to https://github.com/moby/moby/issues/19177;
//
// Make docker load to output json when the response content type is json
// Swarm hijacks the response from docker load and returns JSON rather
// than plain text like the Engine does. This makes the API library to return
// information to figure that out.
//
// However the "load" endpoint unconditionally returns JSON;
// https://github.com/moby/moby/blob/7b9d2ef6e5518a3d3f3cc418459f8df786cfbbd1/api/server/router/image/image_routes.go#L248-L255
//
// PR https://github.com/moby/moby/pull/21959 made the response-type depend
// on whether "quiet" was set, but this logic got changed in a follow-up
// https://github.com/moby/moby/pull/25557, which made the JSON response-type
// unconditionally, but the output produced depend on whether"quiet" was set.
//
// We should deprecated the "quiet" option, as it's really a client
// responsibility.
type LoadResponse = client.LoadResponse

// HistoryResponseItem individual image layer information in response to ImageHistory operation
type HistoryResponseItem = image.HistoryResponseItem

// RootFS returns Image's RootFS description including the layer IDs.
type RootFS = image.RootFS

// InspectResponse contains response of Engine API:
// GET "/images/{name:.*}/json"
type InspectResponse = image.InspectResponse

type ManifestKind = image.ManifestKind

const (
	ManifestKindImage       = image.ManifestKindImage
	ManifestKindAttestation = image.ManifestKindAttestation
	ManifestKindUnknown     = image.ManifestKindUnknown
)

type ManifestSummary = image.ManifestSummary

type ImageProperties = image.ImageProperties

type AttestationProperties = image.AttestationProperties

// ImportSource holds source information for ImageImport
type ImportSource = client.ImageImportSource

// ImportOptions holds information to import images from the client host.
type ImportOptions = client.ImageImportOptions

// CreateOptions holds information to create images.
type CreateOptions = client.ImageCreateOptions

// PullOptions holds information to pull images.
type PullOptions = client.ImagePullOptions

// PushOptions holds information to push images.
type PushOptions = client.ImagePushOptions

// ListOptions holds parameters to list images with.
type ListOptions = client.ImageListOptions

// RemoveOptions holds parameters to remove images.
type RemoveOptions = client.ImageRemoveOptions

// HistoryOptions holds parameters to get image history.
//
// FIXME(thaJeztah): no longer exported in client: either deprecate or export in client
type HistoryOptions struct {
	// Platform from the manifest list to use for history.
	Platform *ocispec.Platform
}

// LoadOptions holds parameters to load images.
//
// FIXME(thaJeztah): no longer exported in client: either deprecate or export in client
type LoadOptions struct {
	// Quiet suppresses progress output
	Quiet bool

	// Platforms selects the platforms to load if the image is a
	// multi-platform image and has multiple variants.
	Platforms []ocispec.Platform
}

// ImageInspectOptions holds parameters to inspect images.
//
// FIXME(thaJeztah): no longer exported in client: either deprecate or export in client
type ImageInspectOptions struct {
	// Manifests returns the image manifests.
	Manifests bool

	// Platform selects the specific platform of a multi-platform image to inspect.
	//
	// This option is only available for API version 1.49 and up.
	Platform *ocispec.Platform
}

// SaveOptions holds parameters to save images.
//
// FIXME(thaJeztah): no longer exported in client: either deprecate or export in client
type SaveOptions struct {
	// Platforms selects the platforms to save if the image is a
	// multi-platform image and has multiple variants.
	Platforms []ocispec.Platform
}

type Summary = image.Summary
