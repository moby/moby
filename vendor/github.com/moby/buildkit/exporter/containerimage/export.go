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
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/rootfs"
	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/push"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	keyImageName        = "name"
	keyPush             = "push"
	keyPushByDigest     = "push-by-digest"
	keyInsecure         = "registry.insecure"
	keyUnpack           = "unpack"
	keyDanglingPrefix   = "dangling-name-prefix"
	keyNameCanonical    = "name-canonical"
	keyLayerCompression = "compression"
	keyForceCompression = "force-compression"
	keyCompressionLevel = "compression-level"
	keyBuildInfo        = "buildinfo"
	keyBuildInfoAttrs   = "buildinfo-attrs"
	ociTypes            = "oci-mediatypes"
	// preferNondistLayersKey is an exporter option which can be used to mark a layer as non-distributable if the layer reference was
	// already found to use a non-distributable media type.
	// When this option is not set, the exporter will change the media type of the layer to a distributable one.
	preferNondistLayersKey = "prefer-nondist-layers"
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
		imageExporter:    e,
		layerCompression: compression.Default,
		buildInfo:        true,
	}

	var esgz bool
	for k, v := range opt {
		switch k {
		case keyImageName:
			i.targetName = v
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
		case ociTypes:
			if v == "" {
				i.ociTypes = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.ociTypes = b
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
		case keyLayerCompression:
			switch v {
			case "gzip":
				i.layerCompression = compression.Gzip
			case "estargz":
				i.layerCompression = compression.EStargz
				esgz = true
			case "zstd":
				i.layerCompression = compression.Zstd
			case "uncompressed":
				i.layerCompression = compression.Uncompressed
			default:
				return nil, errors.Errorf("unsupported layer compression type: %v", v)
			}
		case keyForceCompression:
			if v == "" {
				i.forceCompression = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value %s specified for %s", v, k)
			}
			i.forceCompression = b
		case keyCompressionLevel:
			ii, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "non-integer value %s specified for %s", v, k)
			}
			v := int(ii)
			i.compressionLevel = &v
		case keyBuildInfo:
			if v == "" {
				i.buildInfo = true
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.buildInfo = b
		case keyBuildInfoAttrs:
			if v == "" {
				i.buildInfoAttrs = false
				continue
			}
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value specified for %s", k)
			}
			i.buildInfoAttrs = b
		case preferNondistLayersKey:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value %s specified for %s", v, k)
			}
			i.preferNondistLayers = b
		default:
			if i.meta == nil {
				i.meta = make(map[string][]byte)
			}
			i.meta[k] = []byte(v)
		}
	}
	if esgz && !i.ociTypes {
		logrus.Warn("forcibly turning on oci-mediatype mode for estargz")
		i.ociTypes = true
	}
	return i, nil
}

type imageExporterInstance struct {
	*imageExporter
	targetName          string
	push                bool
	pushByDigest        bool
	unpack              bool
	insecure            bool
	ociTypes            bool
	nameCanonical       bool
	danglingPrefix      string
	layerCompression    compression.Type
	forceCompression    bool
	compressionLevel    *int
	buildInfo           bool
	buildInfoAttrs      bool
	meta                map[string][]byte
	preferNondistLayers bool
}

func (e *imageExporterInstance) Name() string {
	return "exporting to image"
}

func (e *imageExporterInstance) Config() exporter.Config {
	return exporter.Config{
		Compression: e.compression(),
	}
}

func (e *imageExporterInstance) compression() compression.Config {
	c := compression.New(e.layerCompression).SetForce(e.forceCompression)
	if e.compressionLevel != nil {
		c = c.SetLevel(*e.compressionLevel)
	}
	return c
}

func (e *imageExporterInstance) Export(ctx context.Context, src exporter.Source, sessionID string) (map[string]string, error) {
	if src.Metadata == nil {
		src.Metadata = make(map[string][]byte)
	}
	for k, v := range e.meta {
		src.Metadata[k] = v
	}

	ctx, done, err := leaseutil.WithLease(ctx, e.opt.LeaseManager, leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}
	defer done(context.TODO())

	refCfg := e.refCfg()
	desc, err := e.opt.ImageWriter.Commit(ctx, src, e.ociTypes, refCfg, e.buildInfo, e.buildInfoAttrs, sessionID)
	if err != nil {
		return nil, err
	}

	defer func() {
		e.opt.ImageWriter.ContentStore().Delete(context.TODO(), desc.Digest)
	}()

	resp := make(map[string]string)

	if n, ok := src.Metadata["image.name"]; e.targetName == "*" && ok {
		e.targetName = string(n)
	}

	nameCanonical := e.nameCanonical
	if e.targetName == "" && e.danglingPrefix != "" {
		e.targetName = e.danglingPrefix + "@" + desc.Digest.String()
		nameCanonical = false
	}

	if e.targetName != "" {
		targetNames := strings.Split(e.targetName, ",")
		for _, targetName := range targetNames {
			if e.opt.Images != nil {
				tagDone := oneOffProgress(ctx, "naming to "+targetName)
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
							return nil, tagDone(err)
						}

						if _, err := e.opt.Images.Create(ctx, img); err != nil {
							return nil, tagDone(err)
						}
					}
				}
				tagDone(nil)

				if e.unpack {
					if err := e.unpackImage(ctx, img, src, session.NewGroup(sessionID)); err != nil {
						return nil, err
					}
				}
			}
			if e.push {
				annotations := map[digest.Digest]map[string]string{}
				mprovider := contentutil.NewMultiProvider(e.opt.ImageWriter.ContentStore())
				if src.Ref != nil {
					remotes, err := src.Ref.GetRemotes(ctx, false, refCfg, false, session.NewGroup(sessionID))
					if err != nil {
						return nil, err
					}
					remote := remotes[0]
					for _, desc := range remote.Descriptors {
						mprovider.Add(desc.Digest, remote.Provider)
						addAnnotations(annotations, desc)
					}
				}
				if len(src.Refs) > 0 {
					for _, r := range src.Refs {
						remotes, err := r.GetRemotes(ctx, false, refCfg, false, session.NewGroup(sessionID))
						if err != nil {
							return nil, err
						}
						remote := remotes[0]
						for _, desc := range remote.Descriptors {
							mprovider.Add(desc.Digest, remote.Provider)
							addAnnotations(annotations, desc)
						}
					}
				}

				if err := push.Push(ctx, e.opt.SessionManager, sessionID, mprovider, e.opt.ImageWriter.ContentStore(), desc.Digest, targetName, e.insecure, e.opt.RegistryHosts, e.pushByDigest, annotations); err != nil {
					return nil, err
				}
			}
		}
		resp["image.name"] = e.targetName
	}

	resp[exptypes.ExporterImageDigestKey] = desc.Digest.String()
	if v, ok := desc.Annotations[exptypes.ExporterConfigDigestKey]; ok {
		resp[exptypes.ExporterImageConfigDigestKey] = v
		delete(desc.Annotations, exptypes.ExporterConfigDigestKey)
	}

	dtdesc, err := json.Marshal(desc)
	if err != nil {
		return nil, err
	}
	resp[exptypes.ExporterImageDescriptorKey] = base64.StdEncoding.EncodeToString(dtdesc)

	return resp, nil
}

func (e *imageExporterInstance) refCfg() cacheconfig.RefConfig {
	return cacheconfig.RefConfig{
		Compression:            e.compression(),
		PreferNonDistributable: e.preferNondistLayers,
	}
}

func (e *imageExporterInstance) unpackImage(ctx context.Context, img images.Image, src exporter.Source, s session.Group) (err0 error) {
	unpackDone := oneOffProgress(ctx, "unpacking to "+img.Name)
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

	remotes, err := topLayerRef.GetRemotes(ctx, true, e.refCfg(), false, s)
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
