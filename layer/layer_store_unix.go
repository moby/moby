// +build !windows

package layer

import "github.com/docker/docker/daemon/graphdriver"

func getApplyDiffOpts(opts *RegisterOpts) *graphdriver.ApplyDiffOpts {
	return &graphdriver.ApplyDiffOpts{}
}
