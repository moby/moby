package policysession

import (
	context "context"

	"github.com/moby/buildkit/session"
)

func NewVerifier(ctx context.Context, sm *session.Manager, gid string) (*PolicyVerifier, error) {
	c, err := sm.Get(ctx, gid, false)
	if err != nil {
		return nil, err
	}
	client := NewPolicyVerifierClient(c.Conn())
	return &PolicyVerifier{
		client: client,
	}, nil
}

type PolicyVerifier struct {
	client PolicyVerifierClient
}

func (p *PolicyVerifier) Check(ctx context.Context, req *CheckPolicyRequest) (*CheckPolicyResponse, error) {
	resp, err := p.client.CheckPolicy(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
