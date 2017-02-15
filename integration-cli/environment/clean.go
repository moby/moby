package environment

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	icmd "github.com/docker/docker/pkg/testutil/cmd"
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
	cleans := []struct {
		name      string
		protected map[string]struct{}
		listFn    func(ctx context.Context, apiClient client.APIClient) ([]string, error)
		removeFn  func(ctx context.Context, ID string) error
	}{
		{
			name:      "unpause-containers",
			protected: e.protectedElements.containers,
			listFn:    listPausedContainers,
			removeFn:  e.client.ContainerUnpause,
		},
		{
			name:      "containers",
			protected: e.protectedElements.containers,
			listFn:    listContainers,
			removeFn: func(ctx context.Context, ID string) error {
				err := e.client.ContainerRemove(ctx, ID, types.ContainerRemoveOptions{
					Force:         true,
					RemoveVolumes: true,
				})
				if err != nil {
					// FIXME(vdemeester) shouldn't be there...
					if strings.Contains(err.Error(), fmt.Sprintf("removal of container %s is already in progress", ID)) {
						t.Logf("Skipping removing container %s, already in progress", ID)
						return nil
					}
					return err
				}
				return nil
			},
		},
		{
			// FIXME(vdemeester) use the API instead.
			name:      "images",
			protected: e.protectedElements.images,
			listFn: func(ctx context.Context, apiClient client.APIClient) ([]string, error) {
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
					//if _, ok := protectedImages[imgTag]; !ok {
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
					//}
				}
				images := []string{}
				for key, _ := range imgMap {
					images = append(images, key)
				}
				return images, nil
			},
			removeFn: func(ctx context.Context, ID string) error {
				return icmd.RunCommand(dockerBinary, "rmi", "-fv", ID).Error
				//_, err := e.client.ImageRemove(ctx, ID, types.ImageRemoveOptions{
				//	Force:         true,
				//	PruneChildren: true,
				//})
				//return err
			},
		},
		{
			name:      "volumes",
			protected: e.protectedElements.volumes,
			listFn:    listVolumes,
			removeFn: func(ctx context.Context, ID string) error {
				return e.client.VolumeRemove(ctx, ID, true)
			},
		},
		{
			name:      "networks",
			protected: e.protectedElements.networks,
			listFn:    listNetworks,
			removeFn:  e.client.NetworkRemove,
		},
		{
			name:      "plugins",
			protected: e.protectedElements.plugins,
			listFn:    listPlugins,
			removeFn: func(ctx context.Context, ID string) error {
				return e.client.PluginRemove(ctx, ID, types.PluginRemoveOptions{
					Force: true,
				})
			},
		},
	}

	ctx := context.Background()
	for _, clean := range cleans {
		items, err := clean.listFn(ctx, e.client)
		if err != nil {
			t.Fatalf("error cleaning %s: %v", clean.name, err)
		}
		for _, item := range items {
			if _, isProtected := clean.protected[item]; isProtected {
				continue
			}
			if err := clean.removeFn(ctx, item); err != nil {
				t.Fatalf("error removing %s element %s : %v", clean.name, item, err)
			}
		}
	}
}
