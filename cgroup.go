package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
)

func (srv *Server) ContainerCgroup(name string, cgroupData *APICgroup, saveToFile bool) ([]APICgroupResponse, error) {
	if container := srv.runtime.Get(name); container != nil {
		utils.Debugf("Access cgroup on %s ReadSubsystem '%s', writeSubsustem '%s', with option saveToFile '%t'\n",
			name, cgroupData.ReadSubsystem, cgroupData.WriteSubsystem, saveToFile)

		if !container.State.IsRunning() {
			return nil, fmt.Errorf("The container %s is not running", name)
		}

		cgroupResponses := []APICgroupResponse{}

		// read
		for _, subsystem := range cgroupData.ReadSubsystem {
			var cgroupResponse APICgroupResponse
			cgroupResponse.Subsystem = subsystem
			output, err := container.GetCgroupSubsysem(subsystem)
			if err != nil {
				cgroupResponse.Err = output
				cgroupResponse.Status = 255
			} else {
				cgroupResponse.Out = output
				cgroupResponse.Status = 0
			}
			cgroupResponses = append(cgroupResponses, cgroupResponse)
		}

		// write
		for _, pair := range cgroupData.WriteSubsystem {
			var cgroupResponse APICgroupResponse
			cgroupResponse.Subsystem = pair.Key
			output, err := container.SetCgroupSubsysem(pair.Key, pair.Value)
			if err != nil {
				cgroupResponse.Err = output
				cgroupResponse.Status = 255
			} else {
				cgroupResponse.Out = output
				cgroupResponse.Status = 0
				if saveToFile {
					container.AddLXCConfig(pair.Key, pair.Value)
				}
			}
			cgroupResponses = append(cgroupResponses, cgroupResponse)
		}

		if saveToFile {
			if err := container.ToDisk(); err != nil {
				return cgroupResponses, err
			}

			if err := container.generateLXCConfig(); err != nil {
				return cgroupResponses, err
			}
		}

		return cgroupResponses, nil
	}
	return nil, fmt.Errorf("No such container: %s", name)
}
