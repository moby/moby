/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package containerd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	contentapi "github.com/containerd/containerd/api/services/content/v1"
	diffapi "github.com/containerd/containerd/api/services/diff/v1"
	eventsapi "github.com/containerd/containerd/api/services/events/v1"
	imagesapi "github.com/containerd/containerd/api/services/images/v1"
	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	leasesapi "github.com/containerd/containerd/api/services/leases/v1"
	namespacesapi "github.com/containerd/containerd/api/services/namespaces/v1"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/api/services/tasks/v1"
	versionservice "github.com/containerd/containerd/api/services/version/v1"
	apitypes "github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	contentproxy "github.com/containerd/containerd/content/proxy"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	leasesproxy "github.com/containerd/containerd/leases/proxy"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/dialer"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/services/introspection"
	"github.com/containerd/containerd/snapshots"
	snproxy "github.com/containerd/containerd/snapshots/proxy"
	"github.com/containerd/typeurl"
	ptypes "github.com/gogo/protobuf/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func init() {
	const prefix = "types.containerd.io"
	// register TypeUrls for commonly marshaled external types
	major := strconv.Itoa(specs.VersionMajor)
	typeurl.Register(&specs.Spec{}, prefix, "opencontainers/runtime-spec", major, "Spec")
	typeurl.Register(&specs.Process{}, prefix, "opencontainers/runtime-spec", major, "Process")
	typeurl.Register(&specs.LinuxResources{}, prefix, "opencontainers/runtime-spec", major, "LinuxResources")
	typeurl.Register(&specs.WindowsResources{}, prefix, "opencontainers/runtime-spec", major, "WindowsResources")
}

// New returns a new containerd client that is connected to the containerd
// instance provided by address
func New(address string, opts ...ClientOpt) (*Client, error) {
	var copts clientOpts
	for _, o := range opts {
		if err := o(&copts); err != nil {
			return nil, err
		}
	}
	if copts.timeout == 0 {
		copts.timeout = 10 * time.Second
	}

	c := &Client{
		defaultns: copts.defaultns,
	}

	if copts.defaultRuntime != "" {
		c.runtime = copts.defaultRuntime
	} else {
		c.runtime = defaults.DefaultRuntime
	}

	if copts.defaultPlatform != nil {
		c.platform = copts.defaultPlatform
	} else {
		c.platform = platforms.Default()
	}

	if copts.services != nil {
		c.services = *copts.services
	}
	if address != "" {
		backoffConfig := backoff.DefaultConfig
		backoffConfig.MaxDelay = 3 * time.Second
		connParams := grpc.ConnectParams{
			Backoff: backoffConfig,
		}
		gopts := []grpc.DialOption{
			grpc.WithBlock(),
			grpc.WithInsecure(),
			grpc.FailOnNonTempDialError(true),
			grpc.WithConnectParams(connParams),
			grpc.WithContextDialer(dialer.ContextDialer),
			grpc.WithReturnConnectionError(),

			// TODO(stevvooe): We may need to allow configuration of this on the client.
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
			grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
		}
		if len(copts.dialOptions) > 0 {
			gopts = copts.dialOptions
		}
		if copts.defaultns != "" {
			unary, stream := newNSInterceptors(copts.defaultns)
			gopts = append(gopts,
				grpc.WithUnaryInterceptor(unary),
				grpc.WithStreamInterceptor(stream),
			)
		}
		connector := func() (*grpc.ClientConn, error) {
			ctx, cancel := context.WithTimeout(context.Background(), copts.timeout)
			defer cancel()
			conn, err := grpc.DialContext(ctx, dialer.DialAddress(address), gopts...)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to dial %q", address)
			}
			return conn, nil
		}
		conn, err := connector()
		if err != nil {
			return nil, err
		}
		c.conn, c.connector = conn, connector
	}
	if copts.services == nil && c.conn == nil {
		return nil, errors.Wrap(errdefs.ErrUnavailable, "no grpc connection or services is available")
	}

	// check namespace labels for default runtime
	if copts.defaultRuntime == "" && c.defaultns != "" {
		if label, err := c.GetLabel(context.Background(), defaults.DefaultRuntimeNSLabel); err != nil {
			return nil, err
		} else if label != "" {
			c.runtime = label
		}
	}

	return c, nil
}

// NewWithConn returns a new containerd client that is connected to the containerd
// instance provided by the connection
func NewWithConn(conn *grpc.ClientConn, opts ...ClientOpt) (*Client, error) {
	var copts clientOpts
	for _, o := range opts {
		if err := o(&copts); err != nil {
			return nil, err
		}
	}
	c := &Client{
		defaultns: copts.defaultns,
		conn:      conn,
		runtime:   fmt.Sprintf("%s.%s", plugin.RuntimePlugin, runtime.GOOS),
	}

	// check namespace labels for default runtime
	if copts.defaultRuntime == "" && c.defaultns != "" {
		if label, err := c.GetLabel(context.Background(), defaults.DefaultRuntimeNSLabel); err != nil {
			return nil, err
		} else if label != "" {
			c.runtime = label
		}
	}

	if copts.services != nil {
		c.services = *copts.services
	}
	return c, nil
}

// Client is the client to interact with containerd and its various services
// using a uniform interface
type Client struct {
	services
	connMu    sync.Mutex
	conn      *grpc.ClientConn
	runtime   string
	defaultns string
	platform  platforms.MatchComparer
	connector func() (*grpc.ClientConn, error)
}

// Reconnect re-establishes the GRPC connection to the containerd daemon
func (c *Client) Reconnect() error {
	if c.connector == nil {
		return errors.Wrap(errdefs.ErrUnavailable, "unable to reconnect to containerd, no connector available")
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	c.conn.Close()
	conn, err := c.connector()
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Runtime returns the name of the runtime being used
func (c *Client) Runtime() string {
	return c.runtime
}

// IsServing returns true if the client can successfully connect to the
// containerd daemon and the healthcheck service returns the SERVING
// response.
// This call will block if a transient error is encountered during
// connection. A timeout can be set in the context to ensure it returns
// early.
func (c *Client) IsServing(ctx context.Context) (bool, error) {
	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		return false, errors.Wrap(errdefs.ErrUnavailable, "no grpc connection available")
	}
	c.connMu.Unlock()
	r, err := c.HealthService().Check(ctx, &grpc_health_v1.HealthCheckRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return false, err
	}
	return r.Status == grpc_health_v1.HealthCheckResponse_SERVING, nil
}

// Containers returns all containers created in containerd
func (c *Client) Containers(ctx context.Context, filters ...string) ([]Container, error) {
	r, err := c.ContainerService().List(ctx, filters...)
	if err != nil {
		return nil, err
	}
	var out []Container
	for _, container := range r {
		out = append(out, containerFromRecord(c, container))
	}
	return out, nil
}

// NewContainer will create a new container in container with the provided id
// the id must be unique within the namespace
func (c *Client) NewContainer(ctx context.Context, id string, opts ...NewContainerOpts) (Container, error) {
	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	container := containers.Container{
		ID: id,
		Runtime: containers.RuntimeInfo{
			Name: c.runtime,
		},
	}
	for _, o := range opts {
		if err := o(ctx, c, &container); err != nil {
			return nil, err
		}
	}
	r, err := c.ContainerService().Create(ctx, container)
	if err != nil {
		return nil, err
	}
	return containerFromRecord(c, r), nil
}

// LoadContainer loads an existing container from metadata
func (c *Client) LoadContainer(ctx context.Context, id string) (Container, error) {
	r, err := c.ContainerService().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return containerFromRecord(c, r), nil
}

// RemoteContext is used to configure object resolutions and transfers with
// remote content stores and image providers.
type RemoteContext struct {
	// Resolver is used to resolve names to objects, fetchers, and pushers.
	// If no resolver is provided, defaults to Docker registry resolver.
	Resolver remotes.Resolver

	// PlatformMatcher is used to match the platforms for an image
	// operation and define the preference when a single match is required
	// from multiple platforms.
	PlatformMatcher platforms.MatchComparer

	// Unpack is done after an image is pulled to extract into a snapshotter.
	// It is done simultaneously for schema 2 images when they are pulled.
	// If an image is not unpacked on pull, it can be unpacked any time
	// afterwards. Unpacking is required to run an image.
	Unpack bool

	// UnpackOpts handles options to the unpack call.
	UnpackOpts []UnpackOpt

	// Snapshotter used for unpacking
	Snapshotter string

	// SnapshotterOpts are additional options to be passed to a snapshotter during pull
	SnapshotterOpts []snapshots.Opt

	// Labels to be applied to the created image
	Labels map[string]string

	// BaseHandlers are a set of handlers which get are called on dispatch.
	// These handlers always get called before any operation specific
	// handlers.
	BaseHandlers []images.Handler

	// HandlerWrapper wraps the handler which gets sent to dispatch.
	// Unlike BaseHandlers, this can run before and after the built
	// in handlers, allowing operations to run on the descriptor
	// after it has completed transferring.
	HandlerWrapper func(images.Handler) images.Handler

	// ConvertSchema1 is whether to convert Docker registry schema 1
	// manifests. If this option is false then any image which resolves
	// to schema 1 will return an error since schema 1 is not supported.
	ConvertSchema1 bool

	// Platforms defines which platforms to handle when doing the image operation.
	// Platforms is ignored when a PlatformMatcher is set, otherwise the
	// platforms will be used to create a PlatformMatcher with no ordering
	// preference.
	Platforms []string

	// MaxConcurrentDownloads is the max concurrent content downloads for each pull.
	MaxConcurrentDownloads int

	// MaxConcurrentUploadedLayers is the max concurrent uploaded layers for each push.
	MaxConcurrentUploadedLayers int

	// AllMetadata downloads all manifests and known-configuration files
	AllMetadata bool

	// ChildLabelMap sets the labels used to reference child objects in the content
	// store. By default, all GC reference labels will be set for all fetched content.
	ChildLabelMap func(ocispec.Descriptor) []string
}

func defaultRemoteContext() *RemoteContext {
	return &RemoteContext{
		Resolver: docker.NewResolver(docker.ResolverOptions{
			Client: http.DefaultClient,
		}),
	}
}

// Fetch downloads the provided content into containerd's content store
// and returns a non-platform specific image reference
func (c *Client) Fetch(ctx context.Context, ref string, opts ...RemoteOpt) (images.Image, error) {
	fetchCtx := defaultRemoteContext()
	for _, o := range opts {
		if err := o(c, fetchCtx); err != nil {
			return images.Image{}, err
		}
	}

	if fetchCtx.Unpack {
		return images.Image{}, errors.Wrap(errdefs.ErrNotImplemented, "unpack on fetch not supported, try pull")
	}

	if fetchCtx.PlatformMatcher == nil {
		if len(fetchCtx.Platforms) == 0 {
			fetchCtx.PlatformMatcher = platforms.All
		} else {
			var ps []ocispec.Platform
			for _, s := range fetchCtx.Platforms {
				p, err := platforms.Parse(s)
				if err != nil {
					return images.Image{}, errors.Wrapf(err, "invalid platform %s", s)
				}
				ps = append(ps, p)
			}

			fetchCtx.PlatformMatcher = platforms.Any(ps...)
		}
	}

	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return images.Image{}, err
	}
	defer done(ctx)

	img, err := c.fetch(ctx, fetchCtx, ref, 0)
	if err != nil {
		return images.Image{}, err
	}
	return c.createNewImage(ctx, img)
}

// Push uploads the provided content to a remote resource
func (c *Client) Push(ctx context.Context, ref string, desc ocispec.Descriptor, opts ...RemoteOpt) error {
	pushCtx := defaultRemoteContext()
	for _, o := range opts {
		if err := o(c, pushCtx); err != nil {
			return err
		}
	}
	if pushCtx.PlatformMatcher == nil {
		if len(pushCtx.Platforms) > 0 {
			var ps []ocispec.Platform
			for _, platform := range pushCtx.Platforms {
				p, err := platforms.Parse(platform)
				if err != nil {
					return errors.Wrapf(err, "invalid platform %s", platform)
				}
				ps = append(ps, p)
			}
			pushCtx.PlatformMatcher = platforms.Any(ps...)
		} else {
			pushCtx.PlatformMatcher = platforms.All
		}
	}

	// Annotate ref with digest to push only push tag for single digest
	if !strings.Contains(ref, "@") {
		ref = ref + "@" + desc.Digest.String()
	}

	pusher, err := pushCtx.Resolver.Pusher(ctx, ref)
	if err != nil {
		return err
	}

	var wrapper func(images.Handler) images.Handler

	if len(pushCtx.BaseHandlers) > 0 {
		wrapper = func(h images.Handler) images.Handler {
			h = images.Handlers(append(pushCtx.BaseHandlers, h)...)
			if pushCtx.HandlerWrapper != nil {
				h = pushCtx.HandlerWrapper(h)
			}
			return h
		}
	} else if pushCtx.HandlerWrapper != nil {
		wrapper = pushCtx.HandlerWrapper
	}

	var limiter *semaphore.Weighted
	if pushCtx.MaxConcurrentUploadedLayers > 0 {
		limiter = semaphore.NewWeighted(int64(pushCtx.MaxConcurrentUploadedLayers))
	}

	return remotes.PushContent(ctx, pusher, desc, c.ContentStore(), limiter, pushCtx.PlatformMatcher, wrapper)
}

// GetImage returns an existing image
func (c *Client) GetImage(ctx context.Context, ref string) (Image, error) {
	i, err := c.ImageService().Get(ctx, ref)
	if err != nil {
		return nil, err
	}
	return NewImage(c, i), nil
}

// ListImages returns all existing images
func (c *Client) ListImages(ctx context.Context, filters ...string) ([]Image, error) {
	imgs, err := c.ImageService().List(ctx, filters...)
	if err != nil {
		return nil, err
	}
	images := make([]Image, len(imgs))
	for i, img := range imgs {
		images[i] = NewImage(c, img)
	}
	return images, nil
}

// Restore restores a container from a checkpoint
func (c *Client) Restore(ctx context.Context, id string, checkpoint Image, opts ...RestoreOpts) (Container, error) {
	store := c.ContentStore()
	index, err := decodeIndex(ctx, store, checkpoint.Target())
	if err != nil {
		return nil, err
	}

	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	copts := []NewContainerOpts{}
	for _, o := range opts {
		copts = append(copts, o(ctx, id, c, checkpoint, index))
	}

	ctr, err := c.NewContainer(ctx, id, copts...)
	if err != nil {
		return nil, err
	}

	return ctr, nil
}

func writeIndex(ctx context.Context, index *ocispec.Index, client *Client, ref string) (d ocispec.Descriptor, err error) {
	labels := map[string]string{}
	for i, m := range index.Manifests {
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i)] = m.Digest.String()
	}
	data, err := json.Marshal(index)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return writeContent(ctx, client.ContentStore(), ocispec.MediaTypeImageIndex, ref, bytes.NewReader(data), content.WithLabels(labels))
}

// GetLabel gets a label value from namespace store
// If there is no default label, an empty string returned with nil error
func (c *Client) GetLabel(ctx context.Context, label string) (string, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		if c.defaultns == "" {
			return "", err
		}
		ns = c.defaultns
	}

	srv := c.NamespaceService()
	labels, err := srv.Labels(ctx, ns)
	if err != nil {
		return "", err
	}

	value := labels[label]
	return value, nil
}

// Subscribe to events that match one or more of the provided filters.
//
// Callers should listen on both the envelope and errs channels. If the errs
// channel returns nil or an error, the subscriber should terminate.
//
// The subscriber can stop receiving events by canceling the provided context.
// The errs channel will be closed and return a nil error.
func (c *Client) Subscribe(ctx context.Context, filters ...string) (ch <-chan *events.Envelope, errs <-chan error) {
	return c.EventService().Subscribe(ctx, filters...)
}

// Close closes the clients connection to containerd
func (c *Client) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// NamespaceService returns the underlying Namespaces Store
func (c *Client) NamespaceService() namespaces.Store {
	if c.namespaceStore != nil {
		return c.namespaceStore
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewNamespaceStoreFromClient(namespacesapi.NewNamespacesClient(c.conn))
}

// ContainerService returns the underlying container Store
func (c *Client) ContainerService() containers.Store {
	if c.containerStore != nil {
		return c.containerStore
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewRemoteContainerStore(containersapi.NewContainersClient(c.conn))
}

// ContentStore returns the underlying content Store
func (c *Client) ContentStore() content.Store {
	if c.contentStore != nil {
		return c.contentStore
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return contentproxy.NewContentStore(contentapi.NewContentClient(c.conn))
}

// SnapshotService returns the underlying snapshotter for the provided snapshotter name
func (c *Client) SnapshotService(snapshotterName string) snapshots.Snapshotter {
	snapshotterName, err := c.resolveSnapshotterName(context.Background(), snapshotterName)
	if err != nil {
		snapshotterName = DefaultSnapshotter
	}
	if c.snapshotters != nil {
		return c.snapshotters[snapshotterName]
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return snproxy.NewSnapshotter(snapshotsapi.NewSnapshotsClient(c.conn), snapshotterName)
}

// TaskService returns the underlying TasksClient
func (c *Client) TaskService() tasks.TasksClient {
	if c.taskService != nil {
		return c.taskService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return tasks.NewTasksClient(c.conn)
}

// ImageService returns the underlying image Store
func (c *Client) ImageService() images.Store {
	if c.imageStore != nil {
		return c.imageStore
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewImageStoreFromClient(imagesapi.NewImagesClient(c.conn))
}

// DiffService returns the underlying Differ
func (c *Client) DiffService() DiffService {
	if c.diffService != nil {
		return c.diffService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewDiffServiceFromClient(diffapi.NewDiffClient(c.conn))
}

// IntrospectionService returns the underlying Introspection Client
func (c *Client) IntrospectionService() introspection.Service {
	if c.introspectionService != nil {
		return c.introspectionService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return introspection.NewIntrospectionServiceFromClient(introspectionapi.NewIntrospectionClient(c.conn))
}

// LeasesService returns the underlying Leases Client
func (c *Client) LeasesService() leases.Manager {
	if c.leasesService != nil {
		return c.leasesService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return leasesproxy.NewLeaseManager(leasesapi.NewLeasesClient(c.conn))
}

// HealthService returns the underlying GRPC HealthClient
func (c *Client) HealthService() grpc_health_v1.HealthClient {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return grpc_health_v1.NewHealthClient(c.conn)
}

// EventService returns the underlying event service
func (c *Client) EventService() EventService {
	if c.eventService != nil {
		return c.eventService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewEventServiceFromClient(eventsapi.NewEventsClient(c.conn))
}

// VersionService returns the underlying VersionClient
func (c *Client) VersionService() versionservice.VersionClient {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return versionservice.NewVersionClient(c.conn)
}

// Conn returns the underlying GRPC connection object
func (c *Client) Conn() *grpc.ClientConn {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn
}

// Version of containerd
type Version struct {
	// Version number
	Version string
	// Revision from git that was built
	Revision string
}

// Version returns the version of containerd that the client is connected to
func (c *Client) Version(ctx context.Context) (Version, error) {
	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		return Version{}, errors.Wrap(errdefs.ErrUnavailable, "no grpc connection available")
	}
	c.connMu.Unlock()
	response, err := c.VersionService().Version(ctx, &ptypes.Empty{})
	if err != nil {
		return Version{}, err
	}
	return Version{
		Version:  response.Version,
		Revision: response.Revision,
	}, nil
}

// ServerInfo represents the introspected server information
type ServerInfo struct {
	UUID string
}

// Server returns server information from the introspection service
func (c *Client) Server(ctx context.Context) (ServerInfo, error) {
	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		return ServerInfo{}, errors.Wrap(errdefs.ErrUnavailable, "no grpc connection available")
	}
	c.connMu.Unlock()

	response, err := c.IntrospectionService().Server(ctx, &ptypes.Empty{})
	if err != nil {
		return ServerInfo{}, err
	}
	return ServerInfo{
		UUID: response.UUID,
	}, nil
}

func (c *Client) resolveSnapshotterName(ctx context.Context, name string) (string, error) {
	if name == "" {
		label, err := c.GetLabel(ctx, defaults.DefaultSnapshotterNSLabel)
		if err != nil {
			return "", err
		}

		if label != "" {
			name = label
		} else {
			name = DefaultSnapshotter
		}
	}

	return name, nil
}

func (c *Client) getSnapshotter(ctx context.Context, name string) (snapshots.Snapshotter, error) {
	name, err := c.resolveSnapshotterName(ctx, name)
	if err != nil {
		return nil, err
	}

	s := c.SnapshotService(name)
	if s == nil {
		return nil, errors.Wrapf(errdefs.ErrNotFound, "snapshotter %s was not found", name)
	}

	return s, nil
}

// CheckRuntime returns true if the current runtime matches the expected
// runtime. Providing various parts of the runtime schema will match those
// parts of the expected runtime
func CheckRuntime(current, expected string) bool {
	cp := strings.Split(current, ".")
	l := len(cp)
	for i, p := range strings.Split(expected, ".") {
		if i > l {
			return false
		}
		if p != cp[i] {
			return false
		}
	}
	return true
}

// GetSnapshotterSupportedPlatforms returns a platform matchers which represents the
// supported platforms for the given snapshotters
func (c *Client) GetSnapshotterSupportedPlatforms(ctx context.Context, snapshotterName string) (platforms.MatchComparer, error) {
	filters := []string{fmt.Sprintf("type==%s, id==%s", plugin.SnapshotPlugin, snapshotterName)}
	in := c.IntrospectionService()

	resp, err := in.Plugins(ctx, filters)
	if err != nil {
		return nil, err
	}

	if len(resp.Plugins) <= 0 {
		return nil, fmt.Errorf("inspection service could not find snapshotter %s plugin", snapshotterName)
	}

	sn := resp.Plugins[0]
	snPlatforms := toPlatforms(sn.Platforms)
	return platforms.Any(snPlatforms...), nil
}

func toPlatforms(pt []apitypes.Platform) []ocispec.Platform {
	platforms := make([]ocispec.Platform, len(pt))
	for i, p := range pt {
		platforms[i] = ocispec.Platform{
			Architecture: p.Architecture,
			OS:           p.OS,
			Variant:      p.Variant,
		}
	}
	return platforms
}
