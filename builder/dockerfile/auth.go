package dockerfile

import (
	"context"

	"github.com/docker/docker/api/types"
)

type staticAuthConfigProvider struct {
	auths map[string]types.AuthConfig
}
type nilAuthConfigProvider struct {
}

func (p *nilAuthConfigProvider) GetRegistryAuth(ctx context.Context, registry string) (*types.AuthConfig, error) {
	return nil, nil
}
func (p *staticAuthConfigProvider) GetRegistryAuth(ctx context.Context, registry string) (*types.AuthConfig, error) {
	if a, ok := p.auths[registry]; ok {
		return &a, nil
	}
	return nil, nil
}
