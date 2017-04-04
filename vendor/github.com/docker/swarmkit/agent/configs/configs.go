package configs

import (
	"sync"

	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
)

// configs is a map that keeps all the currently available configs to the agent
// mapped by config ID.
type configs struct {
	mu sync.RWMutex
	m  map[string]*api.Config
}

// NewManager returns a place to store configs.
func NewManager() exec.ConfigsManager {
	return &configs{
		m: make(map[string]*api.Config),
	}
}

// Get returns a config by ID.  If the config doesn't exist, returns nil.
func (r *configs) Get(configID string) *api.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r, ok := r.m[configID]; ok {
		return r
	}
	return nil
}

// Add adds one or more configs to the config map.
func (r *configs) Add(configs ...api.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, config := range configs {
		r.m[config.ID] = config.Copy()
	}
}

// Remove removes one or more configs by ID from the config map. Succeeds
// whether or not the given IDs are in the map.
func (r *configs) Remove(configs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, config := range configs {
		delete(r.m, config)
	}
}

// Reset removes all the configs.
func (r *configs) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = make(map[string]*api.Config)
}

// taskRestrictedConfigsProvider restricts the ids to the task.
type taskRestrictedConfigsProvider struct {
	configs   exec.ConfigGetter
	configIDs map[string]struct{} // allow list of config ids
}

func (sp *taskRestrictedConfigsProvider) Get(configID string) *api.Config {
	if _, ok := sp.configIDs[configID]; !ok {
		return nil
	}

	return sp.configs.Get(configID)
}

// Restrict provides a getter that only allows access to the configs
// referenced by the task.
func Restrict(configs exec.ConfigGetter, t *api.Task) exec.ConfigGetter {
	cids := map[string]struct{}{}

	container := t.Spec.GetContainer()
	if container != nil {
		for _, configRef := range container.Configs {
			cids[configRef.ConfigID] = struct{}{}
		}
	}

	return &taskRestrictedConfigsProvider{configs: configs, configIDs: cids}
}
