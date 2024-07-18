package exporter

import (
	"context"
	"strings"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/builder/builder-next/exporter/overrides"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type BuildkitCallbacks struct {
	// Exported is a Called when an image is exported by buildkit.
	Exported func(ctx context.Context, id string, desc ocispec.Descriptor)

	// Named is a callback that is called when an image is created in the
	// containerd image store by buildkit.
	Named func(ctx context.Context, ref reference.NamedTagged, desc ocispec.Descriptor)
}

// Wraps the containerimage exporter's Resolve method to apply moby-specific
// overrides to the exporter attributes.
type imageExporterMobyWrapper struct {
	exp       exporter.Exporter
	callbacks BuildkitCallbacks
}

// NewWrapper returns an exporter wrapper that applies moby specific attributes
// and hooks the export process.
func NewWrapper(exp exporter.Exporter, callbacks BuildkitCallbacks) (exporter.Exporter, error) {
	return &imageExporterMobyWrapper{
		exp:       exp,
		callbacks: callbacks,
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
	exporterAttrs[string(exptypes.OptKeyName)] = strings.Join(reposAndTags, ",")
	exporterAttrs[string(exptypes.OptKeyUnpack)] = "true"
	if _, has := exporterAttrs[string(exptypes.OptKeyDanglingPrefix)]; !has {
		exporterAttrs[string(exptypes.OptKeyDanglingPrefix)] = "moby-dangling"
	}

	inst, err := e.exp.Resolve(ctx, id, exporterAttrs)
	if err != nil {
		return nil, err
	}

	return &imageExporterInstanceWrapper{
		ExporterInstance: inst,
		callbacks:        e.callbacks,
	}, nil
}

type imageExporterInstanceWrapper struct {
	exporter.ExporterInstance
	callbacks BuildkitCallbacks
}

func (i *imageExporterInstanceWrapper) Export(ctx context.Context, src *exporter.Source, inlineCache exptypes.InlineCache, sessionID string) (map[string]string, exporter.DescriptorReference, error) {
	out, ref, err := i.ExporterInstance.Export(ctx, src, inlineCache, sessionID)
	if err != nil {
		return out, ref, err
	}

	desc := ref.Descriptor()
	imageID := out[exptypes.ExporterImageDigestKey]
	if i.callbacks.Exported != nil {
		i.callbacks.Exported(ctx, imageID, desc)
	}

	if i.callbacks.Named != nil {
		for _, name := range strings.Split(out[string(exptypes.OptKeyName)], ",") {
			ref, err := reference.ParseNormalizedNamed(name)
			if err != nil {
				// Shouldn't happen, but log if it does and continue.
				log.G(ctx).WithFields(log.Fields{
					"name":  name,
					"error": err,
				}).Warn("image named with invalid reference produced by buildkit")
				continue
			}

			namedTagged := reference.TagNameOnly(ref).(reference.NamedTagged)
			i.callbacks.Named(ctx, namedTagged, desc)
		}
	}

	return out, ref, nil
}
