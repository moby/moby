package container

import (
	"fmt"
	"golang.org/x/net/context"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
)

func ipAppend(conf *container.Config) error {
	ipdev, ok := conf.Labels["upm.ip"]
	if ok {
		strs := strings.SplitN(ipdev, ":", 2)
		if len(strs) != 2 {
			return fmt.Errorf("ip label formate is wrong!")
		}
		ip := strs[0]
		dev := strs[1]
		if findIp(ip, dev) {
			return nil
		}
		script := fmt.Sprintf("ip addr add %s dev %s", ip, dev)
		command := exec.Command("/bin/bash", "-c", script)
		if err := command.Run(); err != nil {
			return err
		}
	}
	return nil
}

func ipRemove(conf *container.Config) error {
	ipdev, ok := conf.Labels["upm.ip"]
	if ok {
		strs := strings.SplitN(ipdev, ":", 2)
		if len(strs) != 2 {
			return fmt.Errorf("ip label formate is wrong!")
		}
		ip := strs[0]
		dev := strs[1]
		script := fmt.Sprintf("ip addr del %s dev %s", ip, dev)
		command := exec.Command("/bin/bash", "-c", script)
		if err := command.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (s *containerRouter) inspectContainer(ctx context.Context, name string, version version.Version) (*container.Config, bool) {
	json, err := s.backend.ContainerInspect(name, false, version)
	if err != nil {
		fmt.Println(err)
		return nil, false
	}
	config, ok := json.(*types.ContainerJSON)
	if ok {
		return config.Config, true
	}
	return nil, false
}

func processConfigAppendip(cr *containerRouter, ctx context.Context, name string) error {
	version := httputils.VersionFromContext(ctx)
	config, ok := cr.inspectContainer(ctx, name, version)
	if !ok {
		return fmt.Errorf("can't find the container")
	}
	return ipAppend(config)
}

func processConfigRemoveip(cr *containerRouter, ctx context.Context, name string) error {
	version := httputils.VersionFromContext(ctx)
	config, ok := cr.inspectContainer(ctx, name, version)
	if !ok {
		return fmt.Errorf("can't find the container")
	}
	return ipRemove(config)
}

func findIp(ip, dev string) bool {

	script := fmt.Sprintf("ip addr show %s", dev)
	command := exec.Command("/bin/bash", "-c", script)
	out, err := command.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), ip)
}
