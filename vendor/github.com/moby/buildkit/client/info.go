package client

import (
	"context"

	controlapi "github.com/moby/buildkit/api/services/control"
	apitypes "github.com/moby/buildkit/api/types"
	"github.com/pkg/errors"
)

type Info struct {
	BuildkitVersion BuildkitVersion
}

type BuildkitVersion struct {
	Package  string
	Version  string
	Revision string
}

func (c *Client) Info(ctx context.Context) (*Info, error) {
	res, err := c.controlClient().Info(ctx, &controlapi.InfoRequest{})
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
