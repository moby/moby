package daemon

import (
	"fmt"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/audit"
)

func (daemon *Daemon) ContainerStop(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
		t    = 10
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	if container := daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container already stopped")
		}
		if err := container.Stop(int(t)); err != nil {
			audit.AuditLogUserEvent(audit.AUDIT_VIRT_CONTROL, fmt.Sprintf("virt=docker op=stop  uuid=%s", container.ID), false)
			return job.Errorf("Cannot stop container %s: %s\n", name, err)
		}
		audit.AuditLogUserEvent(audit.AUDIT_VIRT_CONTROL, fmt.Sprintf("virt=docker op=stop uuid=%s", container.ID), true)
		container.LogEvent("stop")
	} else {
		return job.Errorf("No such container: %s\n", name)
	}
	return engine.StatusOK
}
