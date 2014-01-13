package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"strconv"
	"strings"
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
				cgroupResponse.Err = strings.TrimSuffix(output, "\n")
				cgroupResponse.Status = 255
			} else {
				cgroupResponse.Out = strings.TrimSuffix(output, "\n")
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
				cgroupResponse.Err = strings.TrimSuffix(output, "\n")
				cgroupResponse.Status = 255
			} else {
				cgroupResponse.Out = strings.TrimSuffix(output, "\n")
				cgroupResponse.Status = 0
				if saveToFile {
					addLXCConfig(container, pair.Key, pair.Value)
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

func addLXCConfig(container *Container, subsystem string, value string) error {
	findAndUpdate := false
	for i, _ := range container.hostConfig.LxcConf {
		if strings.HasSuffix(container.hostConfig.LxcConf[i].Key, subsystem) {
			if isInBytesSubsystem(subsystem) {
				parsedValue, err := utils.RAMInBytes(value)
				if err != nil {
					return err
				}
				value = strconv.FormatInt(parsedValue, 10)
			}
			container.hostConfig.LxcConf[i].Value = value
			findAndUpdate = true
		}
	}
	if !findAndUpdate {
		var kvPair KeyValuePair
		kvPair.Key = "lxc.cgroup." + subsystem
		if isInBytesSubsystem(subsystem) {
			parsedValue, err := utils.RAMInBytes(value)
			if err != nil {
				return err
			}
			value = strconv.FormatInt(parsedValue, 10)
		}
		kvPair.Value = value
		container.hostConfig.LxcConf = append(container.hostConfig.LxcConf, kvPair)
	}
	return nil
}

func isInBytesSubsystem(subsystem string) bool {
	if strings.Contains(subsystem, "in_bytes") {
		return true
	}
	return false
}
