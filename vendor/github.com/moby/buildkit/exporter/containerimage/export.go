package containerimage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/rootfs"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/push"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	keyPush           = "push"
	keyPushByDigest   = "push-by-digest"
	keyInsecure       = "registry.insecure"
	keyUnpack         = "unpack"
	keyDanglingPrefix = "dangling-name-prefix"
	keyNameCanonical  = "name-canonical"
	keyStore          = "store"

	// keyUnsafeInternalStoreAllowIncomplete should only be used for tests. This option allows exporting image to the image store
	// as well as lacking some blobs in the content store. Some integration tests for lazyref behaviour depends on this option.
	// Ignored when store=false.
	keyUnsafeInternalStoreAllowIncomplete = "unsafe-internal-store-allow-incomplete"
)

type Opt struct {
	SessionManager *session.Manager
	ImageWriter    *ImageWriter
	Images         images.Store
	RegistryHosts  docker.RegistryHosts
	LeaseManager   leases.Manager
}

type imageExporter struct {
	opt Opt
}

// New returns a new containerimage exporter instance that supports exporting
// to an image store and pushing the image to registry.
// This exporter supports following values in returned kv map:
// - containerimage.digest - The digest of the root manifest for the image.
func New(opt Opt) (exporter.Exporter, error) {
	im := &imageExporter{opt: opt}
	return im, nil
}

func (e *imageExporter) Resolve(ctx context.Context, opt map[string]string) (exporter.ExporterInstance, error) {
	i := &imageExporterInstance{
		imageExporter: e,
		opts: ImageCommitOpts{
			RefCfg: cacheconfig.RefConfig{
				Compression: compression.New(compression.Default),
			},
			BuildInfo:               true,
			ForceInlineAttestations: true,
		},
		store: true,
	}

	opt, err := i.opts.Load(opt)
	if err != nil {
		return nil, err
	}

	for k, v := range opt {
		switch k {
		case keyPush:
			if v == "" {
				i.push = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.push = b
		case keyPushByDigest:
			if v == "" {
				i.pushByDigest = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.pushByDigest = b
		case keyInsecure:
			if v == "" {
				i.insecure = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.insecure = b
		case keyUnpack:
			if v == "" {
				i.unpack = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.unpack = b
		case keyStore:
			if v == "" {
				i.store = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.store = b
		case keyUnsafeInternalStoreAllowIncomplete:
			if v == "" {
				i.storeAllowIncomplete = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.storeAllowIncomplete = b
		case keyDanglingPrefix:
			i.danglingPrefix = v
		case keyNameCanonical:
			if v == "" {
				i.nameCanonical = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.nameCanonical = b
		default:
			if i.meta == nil {
				i.meta = make(map[string][]byte)
			}
			i.meta[k] = []byte(v)
		}
	}
	return i, nil
}

type imageExporterInstance struct {
	*imageExporter
	opts                 ImageCommitOpts
	push                 bool
	pushByDigest         bool
	unpack               bool
	store                bool
	storeAllowIncomplete bool
	insecure             bool
	nameCanonical        bool
	danglingPrefix       string
	meta                 map[string][]byte
}

func (e *imageExporterInstance) Name() string {
	return "exporting to image"
}

func (e *imageExporterInstance) Config() *exporter.Config {
	return exporter.NewConfigWithCompression(e.opts.RefCfg.Compression)
}

func (e *imageExporterInstance) Export(ctx context.Context, src *exporter.Source, sessionID string) (_ map[string]string, descref exporter.DescriptorReference, err error) {
	if src.Metadata == nil {
		src.Metadata = make(map[string][]byte)
	}
	for k, v := range e.meta {
		src.Metadata[k] = v
	}

	opts := e.opts
	as, _, err := ParseAnnotations(src.Metadata)
	if err != nil {
		return nil, nil, err
	}
	opts.Annotations = opts.Annotations.Merge(as)

	ctx, done, err := leaseutil.WithLease(ctx, e.opt.LeaseManager, leaseutil.MakeTemporary)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if descref == nil {
			done(context.TODO())
		}
	}()

	desc, err := e.opt.ImageWriter.Commit(ctx, src, sessionID, &opts)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err == nil {
			descref = NewDescriptorReference(*desc, done)
		}
	}()

	resp := make(map[string]string)

	if n, ok := src.Metadata["image.name"]; e.opts.ImageName == "*" && ok {
		e.opts.ImageName = string(n)
	}

	nameCanonical := e.nameCanonical
	if e.opts.ImageName == "" && e.danglingPrefix != "" {
		e.opts.ImageName = e.danglingPrefix + "@" + desc.Digest.String()
		nameCanonical = false
	}

	if e.opts.ImageName != "" {
		targetNames := strings.Split(e.opts.ImageName, ",")
		for _, targetName := range targetNames {
			if e.opt.Images != nil && e.store {
				tagDone := progress.OneOff(ctx, "naming to "+targetName)
				img := images.Image{
					Target:    *desc,
					CreatedAt: time.Now(),
				}
				sfx := []string{""}
				if nameCanonical {
					sfx = append(sfx, "@"+desc.Digest.String())
				}
				for _, sfx := range sfx {
					img.Name = targetName + sfx
					if _, err := e.opt.Images.Update(ctx, img); err != nil {
						if !errors.Is(err, errdefs.ErrNotFound) {
							return nil, nil, tagDone(err)
						}

						if _, err := e.opt.Images.Create(ctx, img); err != nil {
							return nil, nil, tagDone(err)
						}
					}
				}
				tagDone(nil)

				if src.Ref != nil && e.unpack {
					if err := e.unpackImage(ctx, img, src, session.NewGroup(sessionID)); err != nil {
						return nil, nil, err
					}
				}

				if !e.storeAllowIncomplete {
					if src.Ref != nil {
						remotes, err := src.Ref.GetRemotes(ctx, false, e.opts.RefCfg, false, session.NewGroup(sessionID))
						if err != nil {
							return nil, nil, err
						}
						remote := remotes[0]
						if unlazier, ok := remote.Provider.(cache.Unlazier); ok {
							if err := unlazier.Unlazy(ctx); err != nil {
								return nil, nil, err
							}
						}
					}
					if len(src.Refs) > 0 {
						for _, r := range src.Refs {
							remotes, err := r.GetRemotes(ctx, false, e.opts.RefCfg, false, session.NewGroup(sessionID))
							if err != nil {
								return nil, nil, err
							}
							remote := remotes[0]
							if unlazier, ok := remote.Provider.(cache.Unlazier); ok {
								if err := unlazier.Unlazy(ctx); err != nil {
									return nil, nil, err
								}
							}
						}
					}
				}
			}
			if e.push {
				err := e.pushImage(ctx, src, sessionID, targetName, desc.Digest)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "failed to push %v", targetName)
				}
			}
		}
		resp["image.name"] = e.opts.ImageName
	}

	resp[exptypes.ExporterImageDigestKey] = desc.Digest.String()
	if v, ok := desc.Annotations[exptypes.ExporterConfigDigestKey]; ok {
		resp[exptypes.ExporterImageConfigDigestKey] = v
		delete(desc.Annotations, exptypes.ExporterConfigDigestKey)
	}

	dtdesc, err := json.Marshal(desc)
	if err != nil {
		return nil, nil, err
	}
	resp[exptypes.ExporterImageDescriptorKey] = base64.StdEncoding.EncodeToString(dtdesc)

	return resp, nil, nil
}

func (e *imageExporterInstance) pushImage(ctx context.Context, src *exporter.Source, sessionID string, targetName string, dgst digest.Digest) error {
	annotations := map[digest.Digest]map[string]string{}
	mprovider := contentutil.NewMultiProvider(e.opt.ImageWriter.ContentStore())
	if src.Ref != nil {
		remotes, err := src.Ref.GetRemotes(ctx, false, e.opts.RefCfg, false, session.NewGroup(sessionID))
		if err != nil {
			return err
		}
		remote := remotes[0]
		for _, desc := range remote.Descriptors {
			mprovider.Add(desc.Digest, remote.Provider)
			addAnnotations(annotations, desc)
		}
	}
	if len(src.Refs) > 0 {
		for _, r := range src.Refs {
			remotes, err := r.GetRemotes(ctx, false, e.opts.RefCfg, false, session.NewGroup(sessionID))
			if err != nil {
				return err
			}
			remote := remotes[0]
			for _, desc := range remote.Descriptors {
				mprovider.Add(desc.Digest, remote.Provider)
				addAnnotations(annotations, desc)
			}
		}
	}

	ctx = remotes.WithMediaTypeKeyPrefix(ctx, intoto.PayloadType, "intoto")
	return push.Push(ctx, e.opt.SessionManager, sessionID, mprovider, e.opt.ImageWriter.ContentStore(), dgst, targetName, e.insecure, e.opt.RegistryHosts, e.pushByDigest, annotations)
}

func (e *imageExporterInstance) unpackImage(ctx context.Context, img images.Image, src *exporter.Source, s session.Group) (err0 error) {
	unpackDone := progress.OneOff(ctx, "unpacking to "+img.Name)
	defer func() {
		unpackDone(err0)
	}()

	var (
		contentStore = e.opt.ImageWriter.ContentStore()
		applier      = e.opt.ImageWriter.Applier()
		snapshotter  = e.opt.ImageWriter.Snapshotter()
	)

	// fetch manifest by default platform
	manifest, err := images.Manifest(ctx, contentStore, img.Target, platforms.Default())
	if err != nil {
		return err
	}

	topLayerRef := src.Ref
	if len(src.Refs) > 0 {
		if r, ok := src.Refs[defaultPlatform()]; ok {
			topLayerRef = r
		} else {
			return errors.Errorf("no reference for default platform %s", defaultPlatform())
		}
	}

	remotes, err := topLayerRef.GetRemotes(ctx, true, e.opts.RefCfg, false, s)
	if err != nil {
		return err
	}
	remote := remotes[0]

	// ensure the content for each layer exists locally in case any are lazy
	if unlazier, ok := remote.Provider.(cache.Unlazier); ok {
		if err := unlazier.Unlazy(ctx); err != nil {
			return err
		}
	}

	layers, err := getLayers(ctx, remote.Descriptors, manifest)
	if err != nil {
		return err
	}

	// get containerd snapshotter
	ctrdSnapshotter, release := snapshot.NewContainerdSnapshotter(snapshotter)
	defer release()

	var chain []digest.Digest
	for _, layer := range layers {
		if _, err := rootfs.ApplyLayer(ctx, layer, chain, ctrdSnapshotter, applier); err != nil {
			return err
		}
		chain = append(chain, layer.Diff.Digest)
	}

	var (
		keyGCLabel   = fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", snapshotter.Name())
		valueGCLabel = identity.ChainID(chain).String()
	)

	cinfo := content.Info{
		Digest: manifest.Config.Digest,
		Labels: map[string]string{keyGCLabel: valueGCLabel},
	}
	_, err = contentStore.Update(ctx, cinfo, fmt.Sprintf("labels.%s", keyGCLabel))
	return err
}

func getLayers(ctx context.Context, descs []ocispecs.Descriptor, manifest ocispecs.Manifest) ([]rootfs.Layer, error) {
	if len(descs) != len(manifest.Layers) {
		return nil, errors.Errorf("mismatched image rootfs and manifest layers")
	}

	layers := make([]rootfs.Layer, len(descs))
	for i, desc := range descs {
		layers[i].Diff = ocispecs.Descriptor{
			MediaType: ocispecs.MediaTypeImageLayer,
			Digest:    digest.Digest(desc.Annotations["containerd.io/uncompressed"]),
		}
		layers[i].Blob = manifest.Layers[i]
	}
	return layers, nil
}

func addAnnotations(m map[digest.Digest]map[string]string, desc ocispecs.Descriptor) {
	if desc.Annotations == nil {
		return
	}
	a, ok := m[desc.Digest]
	if !ok {
		m[desc.Digest] = desc.Annotations
		return
	}
	for k, v := range desc.Annotations {
		a[k] = v
	}
}

func defaultPlatform() string {
	// Use normalized platform string to avoid the mismatch with platform options which
	// are normalized using platforms.Normalize()
	return platforms.Format(platforms.Normalize(platforms.DefaultSpec()))
}

func NewDescriptorReference(desc ocispecs.Descriptor, release func(context.Context) error) exporter.DescriptorReference {
	return &descriptorReference{
		desc:    desc,
		release: release,
	}
}

type descriptorReference struct {
	desc    ocispecs.Descriptor
	release func(context.Context) error
}

func (d *descriptorReference) Descriptor() ocispecs.Descriptor {
	return d.desc
}

func (d *descriptorReference) Release() error {
	return d.release(context.TODO())
}
