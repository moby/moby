package container

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/controllers/plugin"
	executorpkg "github.com/docker/docker/daemon/cluster/executor"
	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	networktypes "github.com/docker/libnetwork/types"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/agent/secrets"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/naming"
	"golang.org/x/net/context"
)

type executor struct {
	backend executorpkg.Backend
	secrets exec.SecretsManager
}

// NewExecutor returns an executor from the docker client.
func NewExecutor(b executorpkg.Backend) exec.Executor {
	return &executor{
		backend: b,
		secrets: secrets.NewManager(),
	}
}

// Describe returns the underlying node description from the docker client.
func (e *executor) Describe(ctx context.Context) (*api.NodeDescription, error) {
	info, err := e.backend.SystemInfo()
	if err != nil {
		return nil, err
	}

	plugins := map[api.PluginDescription]struct{}{}
	addPlugins := func(typ string, names []string) {
		for _, name := range names {
			plugins[api.PluginDescription{
				Type: typ,
				Name: name,
			}] = struct{}{}
		}
	}

	// add v1 plugins
	addPlugins("Volume", info.Plugins.Volume)
	// Add builtin driver "overlay" (the only builtin multi-host driver) to
	// the plugin list by default.
	addPlugins("Network", append([]string{"overlay"}, info.Plugins.Network...))
	addPlugins("Authorization", info.Plugins.Authorization)

	// add v2 plugins
	v2Plugins, err := e.backend.PluginManager().List(filters.NewArgs())
	if err == nil {
		for _, plgn := range v2Plugins {
			for _, typ := range plgn.Config.Interface.Types {
				if typ.Prefix != "docker" || !plgn.Enabled {
					continue
				}
				plgnTyp := typ.Capability
				if typ.Capability == "volumedriver" {
					plgnTyp = "Volume"
				} else if typ.Capability == "networkdriver" {
					plgnTyp = "Network"
				}
				plugins[api.PluginDescription{
					Type: plgnTyp,
					Name: plgn.Name,
				}] = struct{}{}
			}
		}
	}

	pluginFields := make([]api.PluginDescription, 0, len(plugins))
	for k := range plugins {
		pluginFields = append(pluginFields, k)
	}

	sort.Sort(sortedPlugins(pluginFields))

	// parse []string labels into a map[string]string
	labels := map[string]string{}
	for _, l := range info.Labels {
		stringSlice := strings.SplitN(l, "=", 2)
		// this will take the last value in the list for a given key
		// ideally, one shouldn't assign multiple values to the same key
		if len(stringSlice) > 1 {
			labels[stringSlice[0]] = stringSlice[1]
		}
	}

	description := &api.NodeDescription{
		Hostname: info.Name,
		Platform: &api.Platform{
			Architecture: info.Architecture,
			OS:           info.OSType,
		},
		Engine: &api.EngineDescription{
			EngineVersion: info.ServerVersion,
			Labels:        labels,
			Plugins:       pluginFields,
		},
		Resources: &api.Resources{
			NanoCPUs:    int64(info.NCPU) * 1e9,
			MemoryBytes: info.MemTotal,
		},
	}

	return description, nil
}

func (e *executor) Configure(ctx context.Context, node *api.Node) error {
	na := node.Attachment
	if na == nil {
		e.backend.ReleaseIngress()
		return nil
	}

	options := types.NetworkCreate{
		Driver: na.Network.DriverState.Name,
		IPAM: &network.IPAM{
			Driver: na.Network.IPAM.Driver.Name,
		},
		Options:        na.Network.DriverState.Options,
		Ingress:        true,
		CheckDuplicate: true,
	}

	for _, ic := range na.Network.IPAM.Configs {
		c := network.IPAMConfig{
			Subnet:  ic.Subnet,
			IPRange: ic.Range,
			Gateway: ic.Gateway,
		}
		options.IPAM.Config = append(options.IPAM.Config, c)
	}

	_, err := e.backend.SetupIngress(clustertypes.NetworkCreateRequest{
		ID: na.Network.ID,
		NetworkCreateRequest: types.NetworkCreateRequest{
			Name:          na.Network.Spec.Annotations.Name,
			NetworkCreate: options,
		},
	}, na.Addresses[0])

	return err
}

// Controller returns a docker container runner.
func (e *executor) Controller(t *api.Task) (exec.Controller, error) {
	if t.Spec.GetAttachment() != nil {
		return newNetworkAttacherController(e.backend, t, e.secrets)
	}

	var ctlr exec.Controller
	switch r := t.Spec.GetRuntime().(type) {
	case *api.TaskSpec_Generic:
		logrus.WithFields(logrus.Fields{
			"kind":     r.Generic.Kind,
			"type_url": r.Generic.Payload.TypeUrl,
		}).Debug("custom runtime requested")
		runtimeKind, err := naming.Runtime(t.Spec)
		if err != nil {
			return ctlr, err
		}
		switch runtimeKind {
		case string(swarmtypes.RuntimePlugin):
			c, err := plugin.NewController()
			if err != nil {
				return ctlr, err
			}
			ctlr = c
		default:
			return ctlr, fmt.Errorf("unsupported runtime type: %q", r.Generic.Kind)
		}
	case *api.TaskSpec_Container:
		c, err := newController(e.backend, t, secrets.Restrict(e.secrets, t))
		if err != nil {
			return ctlr, err
		}
		ctlr = c
	default:
		return ctlr, fmt.Errorf("unsupported runtime: %q", r)
	}

	return ctlr, nil
}

func (e *executor) SetNetworkBootstrapKeys(keys []*api.EncryptionKey) error {
	nwKeys := []*networktypes.EncryptionKey{}
	for _, key := range keys {
		nwKey := &networktypes.EncryptionKey{
			Subsystem:   key.Subsystem,
			Algorithm:   int32(key.Algorithm),
			Key:         make([]byte, len(key.Key)),
			LamportTime: key.LamportTime,
		}
		copy(nwKey.Key, key.Key)
		nwKeys = append(nwKeys, nwKey)
	}
	e.backend.SetNetworkBootstrapKeys(nwKeys)

	return nil
}

func (e *executor) Secrets() exec.SecretsManager {
	return e.secrets
}

type sortedPlugins []api.PluginDescription

func (sp sortedPlugins) Len() int { return len(sp) }

func (sp sortedPlugins) Swap(i, j int) { sp[i], sp[j] = sp[j], sp[i] }

func (sp sortedPlugins) Less(i, j int) bool {
	if sp[i].Type != sp[j].Type {
		return sp[i].Type < sp[j].Type
	}
	return sp[i].Name < sp[j].Name
}
