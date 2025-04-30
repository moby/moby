package dockerui

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

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

			var p ocispecs.Platform
			if tp != nil {
				p = *tp
			} else {
				p = platforms.DefaultSpec()
			}
			expPlat := makeExportPlatform(p, img.Platform)
			if bc.MultiPlatformRequested {
				res.AddRef(expPlat.ID, ref)
				res.AddMeta(fmt.Sprintf("%s/%s", exptypes.ExporterImageConfigKey, expPlat.ID), config)
				if len(baseConfig) > 0 {
					res.AddMeta(fmt.Sprintf("%s/%s", exptypes.ExporterImageBaseConfigKey, expPlat.ID), baseConfig)
				}
			} else {
				res.SetRef(ref)
				res.AddMeta(exptypes.ExporterImageConfigKey, config)
				if len(baseConfig) > 0 {
					res.AddMeta(exptypes.ExporterImageBaseConfigKey, baseConfig)
				}
			}
			expPlatforms.Platforms[i] = expPlat
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

func extendWindowsPlatform(p, imgP ocispecs.Platform) ocispecs.Platform {
	// in certain conditions we allow input platform to be extended from base image
	if p.OS == "windows" && imgP.OS == p.OS {
		if p.OSVersion == "" && imgP.OSVersion != "" {
			p.OSVersion = imgP.OSVersion
		}
		if p.OSFeatures == nil && len(imgP.OSFeatures) > 0 {
			p.OSFeatures = slices.Clone(imgP.OSFeatures)
		}
	}
	return p
}

func makeExportPlatform(p, imgP ocispecs.Platform) exptypes.Platform {
	p = platforms.Normalize(p)
	exp := exptypes.Platform{
		ID: platforms.FormatAll(p),
	}
	if p.OS == "windows" {
		p = extendWindowsPlatform(p, imgP)
		p = platforms.Normalize(p)
	}
	exp.Platform = p
	return exp
}
