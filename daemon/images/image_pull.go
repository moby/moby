package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/stringid"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PullImage initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func (i *ImageService) PullImage(ctx context.Context, image, tag string, platform *specs.Platform, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	start := time.Now()
	// Special case: "pull -a" may send an image name with a
	// trailing :. This is ugly, but let's not break API
	// compatibility.
	image = strings.TrimSuffix(image, ":")

	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	if tag != "" {
		// The "tag" could actually be a digest.
		var dgst digest.Digest
		dgst, err = digest.Parse(tag)
		if err == nil {
			ref, err = reference.WithDigest(reference.TrimNamed(ref), dgst)
		} else {
			ref, err = reference.WithTag(ref, tag)
		}
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
	}

	err = i.pullImageWithReference(ctx, ref, platform, metaHeaders, authConfig, outStream)
	imageActions.WithValues("pull").UpdateSince(start)
	return err
}

func (i *ImageService) pullImageWithReference(ctx context.Context, ref reference.Named, platform *specs.Platform, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	c, err := i.getCache(ctx)
	if err != nil {
		return err
	}

	ongoing := newJobs(ref.Name())

	pctx, stopProgress := context.WithCancel(ctx)
	progress := make(chan struct{})

	go func() {
		// no progress bar, because it hides some debug logs
		showProgress(pctx, ongoing, ref, i.client.ContentStore(), outStream)
		close(progress)
	}()
	// TODO: Lease
	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ongoing.add(desc)
		}
		return nil, nil
	})
	opts := []containerd.RemoteOpt{
		containerd.WithImageHandler(h),
	}
	// TODO: Custom resolver
	//  - Auth config
	//  - Custom headers
	// TODO: Platforms using `platform`
	// TODO(containerd): progress tracking
	// TODO: unpack tracking, use download manager for now?

	img, err := i.client.Pull(pctx, ref.String(), opts...)

	config, err := img.Config(pctx)
	if err != nil {
		return errors.Wrap(err, "failed to resolve configuration")
	}

	l, err := i.unpack(pctx, img.Target())
	if err != nil {
		return errors.Wrapf(err, "failed to unpack %s", img.Target().Digest)
	}

	// TODO: Unpack into layer store
	// TODO: only unpack image types (does containerd already do this?)

	// TODO: Update image with ID label
	// TODO(containerd): Create manifest reference and add image

	c.m.Lock()
	ci, ok := c.idCache[config.Digest]
	if ok {
		ll := ci.layer
		ci.layer = l
		if ll != nil {
			metadata, err := i.layerStores[runtime.GOOS].Release(ll)
			if err != nil {
				return errors.Wrap(err, "failed to release layer")
			}
			layer.LogReleaseMetadata(metadata)
		}

		ci.addReference(ref)
		// TODO: Add manifest digest ref
	} else {
		ci = &cachedImage{
			config:     config,
			references: []reference.Named{ref},
			layer:      l,
		}
		c.idCache[config.Digest] = ci
	}
	c.tCache[img.Target().Digest] = ci
	c.m.Unlock()
	stopProgress()
	<-progress

	return err
}

// TODO: Add shallow pull function which returns descriptor

func (i *ImageService) unpack(ctx context.Context, target ocispec.Descriptor) (layer.Layer, error) {
	var (
		cs = i.client.ContentStore()
	)

	manifest, err := images.Manifest(ctx, cs, target, platforms.Default())
	if err != nil {
		return nil, err
	}

	diffIDs, err := images.RootFS(ctx, cs, manifest.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve rootfs")
	}
	if len(diffIDs) != len(manifest.Layers) {
		return nil, errors.Errorf("mismatched image rootfs and manifest layers")
	}

	var (
		chain = []digest.Digest{}
		l     layer.Layer
	)
	for d := range diffIDs {
		chain = append(chain, diffIDs[d])

		nl, err := i.applyLayer(ctx, manifest.Layers[d], chain)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to apply layer %d", d)
		}
		logrus.Debugf("Layer applied: %s (%s)", nl.DiffID(), diffIDs[d])

		if l != nil {
			metadata, err := i.layerStores[runtime.GOOS].Release(l)
			if err != nil {
				return nil, errors.Wrap(err, "failed to release layer")
			}
			layer.LogReleaseMetadata(metadata)
		}

		// TODO(containerd): verify diff ID

		l = nl
	}
	return l, nil
}

func (i *ImageService) applyLayer(ctx context.Context, blob ocispec.Descriptor, layers []digest.Digest) (layer.Layer, error) {
	var (
		cs = i.client.ContentStore()
		ls = i.layerStores[runtime.GOOS]
	)

	l, err := ls.Get(layer.ChainID(identity.ChainID(layers)))
	if err == nil {
		return l, nil
	} else if err != layer.ErrLayerDoesNotExist {
		return nil, err
	}

	ra, err := cs.ReaderAt(ctx, blob)
	if err != nil {
		return nil, err
	}
	defer ra.Close()

	dc, err := compression.DecompressStream(content.NewReader(ra))
	if err != nil {
		return nil, err
	}
	defer dc.Close()

	var parent digest.Digest
	if len(layers) > 1 {
		parent = identity.ChainID(layers[:len(layers)-1])
	}

	return ls.Register(dc, layer.ChainID(parent))
}
func getTagOrDigest(ref reference.Named) string {
	var (
		// manifest    distribution.Manifest
		tagOrDigest string // Used for logging/progress only
	)
	if digested, isDigested := ref.(reference.Canonical); isDigested {
		tagOrDigest = digested.Digest().String()
	} else if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
		tagOrDigest = tagged.Tag()
	}
	// todo: is it safe to assume it is always a tag or digest?
	return tagOrDigest
}

const (
	downloading = "Downloading"
	dlcomplete  = "Download complete"
	waiting     = "Waiting"
	exists      = "Already exists"
)

func showProgress(ctx context.Context, ongoing *jobs, ref reference.Named, cs content.Store, out io.Writer) {
	progressOutput := streamformatter.NewJSONProgressOutput(out, false)
	progressOutput.WriteProgress(progress.Progress{ID: getTagOrDigest(ref), Message: "Pulling from " + reference.Path(ref)})
	var (
		ticker   = time.NewTicker(100 * time.Millisecond)
		start    = time.Now()
		statuses = map[string]StatusInfo{}
		done     bool
	)
	defer ticker.Stop()

outer:
	for {
		select {
		case <-ticker.C:
			activeSeen := map[string]struct{}{}
			if !done {
				active, err := cs.ListStatuses(ctx, "")
				if err != nil {
					logrus.Error("active check failed")
					continue
				}
				// update status of active entries!
				for _, active := range active {
					descID := stringid.TruncateID(active.Ref)
					if !strings.Contains(active.Ref, "layer") {
						continue
					}
					progressOutput.WriteProgress(progress.Progress{ID: descID, Action: downloading, Current: active.Offset, Total: active.Total, LastUpdate: false})
					statuses[descID] = StatusInfo{
						Status: downloading, // Downloading
					}
					activeSeen[descID] = struct{}{}
				}
			}

			// now, update the items in jobs that are not in active
			for _, j := range ongoing.jobs() {
				descID := stringid.TruncateID(j.Digest.String())
				if _, ok := activeSeen[descID]; ok {
					continue
				}
				// skip displaying non-layer info
				if !isLayer(j) {
					continue
				}
				status, ok := statuses[descID]
				if !done && (!ok || status.Status == downloading) {
					info, err := cs.Info(ctx, j.Digest)
					if err != nil {
						if !errdefs.IsNotFound(err) {
							logrus.Errorf("failed to get content info")
							continue outer
						} else {
							progressOutput.WriteProgress(progress.Progress{ID: descID, Action: waiting})
							statuses[descID] = StatusInfo{
								Status: waiting,
							}
						}
					} else if info.CreatedAt.After(start) {
						progressOutput.WriteProgress(progress.Progress{ID: descID, Action: dlcomplete})
						statuses[descID] = StatusInfo{
							Status: dlcomplete,
						}
					} else {
						progressOutput.WriteProgress(progress.Progress{ID: descID, Action: exists})
						statuses[descID] = StatusInfo{
							Status: exists,
						}
					}
				} else if done {
					progressOutput.WriteProgress(progress.Progress{ID: descID, Action: dlcomplete})
					if ok {
						if status.Status != dlcomplete && status.Status != exists {
							status.Status = dlcomplete
							statuses[descID] = status
						}
					} else {
						statuses[descID] = StatusInfo{
							Status: dlcomplete,
						}
					}
				}
			}

			if done {
				return
			}
		case <-ctx.Done():
			done = true // allow ui to update once more
		}
	}
}

// StatusInfo holds the status info for an upload or download
type StatusInfo struct {
	Status string
}

func isLayer(desc ocispec.Descriptor) bool {
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Layer, images.MediaTypeDockerSchema2LayerGzip,
		images.MediaTypeDockerSchema2LayerForeign, images.MediaTypeDockerSchema2LayerForeignGzip,
		ocispec.MediaTypeImageLayer, ocispec.MediaTypeImageLayerGzip,
		ocispec.MediaTypeImageLayerNonDistributable, ocispec.MediaTypeImageLayerNonDistributableGzip:
		return true
	default:
		return false
	}
}

// jobs provides a way of identifying the download keys for a particular task
// encountering during the pull walk.
//
// This is very minimal and will probably be replaced with something more
// featured.
type jobs struct {
	name  string
	added map[digest.Digest]struct{}
	descs []ocispec.Descriptor
	mu    sync.Mutex
}

func newJobs(name string) *jobs {
	return &jobs{
		name:  name,
		added: map[digest.Digest]struct{}{},
	}
}

func (j *jobs) add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.added[desc.Digest]; ok {
		return
	}
	j.descs = append(j.descs, desc)
	j.added[desc.Digest] = struct{}{}
}

func (j *jobs) jobs() []ocispec.Descriptor {
	j.mu.Lock()
	defer j.mu.Unlock()

	var descs []ocispec.Descriptor
	return append(descs, j.descs...)
}
