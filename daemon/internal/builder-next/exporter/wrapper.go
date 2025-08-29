package exporter

import (
	"context"
	"strings"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/moby/v2/daemon/internal/builder-next/exporter/overrides"

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

	if _, has := exporterAttrs[string(exptypes.OptKeyUnpack)]; !has {
		exporterAttrs[string(exptypes.OptKeyUnpack)] = "true"
	}
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
		i.processNamedCallback(ctx, out, desc)
	}

	return out, ref, nil
}

func (i *imageExporterInstanceWrapper) processNamedCallback(ctx context.Context, out map[string]string, desc ocispec.Descriptor) {
	// TODO(vvoland): Change to exptypes.ExporterImageNameKey when BuildKit v0.21 is vendored.
	imageName := out["image.name"]
	if imageName == "" {
		log.G(ctx).Warn("image named with empty image.name produced by buildkit")
		return
	}

	for _, name := range strings.Split(imageName, ",") {
		ref, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			// Shouldn't happen, but log if it does and continue.
			log.G(ctx).WithFields(log.Fields{
				"name":  name,
				"error": err,
			}).Warn("image named with invalid reference produced by buildkit")
			continue
		}

		if namedTagged, ok := reference.TagNameOnly(ref).(reference.NamedTagged); ok {
			i.callbacks.Named(ctx, namedTagged, desc)
		}
	}
}
