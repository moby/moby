package environment

import (
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gotestyourself/gotestyourself/icmd"
	"golang.org/x/net/context"
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
	cli, err := client.NewEnvClient()
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer cli.Close()

	if (e.DaemonPlatform() != "windows") || (e.DaemonPlatform() == "windows" && e.Isolation() == "hyperv") {
		unpauseAllContainers(t, dockerBinary)
	}
	deleteAllContainers(t, dockerBinary)
	deleteAllImages(t, dockerBinary, e.protectedElements.images)
	deleteAllVolumes(t, cli)
	deleteAllNetworks(t, cli, e.DaemonPlatform())
	if e.DaemonPlatform() == "linux" {
		deleteAllPlugins(t, cli, dockerBinary)
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

var alreadyExists = regexp.MustCompile(`Error response from daemon: removal of container (\w+) is already in progress`)

func deleteAllContainers(t testingT, dockerBinary string) {
	containers := getAllContainers(t, dockerBinary)
	if len(containers) > 0 {
		result := icmd.RunCommand(dockerBinary, append([]string{"rm", "-fv"}, containers...)...)
		if result.Error != nil {
			// If the error is "No such container: ..." this means the container doesn't exists anymore,
			// or if it is "... removal of container ... is already in progress" it will be removed eventually.
			// We can safely ignore those.
			if strings.Contains(result.Stderr(), "No such container") || alreadyExists.MatchString(result.Stderr()) {
				return
			}
			t.Fatalf("error removing containers %v : %v (%s)", containers, result.Error, result.Combined())
		}
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

func deleteAllVolumes(t testingT, c client.APIClient) {
	var errs []string
	volumes, err := getAllVolumes(c)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, v := range volumes {
		err := c.VolumeRemove(context.Background(), v.Name, true)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
	}
	if len(errs) > 0 {
		t.Fatalf("%v", strings.Join(errs, "\n"))
	}
}

func getAllVolumes(c client.APIClient) ([]*types.Volume, error) {
	volumes, err := c.VolumeList(context.Background(), filters.Args{})
	if err != nil {
		return nil, err
	}
	return volumes.Volumes, nil
}

func deleteAllNetworks(t testingT, c client.APIClient, daemonPlatform string) {
	networks, err := getAllNetworks(c)
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
		err := c.NetworkRemove(context.Background(), n.ID)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
	}
	if len(errs) > 0 {
		t.Fatalf("%v", strings.Join(errs, "\n"))
	}
}

func getAllNetworks(c client.APIClient) ([]types.NetworkResource, error) {
	networks, err := c.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}
	return networks, nil
}

func deleteAllPlugins(t testingT, c client.APIClient, dockerBinary string) {
	plugins, err := getAllPlugins(c)
	if err != nil {
		t.Fatalf("%v", err)
	}
	var errs []string
	for _, p := range plugins {
		err := c.PluginRemove(context.Background(), p.Name, types.PluginRemoveOptions{Force: true})
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
	}
	if len(errs) > 0 {
		t.Fatalf("%v", strings.Join(errs, "\n"))
	}
}

func getAllPlugins(c client.APIClient) (types.PluginsListResponse, error) {
	plugins, err := c.PluginList(context.Background(), filters.Args{})
	if err != nil {
		return nil, err
	}
	return plugins, nil
}
