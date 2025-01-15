package dockerui

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/platforms"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type BuildFunc func(ctx context.Context, platform *ocispecs.Platform, idx int) (r client.Reference, img, baseImg *dockerspec.DockerOCIImage, err error)

func (bc *Client) Build(ctx context.Context, fn BuildFunc) (*ResultBuilder, error) {
	res := client.NewResult()

	targets := make([]*ocispecs.Platform, 0, len(bc.TargetPlatforms))
	for _, p := range bc.TargetPlatforms {
		p := p
		targets = append(targets, &p)
	}
	if len(targets) == 0 {
		targets = append(targets, nil)
	}
	expPlatforms := &exptypes.Platforms{
		Platforms: make([]exptypes.Platform, len(targets)),
	}

	eg, ctx := errgroup.WithContext(ctx)

	for i, tp := range targets {
		i, tp := i, tp
		eg.Go(func() error {
			ref, img, baseImg, err := fn(ctx, tp, i)
			if err != nil {
				return err
			}

			config, err := json.Marshal(img)
			if err != nil {
				return errors.Wrapf(err, "failed to marshal image config")
			}

			var baseConfig []byte
			if baseImg != nil {
				baseConfig, err = json.Marshal(baseImg)
				if err != nil {
					return errors.Wrapf(err, "failed to marshal source image config")
				}
			}

			p := platforms.DefaultSpec()
			if tp != nil {
				p = *tp
			}

			// in certain conditions we allow input platform to be extended from base image
			if p.OS == "windows" && img.OS == p.OS {
				if p.OSVersion == "" && img.OSVersion != "" {
					p.OSVersion = img.OSVersion
				}
				if p.OSFeatures == nil && len(img.OSFeatures) > 0 {
					p.OSFeatures = append([]string{}, img.OSFeatures...)
				}
			}

			p = platforms.Normalize(p)
			k := platforms.FormatAll(p)

			if bc.MultiPlatformRequested {
				res.AddRef(k, ref)
				res.AddMeta(fmt.Sprintf("%s/%s", exptypes.ExporterImageConfigKey, k), config)
				if len(baseConfig) > 0 {
					res.AddMeta(fmt.Sprintf("%s/%s", exptypes.ExporterImageBaseConfigKey, k), baseConfig)
				}
			} else {
				res.SetRef(ref)
				res.AddMeta(exptypes.ExporterImageConfigKey, config)
				if len(baseConfig) > 0 {
					res.AddMeta(exptypes.ExporterImageBaseConfigKey, baseConfig)
				}
			}
			expPlatforms.Platforms[i] = exptypes.Platform{
				ID:       k,
				Platform: p,
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &ResultBuilder{
		Result:       res,
		expPlatforms: expPlatforms,
	}, nil
}

type ResultBuilder struct {
	*client.Result
	expPlatforms *exptypes.Platforms
}

func (rb *ResultBuilder) Finalize() (*client.Result, error) {
	dt, err := json.Marshal(rb.expPlatforms)
	if err != nil {
		return nil, err
	}
	rb.AddMeta(exptypes.ExporterPlatformsKey, dt)

	return rb.Result, nil
}

func (rb *ResultBuilder) EachPlatform(ctx context.Context, fn func(ctx context.Context, id string, p ocispecs.Platform) error) error {
	eg, ctx := errgroup.WithContext(ctx)
	for _, p := range rb.expPlatforms.Platforms {
		p := p
		eg.Go(func() error {
			return fn(ctx, p.ID, p.Platform)
		})
	}
	return eg.Wait()
}
