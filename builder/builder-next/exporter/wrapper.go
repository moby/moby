package exporter

import (
	"context"
	"fmt"
	"strings"

	distref "github.com/distribution/reference"
	"github.com/docker/docker/builder/builder-next/exporter/overrides"
	"github.com/docker/docker/image"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/util/progress"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Opt are options for the exporter wrapper.
type Opt struct {
	// Callbacks contains callbacks used by the image exporter.
	Callbacks BuildkitCallbacks

	// ImageTagger is used to tag the image after it is exported.
	ImageTagger ImageTagger
}

type BuildkitCallbacks struct {
	// Exported is a Called when an image is exported by buildkit.
	Exported func(ctx context.Context, id string, desc ocispec.Descriptor)
}

type ImageTagger interface {
	TagImage(ctx context.Context, imageID image.ID, newTag distref.Named) error
}

// Wraps the containerimage exporter's Resolve method to apply moby-specific
// overrides to the exporter attributes.
type imageExporterMobyWrapper struct {
	exp exporter.Exporter
	opt Opt
}

// NewWrapper returns an exporter wrapper that applies moby specific attributes
// and hooks the export process.
func NewWrapper(exp exporter.Exporter, opt Opt) (exporter.Exporter, error) {
	return &imageExporterMobyWrapper{
		exp: exp,
		opt: opt,
	}, nil
}

// Resolve applies moby specific attributes to the request.
func (e *imageExporterMobyWrapper) Resolve(ctx context.Context, id int, exporterAttrs map[string]string) (exporter.ExporterInstance, error) {
	if exporterAttrs == nil {
		exporterAttrs = make(map[string]string)
	}
	reposAndTags, err := overrides.SanitizeRepoAndTags(strings.Split(exporterAttrs[string(exptypes.OptKeyName)], ","))
	if err != nil {
		return nil, err
	}

	// Force the exporter to not use a name so it always creates a dangling image.
	exporterAttrs[string(exptypes.OptKeyName)] = ""
	exporterAttrs[string(exptypes.OptKeyUnpack)] = "true"
	if _, has := exporterAttrs[string(exptypes.OptKeyDanglingPrefix)]; !has {
		exporterAttrs[string(exptypes.OptKeyDanglingPrefix)] = "moby-dangling"
	}
	exporterAttrs[string(exptypes.OptKeyDanglingEmptyOnly)] = "true"

	inst, err := e.exp.Resolve(ctx, id, exporterAttrs)
	if err != nil {
		return nil, err
	}

	return &imageExporterInstanceWrapper{
		ExporterInstance: inst,
		reposAndTags:     reposAndTags,
		opt:              e.opt,
	}, nil
}

type imageExporterInstanceWrapper struct {
	exporter.ExporterInstance
	reposAndTags []string
	opt          Opt
}

func (i *imageExporterInstanceWrapper) Export(ctx context.Context, src *exporter.Source, inlineCache exptypes.InlineCache, sessionID string) (map[string]string, exporter.DescriptorReference, error) {
	out, ref, err := i.ExporterInstance.Export(ctx, src, inlineCache, sessionID)
	if err != nil {
		return out, ref, err
	}

	desc := ref.Descriptor()
	imageID := out[exptypes.ExporterImageDigestKey]
	if i.opt.Callbacks.Exported != nil {
		i.opt.Callbacks.Exported(ctx, imageID, desc)
	}

	err = i.processNamed(ctx, image.ID(imageID), out, desc)
	return out, ref, err
}

func (i *imageExporterInstanceWrapper) processNamed(ctx context.Context, imageID image.ID, out map[string]string, desc ocispec.Descriptor) error {
	if len(i.reposAndTags) == 0 {
		return nil
	}

	for _, repoAndTag := range i.reposAndTags {
		newTag, err := distref.ParseNamed(repoAndTag)
		if err != nil {
			return err
		}

		done := progress.OneOff(ctx, fmt.Sprintf("naming to %s", newTag))
		if err := i.opt.ImageTagger.TagImage(ctx, imageID, newTag); err != nil {
			return done(err)
		}
		done(nil)
	}
	out[exptypes.ExporterImageNameKey] = strings.Join(i.reposAndTags, ",")
	return nil
}
