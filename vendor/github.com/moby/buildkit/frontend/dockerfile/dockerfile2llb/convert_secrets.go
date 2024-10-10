package dockerfile2llb

import (
	"path"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/pkg/errors"
)

func dispatchSecret(d *dispatchState, m *instructions.Mount, loc []parser.Range) (llb.RunOption, error) {
	id := m.CacheID
	if m.Source != "" {
		id = m.Source
	}

	if id == "" {
		if m.Target == "" {
			return nil, errors.Errorf("one of source, target required")
		}
		id = path.Base(m.Target)
	}

	var target *string
	if m.Target != "" {
		target = &m.Target
	}

	if m.Env == nil {
		dest := m.Target
		if dest == "" {
			dest = "/run/secrets/" + path.Base(id)
		}
		target = &dest
	}

	if _, ok := d.outline.secrets[id]; !ok {
		d.outline.secrets[id] = secretInfo{
			location: loc,
			required: m.Required,
		}
	}

	opts := []llb.SecretOption{llb.SecretID(id)}

	if !m.Required {
		opts = append(opts, llb.SecretOptional)
	}
	if m.Env != nil {
		opts = append(opts, llb.SecretAsEnvName(*m.Env))
	}

	if m.UID != nil || m.GID != nil || m.Mode != nil {
		var uid, gid, mode int
		if m.UID != nil {
			uid = int(*m.UID)
		}
		if m.GID != nil {
			gid = int(*m.GID)
		}
		if m.Mode != nil {
			mode = int(*m.Mode)
		} else {
			mode = 0400
		}
		opts = append(opts, llb.SecretFileOpt(uid, gid, mode))
	}

	return llb.AddSecretWithDest(id, target, opts...), nil
}

// withSecretEnvMask returns an EnvGetter that masks secret values in the environment.
// This is not needed to hide actual secret values but to make it clear that the value is loaded from a secret.
func withSecretEnvMask(c *instructions.RunCommand, env shell.EnvGetter) shell.EnvGetter {
	ev := &llb.EnvList{}
	set := false
	mounts := instructions.GetMounts(c)
	for _, mount := range mounts {
		if mount.Type == instructions.MountTypeSecret {
			if mount.Env != nil {
				ev = ev.AddOrReplace(*mount.Env, "****")
				set = true
			}
		}
	}
	if !set {
		return env
	}
	return &secretEnv{
		base: env,
		env:  ev,
	}
}

type secretEnv struct {
	base shell.EnvGetter
	env  *llb.EnvList
}

func (s *secretEnv) Get(key string) (string, bool) {
	v, ok := s.env.Get(key)
	if ok {
		return v, true
	}
	return s.base.Get(key)
}

func (s *secretEnv) Keys() []string {
	bkeys := s.base.Keys()
	skeys := s.env.Keys()
	keys := make([]string, 0, len(bkeys)+len(skeys))
	for _, k := range bkeys {
		if _, ok := s.env.Get(k); !ok {
			keys = append(keys, k)
		}
	}
	keys = append(keys, skeys...)
	return keys
}
