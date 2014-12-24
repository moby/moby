package daemon

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
)

func (daemon *Daemon) ContainerLimit(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
	)

	if container := daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container already stopped")
		}
		memory := job.GetenvInt64("memory")
		cpuShares := job.GetenvInt64("cpuShares")
		cpuset := job.Getenv("cpuset")
		saveChanges := job.GetenvBool("saveChanges")
		log.Debugf("Memory: %v, CpuShares: %v, Cpuset: %v.", memory, cpuShares, cpuset)
		c := &cgroups.Cgroup{
			Name:       container.ID,
			Parent:     daemon.ExecutionDriver().Parent(),
			Memory:     memory,
			CpuShares:  cpuShares,
			CpusetCpus: cpuset,
		}
		if _, err := fs.SetResources(c, container.Pid); err != nil {
			return job.Errorf("%v: Failed to change resources: %v", container.ID, err)
		}
		if saveChanges {
			if c.Memory != 0 {
				container.Config.Memory = c.Memory
			}
			if c.CpuShares != 0 {
				container.Config.CpuShares = c.CpuShares
			}
			if c.CpusetCpus != "" {
				container.Config.Cpuset = c.CpusetCpus
			}
			if err := container.ToDisk(); err != nil {
				return job.Errorf("%v: Failed to save changes: %v", container.ID, err)
			}
		}
	} else {
		return job.Errorf("No such container: %s\n", name)
	}
	return engine.StatusOK
}
