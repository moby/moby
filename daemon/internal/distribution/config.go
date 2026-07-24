package distribution

import (
	"context"
	"encoding/json"
	"io"
	"runtime"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/internal/distribution/metadata"
	"github.com/moby/moby/v2/daemon/internal/distribution/xfer"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/moby/moby/v2/daemon/internal/progress"
	refstore "github.com/moby/moby/v2/daemon/internal/refstore"
	registrypkg "github.com/moby/moby/v2/daemon/pkg/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Config stores configuration for communicating
// with a registry.
type Config struct {
	// MetaHeaders stores HTTP headers with metadata about the image
	MetaHeaders map[string][]string
	// AuthConfig holds authentication credentials for authenticating with
	// the registry.
	AuthConfig *registry.AuthConfig
	// ProgressOutput is the interface for showing the status of the pull
	// operation.
	ProgressOutput progress.Output
	// RegistryService is the registry service to use for TLS configuration
	// and endpoint lookup.
	RegistryService RegistryResolver
	// ImageEventLogger notifies events for a given image
	ImageEventLogger func(ctx context.Context, id, name string, action events.Action)
	// MetadataStore is the storage backend for distribution-specific
	// metadata.
	MetadataStore metadata.Store
	// ImageStore manages images.
	ImageStore ImageConfigStore
	// ReferenceStore manages tags. This value is optional, when excluded
	// content will not be tagged.
	ReferenceStore refstore.Store
}

// ImagePullConfig stores pull configuration.
type ImagePullConfig struct {
	Config

	// DownloadManager manages concurrent pulls.
	DownloadManager *xfer.LayerDownloadManager
	// Schema2Types is an optional list of valid schema2 configuration types
	// allowed by the pull operation. If omitted, the default list of accepted
	// types is used.
	Schema2Types []string
	// Platform is the requested platform of the image being pulled
	Platform *ocispec.Platform
}

// ImagePushConfig stores push configuration.
type ImagePushConfig struct {
	Config

	// ConfigMediaType is the configuration media type for
	// schema2 manifests.
	ConfigMediaType string
	// LayerStores manages layers.
	LayerStores PushLayerProvider
	// ForceCheckLayerExist indicates force checking if layers exist in remote registry when pushing image layers
	ForceCheckLayerExist bool
	// UploadManager dispatches uploads.
	UploadManager *xfer.LayerUploadManager
}

// RegistryResolver is used for TLS configuration and endpoint lookup.
type RegistryResolver interface {
	ResolveAuthConfig(map[string]registry.AuthConfig, reference.Named) registry.AuthConfig
	LookupPushEndpoints(hostname string) (endpoints []registrypkg.APIEndpoint, err error)
	LookupPullEndpoints(hostname string) (endpoints []registrypkg.APIEndpoint, err error)
}

// ImageConfigStore handles storing and getting image configurations
// by digest. Allows getting an image configurations rootfs from the
// configuration.
type ImageConfigStore interface {
	Put(context.Context, []byte) (digest.Digest, error)
	Get(context.Context, digest.Digest) ([]byte, error)
}

// PushLayerProvider provides layers to be pushed by ChainID.
type PushLayerProvider interface {
	Get(layer.ChainID) (PushLayer, error)
}

// PushLayer is a pushable layer with metadata about the layer
// and access to the content of the layer.
type PushLayer interface {
	ChainID() layer.ChainID
	DiffID() layer.DiffID
	Parent() PushLayer
	Open() (io.ReadCloser, error)
	Size() int64
	MediaType() string
	Release()
}

type imageConfigStore struct {
	image.Store
}

// NewImageConfigStoreFromStore returns an ImageConfigStore backed
// by an image.Store for container images.
func NewImageConfigStoreFromStore(is image.Store) ImageConfigStore {
	return &imageConfigStore{
		Store: is,
	}
}

func (s *imageConfigStore) Put(_ context.Context, c []byte) (digest.Digest, error) {
	id, err := s.Store.Create(c)
	return digest.Digest(id), err
}

func (s *imageConfigStore) Get(_ context.Context, d digest.Digest) ([]byte, error) {
	img, err := s.Store.Get(image.ID(d))
	if err != nil {
		return nil, err
	}
	return img.RawJSON(), nil
}

func rootFSFromConfig(c []byte) (*image.RootFS, error) {
	var unmarshalledConfig image.Image
	if err := json.Unmarshal(c, &unmarshalledConfig); err != nil {
		return nil, err
	}
	return unmarshalledConfig.RootFS, nil
}

func platformFromConfig(c []byte) (*ocispec.Platform, error) {
	var unmarshalledConfig image.Image
	if err := json.Unmarshal(c, &unmarshalledConfig); err != nil {
		return nil, err
	}

	os := unmarshalledConfig.OS
	if os == "" {
		os = runtime.GOOS
	}
	if err := image.CheckOS(os); err != nil {
		return nil, errors.Wrapf(err, "image operating system %q cannot be used on this platform", os)
	}
	return &ocispec.Platform{
		OS:           os,
		Architecture: unmarshalledConfig.Architecture,
		Variant:      unmarshalledConfig.Variant,
		OSVersion:    unmarshalledConfig.OSVersion,
	}, nil
}

type storeLayerProvider struct {
	ls layer.Store
}

// NewLayerProvidersFromStore returns layer providers backed by
// an instance of LayerStore. Only getting layers as gzipped
// tars is supported.
func NewLayerProvidersFromStore(ls layer.Store) PushLayerProvider {
	return &storeLayerProvider{ls: ls}
}

func (p *storeLayerProvider) Get(lid layer.ChainID) (PushLayer, error) {
	if lid == "" {
		return &storeLayer{
			Layer: layer.EmptyLayer,
		}, nil
	}
	l, err := p.ls.Get(lid)
	if err != nil {
		return nil, err
	}

	sl := storeLayer{
		Layer: l,
		ls:    p.ls,
	}
	if d, ok := l.(distribution.Describable); ok {
		return &describableStoreLayer{
			storeLayer:  sl,
			describable: d,
		}, nil
	}

	return &sl, nil
}

type storeLayer struct {
	layer.Layer
	ls layer.Store
}

func (l *storeLayer) Parent() PushLayer {
	p := l.Layer.Parent()
	if p == nil {
		return nil
	}
	sl := storeLayer{
		Layer: p,
		ls:    l.ls,
	}
	if d, ok := p.(distribution.Describable); ok {
		return &describableStoreLayer{
			storeLayer:  sl,
			describable: d,
		}
	}

	return &sl
}

func (l *storeLayer) Open() (io.ReadCloser, error) {
	return l.Layer.TarStream()
}

func (l *storeLayer) Size() int64 {
	return l.Layer.DiffSize()
}

func (l *storeLayer) MediaType() string {
	// layer store always returns uncompressed tars
	return schema2.MediaTypeUncompressedLayer
}

func (l *storeLayer) Release() {
	if l.ls != nil {
		layer.ReleaseAndLog(l.ls, l.Layer)
	}
}

type describableStoreLayer struct {
	storeLayer
	describable distribution.Describable
}

func (l *describableStoreLayer) Descriptor() distribution.Descriptor {
	return l.describable.Descriptor()
}
