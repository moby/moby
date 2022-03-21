package build // import "github.com/docker/docker/api/server/backend/build"

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/builder"
	buildkit "github.com/docker/docker/builder/builder-next"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// ImageComponent provides an interface for working with images
type ImageComponent interface {
	SquashImage(ctx context.Context, from string, to string) (string, error)
	TagImageWithReference(context.Context, image.ID, reference.Named) error
}

// Builder defines interface for running a build
type Builder interface {
	Build(context.Context, backend.BuildConfig) (*builder.Result, error)
}

// Backend provides build functionality to the API router
type Backend struct {
	builder        Builder
	imageComponent ImageComponent
	buildkit       *buildkit.Builder
	eventsService  *daemonevents.Events
}

// NewBackend creates a new build backend from components
func NewBackend(components ImageComponent, builder Builder, buildkit *buildkit.Builder, es *daemonevents.Events) (*Backend, error) {
	return &Backend{imageComponent: components, builder: builder, buildkit: buildkit, eventsService: es}, nil
}

// RegisterGRPC registers buildkit controller to the grpc server.
func (b *Backend) RegisterGRPC(s *grpc.Server) {
	if b.buildkit != nil {
		b.buildkit.RegisterGRPC(s)
	}
}

// Build builds an image from a Source
func (b *Backend) Build(ctx context.Context, config backend.BuildConfig) (string, error) {
	options := config.Options
	useBuildKit := options.Version == types.BuilderBuildKit

	tagger, err := NewTagger(b.imageComponent, config.ProgressWriter.StdoutFormatter, options.Tags)
	if err != nil {
		return "", err
	}

	var build *builder.Result
	if useBuildKit {
		build, err = b.buildkit.Build(ctx, config)
		if err != nil {
			return "", err
		}
	} else {
		build, err = b.builder.Build(ctx, config)
		if err != nil {
			return "", err
		}
	}

	if build == nil {
		return "", nil
	}

	var imageID = build.ImageID
	if options.Squash {
		if imageID, err = squashBuild(ctx, build, b.imageComponent); err != nil {
			return "", err
		}
		if config.ProgressWriter.AuxFormatter != nil {
			if err = config.ProgressWriter.AuxFormatter.Emit("moby.image.id", types.BuildResult{ID: imageID}); err != nil {
				return "", err
			}
		}
	}

	if !useBuildKit {
		stdout := config.ProgressWriter.StdoutFormatter
		fmt.Fprintf(stdout, "Successfully built %s\n", stringid.TruncateID(imageID))
	}
	if imageID != "" {
		err = tagger.TagImages(ctx, image.ID(imageID))
	}
	return imageID, err
}

// PruneCache removes all cached build sources
func (b *Backend) PruneCache(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error) {
	buildCacheSize, cacheIDs, err := b.buildkit.Prune(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prune build cache")
	}
	b.eventsService.Log("prune", events.BuilderEventType, events.Actor{
		Attributes: map[string]string{
			"reclaimed": strconv.FormatInt(buildCacheSize, 10),
		},
	})
	return &types.BuildCachePruneReport{SpaceReclaimed: uint64(buildCacheSize), CachesDeleted: cacheIDs}, nil
}

// Cancel cancels the build by ID
func (b *Backend) Cancel(ctx context.Context, id string) error {
	return b.buildkit.Cancel(ctx, id)
}

func squashBuild(ctx context.Context, build *builder.Result, imageComponent ImageComponent) (string, error) {
	var fromID string
	if build.FromImage != nil {
		fromID = build.FromImage.ImageID()
	}
	imageID, err := imageComponent.SquashImage(ctx, build.ImageID, fromID)
	if err != nil {
		return "", errors.Wrap(err, "error squashing image")
	}
	return imageID, nil
}
