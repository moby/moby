package decorators

import (
	"net/http"
	"time"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions/v1p19"
	"github.com/docker/docker/api/types/versions/v1p20"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/runconfig"
)

// inspectHandler defines a function to transform information from the daemon into information to consume in the API.
type inspectHandler func(container *daemon.Container, hostConfig *runconfig.HostConfig, graphData map[string]string) error

// InspectDecorator transforms the information coming from the daemon
// into information consumable by the API.
// It take versioning and platforms into account.
type InspectDecorator struct {
	version version.Version
	w       http.ResponseWriter
}

// NewInspectDecorator initializes a new decorator.
func NewInspectDecorator(version version.Version, w http.ResponseWriter) InspectDecorator {
	return InspectDecorator{version, w}
}

// HandleFunc transforms the information from the daemon into structs
// to be consumed by the API depending on the api version.
func (i InspectDecorator) HandleFunc(container *daemon.Container, hostConfig *runconfig.HostConfig, graphData map[string]string) error {
	base := getBaseContainer(container, hostConfig, graphData)
	mounts := getMountPoints(container)

	var json interface{}
	switch {
	case i.version.LessThan("1.20"):
		volumes := make(map[string]string)
		volumesRW := make(map[string]bool)
		for _, m := range mounts {
			volumes[m.Destination] = m.Source
			volumesRW[m.Destination] = m.RW
		}

		config := &v1p19.ContainerConfig{
			container.Config,
			hostConfig.VolumeDriver,
			hostConfig.Memory,
			hostConfig.MemorySwap,
			hostConfig.CPUShares,
			hostConfig.CpusetCpus,
		}

		json = &v1p19.ContainerJSON{base, volumes, volumesRW, config}
	case i.version.Equal("1.20"):
		config := &v1p20.ContainerConfig{
			container.Config,
			hostConfig.VolumeDriver,
		}

		json = &v1p20.ContainerJSON{base, mounts, config}
	default:
		json = &types.ContainerJSON{base, mounts, container.Config}
	}

	return httputils.WriteJSON(i.w, http.StatusOK, json)
}

// getBaseContainer creates a base struct representing a container.
// This is used by decorators to build complete structs to serialize.
func getBaseContainer(container *daemon.Container, hostConfig *runconfig.HostConfig, graphDriverData map[string]string) *types.ContainerJSONBase {
	containerState := &types.ContainerState{
		Status:     container.State.StateString(),
		Running:    container.State.Running,
		Paused:     container.State.Paused,
		Restarting: container.State.Restarting,
		OOMKilled:  container.State.OOMKilled,
		Dead:       container.State.Dead,
		Pid:        container.State.Pid,
		ExitCode:   container.State.ExitCode,
		Error:      container.State.Error,
		StartedAt:  container.State.StartedAt.Format(time.RFC3339Nano),
		FinishedAt: container.State.FinishedAt.Format(time.RFC3339Nano),
	}

	contJSONBase := &types.ContainerJSONBase{
		ID:              container.ID,
		Created:         container.Created.Format(time.RFC3339Nano),
		Path:            container.Path,
		Args:            container.Args,
		State:           containerState,
		Image:           container.ImageID,
		NetworkSettings: container.NetworkSettings,
		LogPath:         container.LogPath,
		Name:            container.Name,
		RestartCount:    container.RestartCount,
		Driver:          container.Driver,
		ExecDriver:      container.ExecDriver,
		MountLabel:      container.MountLabel,
		ProcessLabel:    container.ProcessLabel,
		ExecIDs:         container.GetExecIDs(),
		HostConfig:      hostConfig,
	}

	// Now set any platform-specific fields
	contJSONBase = setPlatformSpecificContainerFields(container, contJSONBase)
	contJSONBase.GraphDriver = types.GraphDriverData{
		Name: container.Driver,
		Data: graphDriverData,
	}

	return contJSONBase
}
