package image // import "github.com/docker/docker/api/server/router/image"

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	opts "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/streamformatter"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Creates an image from Pull or from Import
func (ir *imageRouter) postImagesCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var (
		img         = r.Form.Get("fromImage")
		repo        = r.Form.Get("repo")
		tag         = r.Form.Get("tag")
		message     = r.Form.Get("message")
		progressErr error
		output      = ioutils.NewWriteFlusher(w)
		platform    *specs.Platform
	)
	defer output.Close()

	w.Header().Set("Content-Type", "application/json")

	version := httputils.VersionFromContext(ctx)
	if versions.GreaterThanOrEqualTo(version, "1.32") {
		if p := r.FormValue("platform"); p != "" {
			sp, err := platforms.Parse(p)
			if err != nil {
				return err
			}
			platform = &sp
		}
	}

	if img != "" { // pull
		metaHeaders := map[string][]string{}
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Meta-") {
				metaHeaders[k] = v
			}
		}

		// For a pull it is not an error if no auth was given. Ignore invalid
		// AuthConfig to increase compatibility with the existing API.
		authConfig, _ := registry.DecodeAuthConfig(r.Header.Get(registry.AuthHeader))
		progressErr = ir.backend.PullImage(ctx, img, tag, platform, metaHeaders, authConfig, output)
	} else { // import
		src := r.Form.Get("fromSrc")
		progressErr = ir.backend.ImportImage(ctx, src, repo, platform, tag, message, r.Body, output, r.Form["changes"])
	}
	if progressErr != nil {
		if !output.Flushed() {
			return progressErr
		}
		_, _ = output.Write(streamformatter.FormatError(progressErr))
	}

	return nil
}

func (ir *imageRouter) postImagesPush(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	metaHeaders := map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var authConfig *registry.AuthConfig
	if authEncoded := r.Header.Get(registry.AuthHeader); authEncoded != "" {
		// the new format is to handle the authConfig as a header. Ignore invalid
		// AuthConfig to increase compatibility with the existing API.
		authConfig, _ = registry.DecodeAuthConfig(authEncoded)
	} else {
		// the old format is supported for compatibility if there was no authConfig header
		var err error
		authConfig, err = registry.DecodeAuthConfigBody(r.Body)
		if err != nil {
			return errors.Wrap(err, "bad parameters and missing X-Registry-Auth")
		}
	}

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()

	w.Header().Set("Content-Type", "application/json")

	img := vars["name"]
	tag := r.Form.Get("tag")
	if err := ir.backend.PushImage(ctx, img, tag, metaHeaders, authConfig, output); err != nil {
		if !output.Flushed() {
			return err
		}
		_, _ = output.Write(streamformatter.FormatError(err))
	}
	return nil
}

func (ir *imageRouter) getImagesGet(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/x-tar")

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	var names []string
	if name, ok := vars["name"]; ok {
		names = []string{name}
	} else {
		names = r.Form["names"]
	}

	if err := ir.backend.ExportImage(ctx, names, output); err != nil {
		if !output.Flushed() {
			return err
		}
		_, _ = output.Write(streamformatter.FormatError(err))
	}
	return nil
}

func (ir *imageRouter) postImagesLoad(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	quiet := httputils.BoolValueOrDefault(r, "quiet", true)

	w.Header().Set("Content-Type", "application/json")

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	if err := ir.backend.LoadImage(ctx, r.Body, output, quiet); err != nil {
		_, _ = output.Write(streamformatter.FormatError(err))
	}
	return nil
}

type missingImageError struct{}

func (missingImageError) Error() string {
	return "image name cannot be blank"
}

func (missingImageError) InvalidParameter() {}

func (ir *imageRouter) deleteImages(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]

	if strings.TrimSpace(name) == "" {
		return missingImageError{}
	}

	force := httputils.BoolValue(r, "force")
	prune := !httputils.BoolValue(r, "noprune")

	list, err := ir.backend.ImageDelete(ctx, name, force, prune)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, list)
}

func (ir *imageRouter) getImagesByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	img, err := ir.backend.GetImage(ctx, vars["name"], opts.GetImageOpts{})
	if err != nil {
		return err
	}

	imageInspect, err := ir.toImageInspect(img)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, imageInspect)
}

func (ir *imageRouter) toImageInspect(img *image.Image) (*types.ImageInspect, error) {
	refs := ir.referenceBackend.References(img.ID().Digest())
	var repoTags, repoDigests []string
	for _, ref := range refs {
		switch ref.(type) {
		case reference.NamedTagged:
			repoTags = append(repoTags, reference.FamiliarString(ref))
		case reference.Canonical:
			repoDigests = append(repoDigests, reference.FamiliarString(ref))
		}
	}

	var size int64
	var layerMetadata map[string]string
	if layerID := img.RootFS.ChainID(); layerID != "" {
		l, err := ir.layerStore.Get(layerID)
		if err != nil {
			return nil, err
		}
		defer layer.ReleaseAndLog(ir.layerStore, l)
		size = l.Size()
		layerMetadata, err = l.Metadata()
		if err != nil {
			return nil, err
		}
	}

	comment := img.Comment
	if len(comment) == 0 && len(img.History) > 0 {
		comment = img.History[len(img.History)-1].Comment
	}

	lastUpdated, err := ir.imageStore.GetLastUpdated(img.ID())
	if err != nil {
		return nil, err
	}

	return &types.ImageInspect{
		ID:              img.ID().String(),
		RepoTags:        repoTags,
		RepoDigests:     repoDigests,
		Parent:          img.Parent.String(),
		Comment:         comment,
		Created:         img.Created.Format(time.RFC3339Nano),
		Container:       img.Container,
		ContainerConfig: &img.ContainerConfig,
		DockerVersion:   img.DockerVersion,
		Author:          img.Author,
		Config:          img.Config,
		Architecture:    img.Architecture,
		Variant:         img.Variant,
		Os:              img.OperatingSystem(),
		OsVersion:       img.OSVersion,
		Size:            size,
		VirtualSize:     size, // TODO: field unused, deprecate
		GraphDriver: types.GraphDriverData{
			Name: ir.layerStore.DriverName(),
			Data: layerMetadata,
		},
		RootFS: rootFSToAPIType(img.RootFS),
		Metadata: types.ImageMetadata{
			LastTagTime: lastUpdated,
		},
	}, nil
}

func rootFSToAPIType(rootfs *image.RootFS) types.RootFS {
	var layers []string
	for _, l := range rootfs.DiffIDs {
		layers = append(layers, l.String())
	}
	return types.RootFS{
		Type:   rootfs.Type,
		Layers: layers,
	}
}

func (ir *imageRouter) getImagesJSON(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	imageFilters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	version := httputils.VersionFromContext(ctx)
	if versions.LessThan(version, "1.41") {
		// NOTE: filter is a shell glob string applied to repository names.
		filterParam := r.Form.Get("filter")
		if filterParam != "" {
			imageFilters.Add("reference", filterParam)
		}
	}

	var sharedSize bool
	if versions.GreaterThanOrEqualTo(version, "1.42") {
		// NOTE: Support for the "shared-size" parameter was added in API 1.42.
		sharedSize = httputils.BoolValue(r, "shared-size")
	}

	images, err := ir.backend.Images(ctx, types.ImageListOptions{
		All:        httputils.BoolValue(r, "all"),
		Filters:    imageFilters,
		SharedSize: sharedSize,
	})
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, images)
}

func (ir *imageRouter) getImagesHistory(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	history, err := ir.backend.ImageHistory(vars["name"])
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, history)
}

func (ir *imageRouter) postImagesTag(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	if _, err := ir.backend.TagImage(vars["name"], r.Form.Get("repo"), r.Form.Get("tag")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (ir *imageRouter) getImagesSearch(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var headers = map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			headers[k] = v
		}
	}

	var limit int
	if r.Form.Get("limit") != "" {
		var err error
		limit, err = strconv.Atoi(r.Form.Get("limit"))
		if err != nil || limit < 0 {
			return errdefs.InvalidParameter(errors.Wrap(err, "invalid limit specified"))
		}
	}
	searchFilters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	// For a search it is not an error if no auth was given. Ignore invalid
	// AuthConfig to increase compatibility with the existing API.
	authConfig, _ := registry.DecodeAuthConfig(r.Header.Get(registry.AuthHeader))
	query, err := ir.backend.SearchRegistryForImages(ctx, searchFilters, r.Form.Get("term"), limit, authConfig, headers)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, query.Results)
}

func (ir *imageRouter) postImagesPrune(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	pruneFilters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	pruneReport, err := ir.backend.ImagesPrune(ctx, pruneFilters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, pruneReport)
}
