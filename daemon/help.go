package daemon

import (
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/helpinfo"
)

func (daemon *Daemon) CmdHelpInfo(job *engine.Job) engine.Status {
	v := &engine.Env{}
	if len(job.Getenv("command")) > 0 {
		v.Set("command", job.Getenv("command"))
		if len(job.Getenv("flag")) > 0 {
			v.Set("flag", job.Getenv("flag"))
		} else {
			v.SetJson("flags", helpinfo.Flags(job.Getenv("command")))
		}
	} else {
		v.SetJson("commands", helpinfo.Commands())
	}
	if len(job.Getenv("command")) > 0 && len(job.Getenv("flag")) > 0 {
		v.SetJson("blurbs", helpinfo.Blurbs(job.Getenv("command"), job.Getenv("flag")))
	}
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
