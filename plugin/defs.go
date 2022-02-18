package plugin // import "github.com/moby/moby/plugin"

import (
	"fmt"
	"strings"
	"sync"

	"github.com/moby/moby/pkg/plugins"
	v2 "github.com/moby/moby/plugin/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Store manages the plugin inventory in memory and on-disk
type Store struct {
	sync.RWMutex
	plugins  map[string]*v2.Plugin
	specOpts map[string][]SpecOpt
	/* handlers are necessary for transition path of legacy plugins
	 * to the new model. Legacy plugins use Handle() for registering an
	 * activation callback.*/
	handlers map[string][]func(string, *plugins.Client)
}

// NewStore creates a Store.
func NewStore() *Store {
	return &Store{
		plugins:  make(map[string]*v2.Plugin),
		specOpts: make(map[string][]SpecOpt),
		handlers: make(map[string][]func(string, *plugins.Client)),
	}
}

// SpecOpt is used for subsystems that need to modify the runtime spec of a plugin
type SpecOpt func(*specs.Spec)

// CreateOpt is used to configure specific plugin details when created
type CreateOpt func(p *v2.Plugin)

// WithSwarmService is a CreateOpt that flags the passed in a plugin as a plugin
// managed by swarm
func WithSwarmService(id string) CreateOpt {
	return func(p *v2.Plugin) {
		p.SwarmServiceID = id
	}
}

// WithEnv is a CreateOpt that passes the user-provided environment variables
// to the plugin container, de-duplicating variables with the same names case
// sensitively and only appends valid key=value pairs
func WithEnv(env []string) CreateOpt {
	return func(p *v2.Plugin) {
		effectiveEnv := make(map[string]string)
		for _, penv := range p.PluginObj.Config.Env {
			if penv.Value != nil {
				effectiveEnv[penv.Name] = *penv.Value
			}
		}
		for _, line := range env {
			if pair := strings.SplitN(line, "=", 2); len(pair) > 1 {
				effectiveEnv[pair[0]] = pair[1]
			}
		}
		p.PluginObj.Settings.Env = make([]string, len(effectiveEnv))
		i := 0
		for key, value := range effectiveEnv {
			p.PluginObj.Settings.Env[i] = fmt.Sprintf("%s=%s", key, value)
			i++
		}
	}
}

// WithSpecMounts is a SpecOpt which appends the provided mounts to the runtime spec
func WithSpecMounts(mounts []specs.Mount) SpecOpt {
	return func(s *specs.Spec) {
		s.Mounts = append(s.Mounts, mounts...)
	}
}
