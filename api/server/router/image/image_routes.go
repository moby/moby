package image // import "github.com/docker/docker/api/server/router/image"

import (
	"context"
	"io"
	"net/http"
	"net/url"
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
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
		comment     = r.Form.Get("message")
		progressErr error
		output      = ioutils.NewWriteFlusher(w)
		platform    *ocispec.Platform
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

		tagRef, err := httputils.RepoTagReference(repo, tag)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}

		if len(comment) == 0 {
			comment = "Imported from " + src
		}

		var layerReader io.ReadCloser
		defer r.Body.Close()
		if src == "-" {
			layerReader = r.Body
		} else {
			if len(strings.Split(src, "://")) == 1 {
				src = "http://" + src
			}
			u, err := url.Parse(src)
			if err != nil {
				return errdefs.InvalidParameter(err)
			}

			resp, err := remotecontext.GetWithStatusError(u.String())
			if err != nil {
				return err
			}
			output.Write(streamformatter.FormatStatus("", "Downloading from %s", u))
			progressOutput := streamformatter.NewJSONProgressOutput(output, true)
			layerReader = progress.NewProgressReader(resp.Body, progressOutput, resp.ContentLength, "", "Importing")
			defer layerReader.Close()
		}

		var id image.ID
		id, progressErr = ir.backend.ImportImage(ctx, tagRef, platform, comment, layerReader, r.Form["changes"])

		if progressErr == nil {
			output.Write(streamformatter.FormatStatus("", id.String()))
		}
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

	var ref reference.Named

	// Tag is empty only in case ImagePushOptions.All is true.
	if tag != "" {
		r, err := httputils.RepoTagReference(img, tag)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		ref = r
	} else {
		r, err := reference.ParseNormalizedNamed(img)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		ref = r
	}

	if err := ir.backend.PushImage(ctx, ref, metaHeaders, authConfig, output); err != nil {
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
	img, err := ir.backend.GetImage(ctx, vars["name"], opts.GetImageOpts{Details: true})
	if err != nil {
		return err
	}

	imageInspect, err := ir.toImageInspect(img)
	if err != nil {
		return err
	}

	version := httputils.VersionFromContext(ctx)
	if versions.LessThan(version, "1.44") {
		imageInspect.VirtualSize = imageInspect.Size //nolint:staticcheck // ignore SA1019: field is deprecated, but still set on API < v1.44.
	}
	return httputils.WriteJSON(w, http.StatusOK, imageInspect)
}

func (ir *imageRouter) toImageInspect(img *image.Image) (*types.ImageInspect, error) {
	var repoTags, repoDigests []string
	for _, ref := range img.Details.References {
		switch ref.(type) {
		case reference.NamedTagged:
			repoTags = append(repoTags, reference.FamiliarString(ref))
		case reference.Canonical:
			repoDigests = append(repoDigests, reference.FamiliarString(ref))
		}
	}

	comment := img.Comment
	if len(comment) == 0 && len(img.History) > 0 {
		comment = img.History[len(img.History)-1].Comment
	}

	// Make sure we output empty arrays instead of nil.
	if repoTags == nil {
		repoTags = []string{}
	}
	if repoDigests == nil {
		repoDigests = []string{}
	}

	var created string
	if img.Created != nil {
		created = img.Created.Format(time.RFC3339Nano)
	}

	return &types.ImageInspect{
		ID:              img.ID().String(),
		RepoTags:        repoTags,
		RepoDigests:     repoDigests,
		Parent:          img.Parent.String(),
		Comment:         comment,
		Created:         created,
		Container:       img.Container,
		ContainerConfig: &img.ContainerConfig,
		DockerVersion:   img.DockerVersion,
		Author:          img.Author,
		Config:          img.Config,
		Architecture:    img.Architecture,
		Variant:         img.Variant,
		Os:              img.OperatingSystem(),
		OsVersion:       img.OSVersion,
		Size:            img.Details.Size,
		GraphDriver: types.GraphDriverData{
			Name: img.Details.Driver,
			Data: img.Details.Metadata,
		},
		RootFS: rootFSToAPIType(img.RootFS),
		Metadata: types.ImageMetadata{
			LastTagTime: img.Details.LastUpdated,
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

	useNone := versions.LessThan(version, "1.43")
	withVirtualSize := versions.LessThan(version, "1.44")
	for _, img := range images {
		if useNone {
			if len(img.RepoTags) == 0 && len(img.RepoDigests) == 0 {
				img.RepoTags = append(img.RepoTags, "<none>:<none>")
				img.RepoDigests = append(img.RepoDigests, "<none>@<none>")
			}
		} else {
			if img.RepoTags == nil {
				img.RepoTags = []string{}
			}
			if img.RepoDigests == nil {
				img.RepoDigests = []string{}
			}
		}
		if withVirtualSize {
			img.VirtualSize = img.Size //nolint:staticcheck // ignore SA1019: field is deprecated, but still set on API < v1.44.
		}
	}

	return httputils.WriteJSON(w, http.StatusOK, images)
}

func (ir *imageRouter) getImagesHistory(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	history, err := ir.backend.ImageHistory(ctx, vars["name"])
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, history)
}

func (ir *imageRouter) postImagesTag(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	ref, err := httputils.RepoTagReference(r.Form.Get("repo"), r.Form.Get("tag"))
	if ref == nil || err != nil {
		return errdefs.InvalidParameter(err)
	}

	img, err := ir.backend.GetImage(ctx, vars["name"], opts.GetImageOpts{})
	if err != nil {
		return errdefs.NotFound(err)
	}

	if err := ir.backend.TagImage(ctx, img.ID(), ref); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (ir *imageRouter) getImagesSearch(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
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

	var headers = http.Header{}
	for k, v := range r.Header {
		k = http.CanonicalHeaderKey(k)
		if strings.HasPrefix(k, "X-Meta-") {
			headers[k] = v
		}
	}
	headers.Set("User-Agent", dockerversion.DockerUserAgent(ctx))
	res, err := ir.searcher.Search(ctx, searchFilters, r.Form.Get("term"), limit, authConfig, headers)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, res)
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
