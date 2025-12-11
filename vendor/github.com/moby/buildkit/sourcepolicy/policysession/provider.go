package policysession

import (
	context "context"

	pb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/pkg/errors"
	grpc "google.golang.org/grpc"
)

type PolicyCallback func(context.Context, *CheckPolicyRequest) (*DecisionResponse, *pb.ResolveSourceMetaRequest, error)

type PolicyProvider struct {
	f PolicyCallback
}

func NewPolicyProvider(f PolicyCallback) *PolicyProvider {
	return &PolicyProvider{
		f: f,
	}
}

func (p *PolicyProvider) CheckPolicy(ctx context.Context, req *CheckPolicyRequest) (*CheckPolicyResponse, error) {
	decision, metareq, err := p.f(ctx, req)
	if err != nil {
		return nil, err
	}
	if metareq != nil && decision != nil {
		return nil, errors.Errorf("cannot return both decision and meta request")
	}
	resp := &CheckPolicyResponse{}
	if decision != nil {
		resp.Result = &CheckPolicyResponse_Decision{
			Decision: decision,
		}
	} else if metareq != nil {
		resp.Result = &CheckPolicyResponse_Request{
			Request: metareq,
		}
	}
	return resp, nil
}

func (p *PolicyProvider) Register(server *grpc.Server) {
	RegisterPolicyVerifierServer(server, p)
}
