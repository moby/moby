package imageutil

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type IngesterProvider interface {
	content.Ingester
	content.Provider
}

func Config(ctx context.Context, str string, resolver remotes.Resolver, ingester IngesterProvider, platform *specs.Platform) (digest.Digest, []byte, error) {
	// TODO: fix containerd to take struct instead of string
	platformStr := platforms.Default()
	if platform != nil {
		platformStr = platforms.Format(*platform)
	}
	ref, err := reference.Parse(str)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	desc := specs.Descriptor{
		Digest: ref.Digest(),
	}
	if desc.Digest != "" {
		ra, err := ingester.ReaderAt(ctx, desc)
		if err == nil {
			desc.Size = ra.Size()
			mt, err := DetectManifestMediaType(ra)
			if err == nil {
				desc.MediaType = mt
			}
		}
	}
	// use resolver if desc is incomplete
	if desc.MediaType == "" {
		_, desc, err = resolver.Resolve(ctx, ref.String())
		if err != nil {
			return "", nil, err
		}
	}

	fetcher, err := resolver.Fetcher(ctx, ref.String())
	if err != nil {
		return "", nil, err
	}

	handlers := []images.Handler{
		remotes.FetchHandler(ingester, fetcher),
		childrenConfigHandler(ingester, platformStr),
	}
	if err := images.Dispatch(ctx, images.Handlers(handlers...), desc); err != nil {
		return "", nil, err
	}
	config, err := images.Config(ctx, ingester, desc, platformStr)
	if err != nil {
		return "", nil, err
	}

	dt, err := content.ReadBlob(ctx, ingester, config)
	if err != nil {
		return "", nil, err
	}

	return desc.Digest, dt, nil
}

func childrenConfigHandler(provider content.Provider, platform string) images.HandlerFunc {
	return func(ctx context.Context, desc specs.Descriptor) ([]specs.Descriptor, error) {
		var descs []specs.Descriptor
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, specs.MediaTypeImageManifest:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			// TODO(stevvooe): We just assume oci manifest, for now. There may be
			// subtle differences from the docker version.
			var manifest specs.Manifest
			if err := json.Unmarshal(p, &manifest); err != nil {
				return nil, err
			}

			descs = append(descs, manifest.Config)
		case images.MediaTypeDockerSchema2ManifestList, specs.MediaTypeImageIndex:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			var index specs.Index
			if err := json.Unmarshal(p, &index); err != nil {
				return nil, err
			}

			if platform != "" {
				pf, err := platforms.Parse(platform)
				if err != nil {
					return nil, err
				}
				matcher := platforms.NewMatcher(pf)

				for _, d := range index.Manifests {
					if d.Platform == nil || matcher.Match(*d.Platform) {
						descs = append(descs, d)
					}
				}
			} else {
				descs = append(descs, index.Manifests...)
			}
		case images.MediaTypeDockerSchema2Config, specs.MediaTypeImageConfig:
			// childless data types.
			return nil, nil
		default:
			return nil, errors.Errorf("encountered unknown type %v; children may not be fetched", desc.MediaType)
		}

		return descs, nil
	}
}

// specs.MediaTypeImageManifest, // TODO: detect schema1/manifest-list
func DetectManifestMediaType(ra content.ReaderAt) (string, error) {
	// TODO: schema1

	p := make([]byte, ra.Size())
	if _, err := ra.ReadAt(p, 0); err != nil {
		return "", err
	}

	var mfst struct {
		MediaType string          `json:"mediaType"`
		Config    json.RawMessage `json:"config"`
	}

	if err := json.Unmarshal(p, &mfst); err != nil {
		return "", err
	}

	if mfst.MediaType != "" {
		return mfst.MediaType, nil
	}
	if mfst.Config != nil {
		return images.MediaTypeDockerSchema2Manifest, nil
	}
	return images.MediaTypeDockerSchema2ManifestList, nil
}
