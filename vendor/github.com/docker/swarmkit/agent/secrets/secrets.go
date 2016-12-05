package secrets

import (
	"sync"

	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
)

// secrets is a map that keeps all the currently available secrets to the agent
// mapped by secret ID
type secrets struct {
	mu sync.RWMutex
	m  map[string]*api.Secret
}

// NewManager returns a place to store secrets.
func NewManager() exec.SecretsManager {
	return &secrets{
		m: make(map[string]*api.Secret),
	}
}

// Get returns a secret by ID.  If the secret doesn't exist, returns nil.
func (s *secrets) Get(secretID string) *api.Secret {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s, ok := s.m[secretID]; ok {
		return s
	}
	return nil
}

// add adds one or more secrets to the secret map
func (s *secrets) Add(secrets ...api.Secret) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, secret := range secrets {
		s.m[secret.ID] = secret.Copy()
	}
}

// remove removes one or more secrets by ID from the secret map.  Succeeds
// whether or not the given IDs are in the map.
func (s *secrets) Remove(secrets []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, secret := range secrets {
		delete(s.m, secret)
	}
}

// reset removes all the secrets
func (s *secrets) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = make(map[string]*api.Secret)
}

// taskRestrictedSecretsProvider restricts the ids to the task.
type taskRestrictedSecretsProvider struct {
	secrets   exec.SecretGetter
	secretIDs map[string]struct{} // allow list of secret ids
}

func (sp *taskRestrictedSecretsProvider) Get(secretID string) *api.Secret {
	if _, ok := sp.secretIDs[secretID]; !ok {
		return nil
	}

	return sp.secrets.Get(secretID)
}

// Restrict provides a getter that only allows access to the secrets
// referenced by the task.
func Restrict(secrets exec.SecretGetter, t *api.Task) exec.SecretGetter {
	sids := map[string]struct{}{}

	container := t.Spec.GetContainer()
	if container != nil {
		for _, ref := range container.Secrets {
			sids[ref.SecretID] = struct{}{}
		}
	}

	return &taskRestrictedSecretsProvider{secrets: secrets, secretIDs: sids}
}
