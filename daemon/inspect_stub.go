// +build !experimental

package daemon

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
)

func addExperimentalState(container *container.Container, data *types.ContainerStateBase) *types.ContainerState {
	return &types.ContainerState{ContainerStateBase: *data}
}
