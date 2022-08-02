package image // import "github.com/docker/docker/api/server/router/image"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
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
func (s *imageRouter) postImagesCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {

	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var (
		image       = r.Form.Get("fromImage")
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

	if image != "" { // pull
		metaHeaders := map[string][]string{}
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Meta-") {
				metaHeaders[k] = v
			}
		}

		authEncoded := r.Header.Get("X-Registry-Auth")
		authConfig := &types.AuthConfig{}
		if authEncoded != "" {
			authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
			if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
				// for a pull it is not an error if no auth was given
				// to increase compatibility with the existing api it is defaulting to be empty
				authConfig = &types.AuthConfig{}
			}
		}
		progressErr = s.backend.PullImage(ctx, image, tag, platform, metaHeaders, authConfig, output)
	} else { // import
		src := r.Form.Get("fromSrc")
		progressErr = s.backend.ImportImage(src, repo, platform, tag, message, r.Body, output, r.Form["changes"])
	}
	if progressErr != nil {
		if !output.Flushed() {
			return progressErr
		}
		_, _ = output.Write(streamformatter.FormatError(progressErr))
	}

	return nil
}

func (s *imageRouter) postImagesPush(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	metaHeaders := map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	authConfig := &types.AuthConfig{}

	authEncoded := r.Header.Get("X-Registry-Auth")
	if authEncoded != "" {
		// the new format is to handle the authConfig as a header
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
			// to increase compatibility to existing api it is defaulting to be empty
			authConfig = &types.AuthConfig{}
		}
	} else {
		// the old format is supported for compatibility if there was no authConfig header
		if err := json.NewDecoder(r.Body).Decode(authConfig); err != nil {
			return errors.Wrap(errdefs.InvalidParameter(err), "Bad parameters and missing X-Registry-Auth")
		}
	}

	image := vars["name"]
	tag := r.Form.Get("tag")

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()

	w.Header().Set("Content-Type", "application/json")

	if err := s.backend.PushImage(ctx, image, tag, metaHeaders, authConfig, output); err != nil {
		if !output.Flushed() {
			return err
		}
		_, _ = output.Write(streamformatter.FormatError(err))
	}
	return nil
}

func (s *imageRouter) getImagesGet(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

	if err := s.backend.ExportImage(names, output); err != nil {
		if !output.Flushed() {
			return err
		}
		_, _ = output.Write(streamformatter.FormatError(err))
	}
	return nil
}

func (s *imageRouter) postImagesLoad(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	quiet := httputils.BoolValueOrDefault(r, "quiet", true)

	w.Header().Set("Content-Type", "application/json")

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	if err := s.backend.LoadImage(r.Body, output, quiet); err != nil {
		if !output.Flushed() {
			w.WriteHeader(errdefs.GetHTTPErrorStatusCode(err))
		}
		_, _ = output.Write(streamformatter.FormatError(err))
	}
	return nil
}

type missingImageError struct{}

func (missingImageError) Error() string {
	return "image name cannot be blank"
}

func (missingImageError) InvalidParameter() {}

func (s *imageRouter) deleteImages(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]

	if strings.TrimSpace(name) == "" {
		return missingImageError{}
	}

	force := httputils.BoolValue(r, "force")
	prune := !httputils.BoolValue(r, "noprune")

	list, err := s.backend.ImageDelete(name, force, prune)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, list)
}

func (s *imageRouter) getImagesByName(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	image, err := s.backend.GetImage(vars["name"], nil)
	if err != nil {
		return err
	}

	imageInspect, err := s.toImageInspect(image)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, imageInspect)
}

func (s *imageRouter) toImageInspect(img *image.Image) (*types.ImageInspect, error) {
	refs := s.referenceBackend.References(img.ID().Digest())
	repoTags := []string{}
	repoDigests := []string{}
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
	layerID := img.RootFS.ChainID()
	if layerID != "" {
		l, err := s.layerStore.Get(layerID)
		if err != nil {
			return nil, err
		}
		defer layer.ReleaseAndLog(s.layerStore, l)
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

	lastUpdated, err := s.imageStore.GetLastUpdated(img.ID())
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
			Name: s.layerStore.DriverName(),
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

func (s *imageRouter) getImagesJSON(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

	images, err := s.backend.Images(ctx, types.ImageListOptions{
		All:        httputils.BoolValue(r, "all"),
		Filters:    imageFilters,
		SharedSize: sharedSize,
	})
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, images)
}

func (s *imageRouter) getImagesHistory(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	name := vars["name"]
	history, err := s.backend.ImageHistory(name)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, history)
}

func (s *imageRouter) postImagesTag(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	if _, err := s.backend.TagImage(vars["name"], r.Form.Get("repo"), r.Form.Get("tag")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (s *imageRouter) getImagesSearch(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	var (
		config      *types.AuthConfig
		authEncoded = r.Header.Get("X-Registry-Auth")
		headers     = map[string][]string{}
	)

	if authEncoded != "" {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(&config); err != nil {
			// for a search it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			config = &types.AuthConfig{}
		}
	}
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

	query, err := s.backend.SearchRegistryForImages(ctx, searchFilters, r.Form.Get("term"), limit, config, headers)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, query.Results)
}

func (s *imageRouter) postImagesPrune(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	pruneFilters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	pruneReport, err := s.backend.ImagesPrune(ctx, pruneFilters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, pruneReport)
}
