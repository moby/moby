package subrequests

import (
	"context"
	"encoding/json"

	"github.com/moby/buildkit/frontend/gateway/client"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/pkg/errors"
)

const RequestSubrequestsDescribe = "frontend.subrequests.describe"

var SubrequestsDescribeDefinition = Request{
	Name:        RequestSubrequestsDescribe,
	Version:     "1.0.0",
	Type:        TypeRPC,
	Description: "List available subrequest types",
	Metadata: []Named{
		{
			Name: "result.json",
		},
	},
}

func Describe(ctx context.Context, c client.Client) ([]Request, error) {
	gwcaps := c.BuildOpts().Caps

	if err := (&gwcaps).Supports(gwpb.CapFrontendCaps); err != nil {
		return nil, errdefs.NewUnsupportedSubrequestError(RequestSubrequestsDescribe)
	}

	res, err := c.Solve(ctx, client.SolveRequest{
		FrontendOpt: map[string]string{
			"requestid":     RequestSubrequestsDescribe,
			"frontend.caps": "moby.buildkit.frontend.subrequests",
		},
		Frontend: "dockerfile.v0",
	})
	if err != nil {
		var reqErr *errdefs.UnsupportedSubrequestError
		if errors.As(err, &reqErr) {
			return nil, err
		}
		var capErr *errdefs.UnsupportedFrontendCapError
		if errors.As(err, &capErr) {
			return nil, errdefs.NewUnsupportedSubrequestError(RequestSubrequestsDescribe)
		}
		return nil, err
	}

	dt, ok := res.Metadata["result.json"]
	if !ok {
		return nil, errors.Errorf("no result.json metadata in response")
	}

	var reqs []Request
	if err := json.Unmarshal(dt, &reqs); err != nil {
		return nil, errors.Wrap(err, "failed to parse describe result")
	}
	return reqs, nil
}
