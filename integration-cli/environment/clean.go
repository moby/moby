package environment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/opts"
	icmd "github.com/docker/docker/pkg/testutil/cmd"
)

type testingT interface {
	logT
	Fatalf(string, ...interface{})
}

type logT interface {
	Logf(string, ...interface{})
}

// Clean the environment, preserving protected objects (images, containers, ...)
// and removing everything else. It's meant to run after any tests so that they don't
// depend on each others.
func (e *Execution) Clean(t testingT, dockerBinary string) {
	unpauseAllContainers(t, dockerBinary)
	deleteAllContainers(t, dockerBinary)
	deleteAllImages(t, dockerBinary, e.protectedElements.images)
	deleteAllVolumes(t, dockerBinary)
	deleteAllNetworks(t, dockerBinary, e.DaemonPlatform())
	if e.DaemonPlatform() == "linux" {
		deleteAllPlugins(t, dockerBinary)
	}
}

func unpauseAllContainers(t testingT, dockerBinary string) {
	containers := getPausedContainers(t, dockerBinary)
	if len(containers) > 0 {
		icmd.RunCommand(dockerBinary, append([]string{"unpause"}, containers...)...).Assert(t, icmd.Success)
	}
}

func getPausedContainers(t testingT, dockerBinary string) []string {
	result := icmd.RunCommand(dockerBinary, "ps", "-f", "status=paused", "-q", "-a")
	result.Assert(t, icmd.Success)
	return strings.Fields(result.Combined())
}

func deleteAllContainers(t testingT, dockerBinary string) {
	containers := getAllContainers(t, dockerBinary)
	if len(containers) > 0 {
		icmd.RunCommand(dockerBinary, append([]string{"rm", "-fv"}, containers...)...).Assert(t, icmd.Success)
	}
}

func getAllContainers(t testingT, dockerBinary string) []string {
	result := icmd.RunCommand(dockerBinary, "ps", "-q", "-a")
	result.Assert(t, icmd.Success)
	return strings.Fields(result.Combined())
}

func deleteAllImages(t testingT, dockerBinary string, protectedImages map[string]struct{}) {
	result := icmd.RunCommand(dockerBinary, "images", "--digests")
	result.Assert(t, icmd.Success)
	lines := strings.Split(string(result.Combined()), "\n")[1:]
	imgMap := map[string]struct{}{}
	for _, l := range lines {
		if l == "" {
			continue
		}
		fields := strings.Fields(l)
		imgTag := fields[0] + ":" + fields[1]
		if _, ok := protectedImages[imgTag]; !ok {
			if fields[0] == "<none>" || fields[1] == "<none>" {
				if fields[2] != "<none>" {
					imgMap[fields[0]+"@"+fields[2]] = struct{}{}
				} else {
					imgMap[fields[3]] = struct{}{}
				}
				// continue
			} else {
				imgMap[imgTag] = struct{}{}
			}
		}
	}
	if len(imgMap) != 0 {
		imgs := make([]string, 0, len(imgMap))
		for k := range imgMap {
			imgs = append(imgs, k)
		}
		icmd.RunCommand(dockerBinary, append([]string{"rmi", "-f"}, imgs...)...).Assert(t, icmd.Success)
	}
}

func deleteAllVolumes(t testingT, dockerBinary string) {
	volumes, err := getAllVolumes()
	if err != nil {
		t.Fatalf("%v", err)
	}
	var errs []string
	for _, v := range volumes {
		status, b, err := request.SockRequest("DELETE", "/volumes/"+v.Name, nil, DaemonHost())
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if status != http.StatusNoContent {
			errs = append(errs, fmt.Sprintf("error deleting volume %s: %s", v.Name, string(b)))
		}
	}
	if len(errs) > 0 {
		t.Fatalf("%v", strings.Join(errs, "\n"))
	}
}

func getAllVolumes() ([]*types.Volume, error) {
	var volumes volumetypes.VolumesListOKBody
	_, b, err := request.SockRequest("GET", "/volumes", nil, DaemonHost())
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &volumes); err != nil {
		return nil, err
	}
	return volumes.Volumes, nil
}

func deleteAllNetworks(t testingT, dockerBinary string, daemonPlatform string) {
	networks, err := getAllNetworks()
	if err != nil {
		t.Fatalf("%v", err)
	}
	var errs []string
	for _, n := range networks {
		if n.Name == "bridge" || n.Name == "none" || n.Name == "host" {
			continue
		}
		if daemonPlatform == "windows" && strings.ToLower(n.Name) == "nat" {
			// nat is a pre-defined network on Windows and cannot be removed
			continue
		}
		status, b, err := request.SockRequest("DELETE", "/networks/"+n.Name, nil, DaemonHost())
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if status != http.StatusNoContent {
			errs = append(errs, fmt.Sprintf("error deleting network %s: %s", n.Name, string(b)))
		}
	}
	if len(errs) > 0 {
		t.Fatalf("%v", strings.Join(errs, "\n"))
	}
}

func getAllNetworks() ([]types.NetworkResource, error) {
	var networks []types.NetworkResource
	_, b, err := request.SockRequest("GET", "/networks", nil, DaemonHost())
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &networks); err != nil {
		return nil, err
	}
	return networks, nil
}

func deleteAllPlugins(t testingT, dockerBinary string) {
	plugins, err := getAllPlugins()
	if err != nil {
		t.Fatalf("%v", err)
	}
	var errs []string
	for _, p := range plugins {
		pluginName := p.Name
		status, b, err := request.SockRequest("DELETE", "/plugins/"+pluginName+"?force=1", nil, DaemonHost())
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if status != http.StatusOK {
			errs = append(errs, fmt.Sprintf("error deleting plugin %s: %s", p.Name, string(b)))
		}
	}
	if len(errs) > 0 {
		t.Fatalf("%v", strings.Join(errs, "\n"))
	}
}

func getAllPlugins() (types.PluginsListResponse, error) {
	var plugins types.PluginsListResponse
	_, b, err := request.SockRequest("GET", "/plugins", nil, DaemonHost())
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

// DaemonHost return the daemon host string for this test execution
func DaemonHost() string {
	daemonURLStr := "unix://" + opts.DefaultUnixSocket
	if daemonHostVar := os.Getenv("DOCKER_HOST"); daemonHostVar != "" {
		daemonURLStr = daemonHostVar
	}
	return daemonURLStr
}
