package container

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	executorpkg "github.com/docker/docker/daemon/cluster/executor"
	"github.com/docker/engine-api/types"
	"github.com/docker/libnetwork"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
)

// containerAdapter conducts remote operations for a container. All calls
// are mostly naked calls to the client API, seeded with information from
// containerConfig.
type containerAdapter struct {
	backend   executorpkg.Backend
	container *containerConfig
}

func newContainerAdapter(b executorpkg.Backend, task *api.Task) (*containerAdapter, error) {
	ctnr, err := newContainerConfig(task)
	if err != nil {
		return nil, err
	}

	return &containerAdapter{
		container: ctnr,
		backend:   b,
	}, nil
}

func (c *containerAdapter) pullImage(ctx context.Context) error {
	// if the image needs to be pulled, the auth config will be retrieved and updated
	encodedAuthConfig := c.container.task.ServiceAnnotations.Labels[fmt.Sprintf("%v.registryauth", systemLabelPrefix)]

	authConfig := &types.AuthConfig{}
	if encodedAuthConfig != "" {
		if err := json.NewDecoder(base64.NewDecoder(base64.URLEncoding, strings.NewReader(encodedAuthConfig))).Decode(authConfig); err != nil {
			logrus.Warnf("invalid authconfig: %v", err)
		}
	}

	pr, pw := io.Pipe()
	metaHeaders := map[string][]string{}
	go func() {
		err := c.backend.PullImage(ctx, c.container.image(), "", metaHeaders, authConfig, pw)
		pw.CloseWithError(err)
	}()

	dec := json.NewDecoder(pr)
	m := map[string]interface{}{}
	for {
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		// TOOD(stevvooe): Report this status somewhere.
		logrus.Debugln("pull progress", m)
	}
	// if the final stream object contained an error, return it
	if errMsg, ok := m["error"]; ok {
		return fmt.Errorf("%v", errMsg)
	}
	return nil
}

func (c *containerAdapter) createNetworks(ctx context.Context) error {
	for _, network := range c.container.networks() {
		ncr, err := c.container.networkCreateRequest(network)
		if err != nil {
			return err
		}

		if err := c.backend.CreateAgentNetwork(ncr); err != nil { // todo name missing
			if _, ok := err.(libnetwork.NetworkNameError); ok {
				continue
			}

			return err
		}
	}

	return nil
}

func (c *containerAdapter) removeNetworks(ctx context.Context) error {
	for _, nid := range c.container.networks() {
		if err := c.backend.DeleteAgentNetwork(nid); err != nil {
			if _, ok := err.(*libnetwork.ActiveEndpointsError); ok {
				continue
			}
			log.G(ctx).Errorf("network %s remove failed: %v", nid, err)
			return err
		}
	}

	return nil
}

func (c *containerAdapter) create(ctx context.Context, backend executorpkg.Backend) error {
	var cr types.ContainerCreateResponse
	var err error
	if cr, err = backend.ContainerCreate(types.ContainerCreateConfig{
		Name:       c.container.name(),
		Config:     c.container.config(),
		HostConfig: c.container.hostConfig(),
		// Use the first network in container create
		NetworkingConfig: c.container.createNetworkingConfig(),
	}); err != nil {
		return err
	}

	// Docker daemon currently doesnt support multiple networks in container create
	// Connect to all other networks
	nc := c.container.connectNetworkingConfig()

	if nc != nil {
		for n, ep := range nc.EndpointsConfig {
			logrus.Errorf("CONNECT %s : %v", n, ep.IPAMConfig.IPv4Address)
			if err := backend.ConnectContainerToNetwork(cr.ID, n, ep); err != nil {
				return err
			}
		}
	}

	if err := backend.UpdateContainerServiceConfig(cr.ID, c.container.serviceConfig()); err != nil {
		return err
	}

	return nil
}

func (c *containerAdapter) start(ctx context.Context) error {
	return c.backend.ContainerStart(c.container.name(), nil)
}

func (c *containerAdapter) inspect(ctx context.Context) (types.ContainerJSON, error) {
	cs, err := c.backend.ContainerInspectCurrent(c.container.name(), false)
	if err != nil {
		return types.ContainerJSON{}, err
	}
	return *cs, nil
}

// events issues a call to the events API and returns a channel with all
// events. The stream of events can be shutdown by cancelling the context.
//
// A chan struct{} is returned that will be closed if the event procressing
// fails and needs to be restarted.
func (c *containerAdapter) wait(ctx context.Context) (<-chan int, error) {
	return c.backend.ContainerWaitWithContext(ctx, c.container.name())
}

func (c *containerAdapter) shutdown(ctx context.Context) error {
	return c.backend.ContainerStop(c.container.name(), int(c.container.spec().StopGracePeriod.Seconds))
}

func (c *containerAdapter) terminate(ctx context.Context) error {
	return c.backend.ContainerKill(c.container.name(), uint64(syscall.SIGKILL))
}

func (c *containerAdapter) remove(ctx context.Context) error {
	return c.backend.ContainerRm(c.container.name(), &types.ContainerRmConfig{
		RemoveVolume: true,
		ForceRemove:  true,
	})
}

func (c *containerAdapter) createVolumes(ctx context.Context, backend executorpkg.Backend) error {
	// Create plugin volumes that are embedded inside a Mount
	for _, mount := range c.container.task.Spec.GetContainer().Mounts {
		if mount.Template != nil {
			req := c.container.volumeCreateRequest(mount)

			// Check if this volume exists on the engine
			if _, err := backend.VolumeCreate(req.Name, req.Driver, req.DriverOpts, req.Labels); err != nil {
				// TODO(amitshukla): Today, volume create through the engine api does not return an error
				// when the named volume with the same parameters already exists.
				// It returns an error if the driver name is different - that is a valid error
				return err
			}
		}
	}

	return nil
}

// todo: typed/wrapped errors
func isContainerCreateNameConflict(err error) bool {
	return strings.Contains(err.Error(), "Conflict. The name")
}

func isUnknownContainer(err error) bool {
	return strings.Contains(err.Error(), "No such container:")
}
