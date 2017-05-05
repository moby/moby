package secret

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type fakeClient struct {
	client.Client
	secretCreateFunc  func(swarm.SecretSpec) (types.SecretCreateResponse, error)
	secretInspectFunc func(string) (swarm.Secret, []byte, error)
	secretListFunc    func(types.SecretListOptions) ([]swarm.Secret, error)
	secretRemoveFunc  func(string) error
}

func (c *fakeClient) SecretCreate(ctx context.Context, spec swarm.SecretSpec) (types.SecretCreateResponse, error) {
	if c.secretCreateFunc != nil {
		return c.secretCreateFunc(spec)
	}
	return types.SecretCreateResponse{}, nil
}

func (c *fakeClient) SecretInspectWithRaw(ctx context.Context, id string) (swarm.Secret, []byte, error) {
	if c.secretInspectFunc != nil {
		return c.secretInspectFunc(id)
	}
	return swarm.Secret{}, nil, nil
}

func (c *fakeClient) SecretList(ctx context.Context, options types.SecretListOptions) ([]swarm.Secret, error) {
	if c.secretListFunc != nil {
		return c.secretListFunc(options)
	}
	return []swarm.Secret{}, nil
}

func (c *fakeClient) SecretRemove(ctx context.Context, name string) error {
	if c.secretRemoveFunc != nil {
		return c.secretRemoveFunc(name)
	}
	return nil
}
