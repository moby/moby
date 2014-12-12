package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerInspect(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	if container := daemon.Get(name); container != nil {
		container.Lock()
		defer container.Unlock()
		if job.GetenvBool("raw") {
			b, err := json.Marshal(&struct {
				*Container
				HostConfig *runconfig.HostConfig
			}{container, container.hostConfig})
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
		out.Set("Image", container.Image)
		out.SetJson("NetworkSettings", container.NetworkSettings)
		out.Set("ResolvConfPath", container.ResolvConfPath)
		out.Set("HostnamePath", container.HostnamePath)
		out.Set("HostsPath", container.HostsPath)
		out.Set("Name", container.Name)
		out.Set("Driver", container.Driver)
		out.Set("ExecDriver", container.ExecDriver)
		out.Set("MountLabel", container.MountLabel)
		out.Set("ProcessLabel", container.ProcessLabel)
		out.SetJson("Volumes", container.Volumes)
		out.SetJson("VolumesRW", container.VolumesRW)
		out.SetJson("AppArmorProfile", container.AppArmorProfile)

		if children, err := daemon.Children(container.Name); err == nil {
			for linkAlias, child := range children {
				container.hostConfig.Links = append(container.hostConfig.Links, fmt.Sprintf("%s:%s", child.Name, linkAlias))
			}
		}

		out.SetJson("HostConfig", container.hostConfig)

		container.hostConfig.Links = nil
		if _, err := out.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

func (daemon *Daemon) ContainerExecInspect(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s ID", job.Name)
	}
	id := job.Args[0]
	eConfig, err := daemon.getExecConfig(id)
	if err != nil {
		return job.Error(err)
	}

	b, err := json.Marshal(*eConfig)
	if err != nil {
		return job.Error(err)
	}
	job.Stdout.Write(b)
	return engine.StatusOK
}
