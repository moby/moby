package client

import (
	"context"

	controlapi "github.com/moby/buildkit/api/services/control"
	apitypes "github.com/moby/buildkit/api/types"
	"github.com/pkg/errors"
)

type Info struct {
	BuildkitVersion BuildkitVersion `json:"buildkitVersion"`
}

type BuildkitVersion struct {
	Package  string `json:"package"`
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

type CDIDevice struct {
	Name        string            `json:"name"`
	AutoAllow   bool              `json:"autoAllow"`
	Annotations map[string]string `json:"annotations"`
	OnDemand    bool              `json:"onDemand"`
}

func (c *Client) Info(ctx context.Context) (*Info, error) {
	res, err := c.ControlClient().Info(ctx, &controlapi.InfoRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to call info")
	}
	return &Info{
		BuildkitVersion: fromAPIBuildkitVersion(res.BuildkitVersion),
	}, nil
}

func fromAPIBuildkitVersion(in *apitypes.BuildkitVersion) BuildkitVersion {
	if in == nil {
		return BuildkitVersion{}
	}
	return BuildkitVersion{
		Package:  in.Package,
		Version:  in.Version,
		Revision: in.Revision,
	}
}

func fromAPICDIDevices(in []*apitypes.CDIDevice) []CDIDevice {
	var out []CDIDevice
	for _, d := range in {
		out = append(out, CDIDevice{
			Name:        d.Name,
			AutoAllow:   d.AutoAllow,
			Annotations: d.Annotations,
			OnDemand:    d.OnDemand,
		})
	}
	return out
}
