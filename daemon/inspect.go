package daemon

import (
	"encoding/json"

	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/runconfig"
)

func (daemon *Daemon) ContainerInspect(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	if container := daemon.Get(name); container != nil {
		if job.GetenvBool("dirty") {
			b, err := json.Marshal(&struct {
				*Container
				HostConfig *runconfig.HostConfig
			}{container, container.HostConfig()})
			if err != nil {
				return job.Error(err)
			}
			job.Stdout.Write(b)
			return engine.StatusOK
		}

		out := &engine.Env{}
		out.Set("Id", container.ID)
		out.SetAuto("Created", container.Created)
		out.SetJson("Path", container.Path)
		out.SetList("Args", container.Args)
		out.SetJson("Config", container.Config)
		out.SetJson("State", container.State)
		out.SetJson("Image", container.Image)
		out.SetJson("NetworkSettings", container.NetworkSettings)
		out.SetJson("ResolvConfPath", container.ResolvConfPath)
		out.SetJson("HostnamePath", container.HostnamePath)
		out.SetJson("HostsPath", container.HostsPath)
		out.SetJson("Name", container.Name)
		out.SetJson("Driver", container.Driver)
		out.SetJson("ExecDriver", container.ExecDriver)
		out.SetJson("MountLabel", container.MountLabel)
		out.SetJson("ProcessLabel", container.ProcessLabel)
		out.SetJson("Volumes", container.Volumes)
		out.SetJson("VolumesRW", container.VolumesRW)
		out.SetJson("HostConfig", container.hostConfig)
		if _, err := out.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}
