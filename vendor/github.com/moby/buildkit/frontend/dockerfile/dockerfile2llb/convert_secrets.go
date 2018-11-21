// +build dfsecrets

package dockerfile2llb

import (
	"path"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/pkg/errors"
)

func dispatchSecret(m *instructions.Mount) (llb.RunOption, error) {
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

	target := m.Target
	if target == "" {
		target = "/run/secrets/" + path.Base(id)
	}

	opts := []llb.SecretOption{llb.SecretID(id)}

	if !m.Required {
		opts = append(opts, llb.SecretOptional)
	}

	return llb.AddSecret(target, opts...), nil
}
