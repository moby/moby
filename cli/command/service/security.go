package service

import (
	"fmt"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/flags"
)

func convertToSecurityConfig(opts map[string]string) (*swarm.SecurityConfig, error) {
	cfg := &swarm.SecurityConfig{}

	// set security opts
	for k, v := range opts {
		switch k {
		case flags.FlagUsernsMode:
			cfg.Userns = v
		case flags.FlagCredentialSpec:
			cfg.CredentialSpec = v
		default:
			return nil, fmt.Errorf("unsupported security option: %q", k)
		}
	}

	return cfg, nil
}
