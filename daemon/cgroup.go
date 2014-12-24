package daemon

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/libcontainer/cgroups/fs"
)

func (daemon *Daemon) ContainerCgroup(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name           = job.Args[0]
		readSubsystem  = job.GetenvList("readSubsystem")
		writeSubsystem []struct {
			Key   string
			Value string
		}
	)

	job.GetenvJson("writeSubsystem", &writeSubsystem)

	log.Debugf("name %s, readSubsystem %s, writeSubsystem %s", name, readSubsystem, writeSubsystem)

	if container := daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container %s is not running", name)
		}

		var object []interface{}

		// read
		for _, subsystem := range readSubsystem {
			var cgroupResponse struct {
				Subsystem string
				Out       string
				Err       string
				Status    int
			}

			cgroupResponse.Subsystem = subsystem
			output, err := fs.Get(container.ID, daemon.ExecutionDriver().Parent(), subsystem)
			if err != nil {
				cgroupResponse.Err = fmt.Sprintf("%s", err)
				cgroupResponse.Status = 255
			} else {
				cgroupResponse.Out = output
				cgroupResponse.Status = 0
			}
			object = append(object, cgroupResponse)
		}

		// write
		for _, pair := range writeSubsystem {
			var cgroupResponse struct {
				Subsystem string
				Out       string
				Err       string
				Status    int
			}

			cgroupResponse.Subsystem = pair.Key
			err := fs.Set(container.ID, daemon.ExecutionDriver().Parent(), pair.Key, pair.Value)
			if err != nil {
				cgroupResponse.Err = fmt.Sprintf("%s", err)
				cgroupResponse.Status = 255
			} else {
				cgroupResponse.Out = pair.Value
				cgroupResponse.Status = 0
			}
			object = append(object, cgroupResponse)
		}

		b, err := json.Marshal(object)
		if err != nil {
			return job.Error(err)
		}

		job.Stdout.Write(b)
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}
