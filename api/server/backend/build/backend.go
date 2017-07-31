package build

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/backend"
	"github.com/moby/moby/builder"
	"github.com/moby/moby/builder/fscache"
	"github.com/moby/moby/image"
	"github.com/moby/moby/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// ImageComponent provides an interface for working with images
type ImageComponent interface {
	SquashImage(from string, to string) (string, error)
	TagImageWithReference(image.ID, string, reference.Named) error
}

// Builder defines interface for running a build
type Builder interface {
	Build(context.Context, backend.BuildConfig) (*builder.Result, error)
}

// Backend provides build functionality to the API router
type Backend struct {
	builder        Builder
	fsCache        *fscache.FSCache
	imageComponent ImageComponent
}

// NewBackend creates a new build backend from components
func NewBackend(components ImageComponent, builder Builder, fsCache *fscache.FSCache) (*Backend, error) {
	return &Backend{imageComponent: components, builder: builder, fsCache: fsCache}, nil
}

// Build builds an image from a Source
func (b *Backend) Build(ctx context.Context, config backend.BuildConfig) (string, error) {
	options := config.Options
	tagger, err := NewTagger(b.imageComponent, config.ProgressWriter.StdoutFormatter, options.Tags)
	if err != nil {
		return "", err
	}

	build, err := b.builder.Build(ctx, config)
	if err != nil {
		return "", err
	}

	var imageID = build.ImageID
	if options.Squash {
		if imageID, err = squashBuild(build, b.imageComponent); err != nil {
			return "", err
		}
		if config.ProgressWriter.AuxFormatter != nil {
			if err = config.ProgressWriter.AuxFormatter.Emit(types.BuildResult{ID: imageID}); err != nil {
				return "", err
			}
		}
	}

	stdout := config.ProgressWriter.StdoutFormatter
	fmt.Fprintf(stdout, "Successfully built %s\n", stringid.TruncateID(imageID))
	err = tagger.TagImages(image.ID(imageID))
	return imageID, err
}

// PruneCache removes all cached build sources
func (b *Backend) PruneCache(ctx context.Context) (*types.BuildCachePruneReport, error) {
	size, err := b.fsCache.Prune(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prune build cache")
	}
	return &types.BuildCachePruneReport{SpaceReclaimed: size}, nil
}

func squashBuild(build *builder.Result, imageComponent ImageComponent) (string, error) {
	var fromID string
	if build.FromImage != nil {
		fromID = build.FromImage.ImageID()
	}
	imageID, err := imageComponent.SquashImage(build.ImageID, fromID)
	if err != nil {
		return "", errors.Wrap(err, "error squashing image")
	}
	return imageID, nil
}
