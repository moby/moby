package containerimage

import (
	"context"
	"strconv"
	"time"

	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/util/epoch"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	"github.com/pkg/errors"
)

type ImageCommitOpts struct {
	ImageName   string
	RefCfg      cacheconfig.RefConfig
	OCITypes    bool
	Annotations AnnotationsGroup
	Epoch       *time.Time

	ForceInlineAttestations bool // force inline attestations to be attached
}

func (c *ImageCommitOpts) Load(ctx context.Context, opt map[string]string) (map[string]string, error) {
	rest := make(map[string]string)

	as, optb, err := ParseAnnotations(toBytesMap(opt))
	if err != nil {
		return nil, err
	}
	opt = toStringMap(optb)

	c.Epoch, opt, err = epoch.ParseExporterAttrs(opt)
	if err != nil {
		return nil, err
	}

	if c.RefCfg.Compression, err = compression.ParseAttributes(opt); err != nil {
		return nil, err
	}

	for k, v := range opt {
		var err error
		switch exptypes.ImageExporterOptKey(k) {
		case exptypes.OptKeyName:
			c.ImageName = v
		case exptypes.OptKeyOCITypes:
			err = parseBoolWithDefault(&c.OCITypes, k, v, true)
		case exptypes.OptKeyForceInlineAttestations:
			err = parseBool(&c.ForceInlineAttestations, k, v)
		case exptypes.OptKeyPreferNondistLayers:
			err = parseBool(&c.RefCfg.PreferNonDistributable, k, v)
		default:
			rest[k] = v
		}

		if err != nil {
			return nil, err
		}
	}

	if c.RefCfg.Compression.Type.OnlySupportOCITypes() {
		c.EnableOCITypes(ctx, c.RefCfg.Compression.Type.String())
	}

	if c.RefCfg.Compression.Type.NeedsForceCompression() {
		c.EnableForceCompression(ctx, c.RefCfg.Compression.Type.String())
	}

	c.Annotations = c.Annotations.Merge(as)

	return rest, nil
}

func (c *ImageCommitOpts) EnableOCITypes(ctx context.Context, reason string) {
	if !c.OCITypes {
		message := "forcibly turning on oci-mediatype mode"
		if reason != "" {
			message += " for " + reason
		}
		bklog.G(ctx).Warn(message)

		c.OCITypes = true
	}
}

func (c *ImageCommitOpts) EnableForceCompression(ctx context.Context, reason string) {
	if !c.RefCfg.Compression.Force {
		message := "forcibly turning on force-compression mode"
		if reason != "" {
			message += " for " + reason
		}
		bklog.G(ctx).Warn(message)

		c.RefCfg.Compression.Force = true
	}
}

func parseBool(dest *bool, key string, value string) error {
	b, err := strconv.ParseBool(value)
	if err != nil {
		return errors.Wrapf(err, "non-bool value specified for %s", key)
	}
	*dest = b
	return nil
}

func parseBoolWithDefault(dest *bool, key string, value string, defaultValue bool) error {
	if value == "" {
		*dest = defaultValue
		return nil
	}
	return parseBool(dest, key, value)
}

func toBytesMap(m map[string]string) map[string][]byte {
	result := make(map[string][]byte)
	for k, v := range m {
		result[k] = []byte(v)
	}
	return result
}

func toStringMap(m map[string][]byte) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = string(v)
	}
	return result
}
