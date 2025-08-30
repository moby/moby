package daemon

import (
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGetInspectData(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		test   func(d *Daemon, c *container.Container)
	}{
		{
			name:   "graph-driver",
			driver: "overlay2",
			test: func(d *Daemon, c *container.Container) {
				cfg := d.configStore.Load()

				_, err := d.getInspectData(&cfg.Config, c)
				assert.Check(t, is.ErrorContains(err, "RWLayer of container inspect-me is unexpectedly nil"))

				c.Dead = true

				response, err := d.getInspectData(&cfg.Config, c)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(response.GraphDriver.Name, "overlay2"))
			},
		},
		{
			name:   "snapshotter",
			driver: "overlayfs",
			test: func(d *Daemon, c *container.Container) {
				cfg := d.configStore.Load()

				response, err := d.getInspectData(&cfg.Config, c)
				assert.NilError(t, err)
				assert.Equal(t, response.StorageDriver.Type, "snapshotter")
				assert.Equal(t, response.StorageDriver.Name, "overlayfs")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &container.Container{
				ID:           "inspect-me",
				HostConfig:   &containertypes.HostConfig{},
				State:        container.NewState(),
				ExecCommands: container.NewExecStore(),
				Driver:       tc.driver,
			}

			d := &Daemon{
				linkIndex:       newLinkIndex(),
				usesSnapshotter: tc.driver == "overlayfs",
			}

			cfg := &configStore{
				Config: config.Config{
					CommonConfig: config.CommonConfig{
						GraphDriver: tc.driver,
					},
				},
			}
			d.configStore.Store(cfg)

			tc.test(d, c)
		})
	}
}
