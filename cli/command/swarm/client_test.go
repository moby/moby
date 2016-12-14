package swarm

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type fakeClient struct {
	client.Client
	infoFunc              func() (types.Info, error)
	swarmInitFunc         func() (string, error)
	swarmInspectFunc      func() (swarm.Swarm, error)
	nodeInspectFunc       func() (swarm.Node, []byte, error)
	swarmGetUnlockKeyFunc func() (types.SwarmUnlockKeyResponse, error)
	swarmJoinFunc         func() error
	swarmLeaveFunc        func() error
	swarmUpdateFunc       func(swarm swarm.Spec, flags swarm.UpdateFlags) error
	swarmUnlockFunc       func(req swarm.UnlockRequest) error
}

func (cli *fakeClient) Info(ctx context.Context) (types.Info, error) {
	if cli.infoFunc != nil {
		return cli.infoFunc()
	}
	return types.Info{}, nil
}

func (cli *fakeClient) NodeInspectWithRaw(ctx context.Context, ref string) (swarm.Node, []byte, error) {
	if cli.nodeInspectFunc != nil {
		return cli.nodeInspectFunc()
	}
	return swarm.Node{}, []byte{}, nil
}

func (cli *fakeClient) SwarmInit(ctx context.Context, req swarm.InitRequest) (string, error) {
	if cli.swarmInitFunc != nil {
		return cli.swarmInitFunc()
	}
	return "", nil
}

func (cli *fakeClient) SwarmInspect(ctx context.Context) (swarm.Swarm, error) {
	if cli.swarmInspectFunc != nil {
		return cli.swarmInspectFunc()
	}
	return swarm.Swarm{}, nil
}

func (cli *fakeClient) SwarmGetUnlockKey(ctx context.Context) (types.SwarmUnlockKeyResponse, error) {
	if cli.swarmGetUnlockKeyFunc != nil {
		return cli.swarmGetUnlockKeyFunc()
	}
	return types.SwarmUnlockKeyResponse{}, nil
}

func (cli *fakeClient) SwarmJoin(ctx context.Context, req swarm.JoinRequest) error {
	if cli.swarmJoinFunc != nil {
		return cli.swarmJoinFunc()
	}
	return nil
}

func (cli *fakeClient) SwarmLeave(ctx context.Context, force bool) error {
	if cli.swarmLeaveFunc != nil {
		return cli.swarmLeaveFunc()
	}
	return nil
}

func (cli *fakeClient) SwarmUpdate(ctx context.Context, version swarm.Version, swarm swarm.Spec, flags swarm.UpdateFlags) error {
	if cli.swarmUpdateFunc != nil {
		return cli.swarmUpdateFunc(swarm, flags)
	}
	return nil
}

func (cli *fakeClient) SwarmUnlock(ctx context.Context, req swarm.UnlockRequest) error {
	if cli.swarmUnlockFunc != nil {
		return cli.swarmUnlockFunc(req)
	}
	return nil
}
