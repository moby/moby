// +build dfssh dfextall

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
		// TODO(AkihiroSuda): support specifying permission bits
		opts = append(opts, llb.SSHSocketTarget(m.Target))
	}

	if !m.Required {
		opts = append(opts, llb.SSHOptional)
	}

	return llb.AddSSHSocket(opts...), nil
}
