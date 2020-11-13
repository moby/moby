package dockerfile2llb

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/pkg/errors"
)

func dispatchSSH(m *instructions.Mount) (llb.RunOption, error) {
	if m.Source != "" {
		return nil, errors.Errorf("ssh does not support source")
	}
	opts := []llb.SSHOption{llb.SSHID(m.CacheID)}

	if m.Target != "" {
		opts = append(opts, llb.SSHSocketTarget(m.Target))
	}

	if !m.Required {
		opts = append(opts, llb.SSHOptional)
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
			mode = 0600
		}
		opts = append(opts, llb.SSHSocketOpt(m.Target, uid, gid, mode))
	}

	return llb.AddSSHSocket(opts...), nil
}
