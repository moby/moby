/*
  Based on https://github.com/containerd/nri/blob/main/plugins/template/ - which is ...

   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package nri

import (
	"context"
	"errors"
	"testing"

	"github.com/containerd/log"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"gotest.tools/v3/assert"
)

type builtinPluginConfig struct {
	pluginName   string
	pluginIdx    string
	sockPath     string
	ctrCreateAdj *api.ContainerAdjustment
}

type builtinPlugin struct {
	stub   stub.Stub
	logG   func(context.Context) *log.Entry
	config builtinPluginConfig
}

// startBuiltinPlugin turns the test binary into an NRI plugin, or fails the test.
// It is the caller's responsibility to call the returned function to stop the plugin.
func startBuiltinPlugin(ctx context.Context, t *testing.T, cfg builtinPluginConfig) func() {
	p := &builtinPlugin{
		logG: func(ctx context.Context) *log.Entry {
			return log.G(ctx).WithField("nri-plugin", cfg.pluginIdx+"-"+cfg.pluginName)
		},
		config: cfg,
	}
	stub, err := stub.New(p,
		stub.WithOnClose(p.onClose),
		stub.WithPluginName(cfg.pluginName),
		stub.WithPluginIdx(cfg.pluginIdx),
		stub.WithSocketPath(cfg.sockPath),
	)
	assert.Assert(t, err)
	p.stub = stub
	err = p.stub.Start(ctx)
	assert.Assert(t, err)
	return p.stub.Stop
}

func (p *builtinPlugin) Configure(ctx context.Context, config, runtime, version string) (stub.EventMask, error) {
	p.logG(ctx).Infof("Connected to %s/%s...", runtime, version)

	if config != "" {
		return 0, errors.New("plugin config from yaml is not implemented")
	}
	return 0, nil
}

func (p *builtinPlugin) Synchronize(ctx context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	p.logG(ctx).Infof("Synchronized state with the runtime (%d pods, %d containers)...",
		len(pods), len(containers))
	return nil, nil
}

func (p *builtinPlugin) Shutdown(ctx context.Context) {
	p.logG(ctx).Info("Runtime shutting down...")
}

func (p *builtinPlugin) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	p.logG(ctx).Infof("Started pod %s/%s...", pod.GetNamespace(), pod.GetName())
	return nil
}

func (p *builtinPlugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	p.logG(ctx).Infof("Stopped pod %s/%s...", pod.GetNamespace(), pod.GetName())
	return nil
}

func (p *builtinPlugin) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	p.logG(ctx).Infof("Removed pod %s/%s...", pod.GetNamespace(), pod.GetName())
	return nil
}

func (p *builtinPlugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	p.logG(ctx).Infof("Creating container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())

	//
	// This is the container creation request handler. Because the container
	// has not been created yet, this is the lifecycle event which allows you
	// the largest set of changes to the container's configuration, including
	// some of the later immutable parameters. Take a look at the adjustment
	// functions in pkg/api/adjustment.go to see the available controls.
	//
	// In addition to reconfiguring the container being created, you are also
	// allowed to update other existing containers. Take a look at the update
	// functions in pkg/api/update.go to see the available controls.
	//

	return p.config.ctrCreateAdj, []*api.ContainerUpdate{}, nil
}

func (p *builtinPlugin) PostCreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	p.logG(ctx).Infof("Created container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())
	return nil
}

func (p *builtinPlugin) StartContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	p.logG(ctx).Infof("Starting container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())
	return nil
}

func (p *builtinPlugin) PostStartContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	p.logG(ctx).Infof("Started container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())
	return nil
}

func (p *builtinPlugin) UpdateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container, r *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	p.logG(ctx).Infof("Updating container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())

	//
	// This is the container update request handler. You can make changes to
	// the container update before it is applied. Take a look at the functions
	// in pkg/api/update.go to see the available controls.
	//
	// In addition to altering the pending update itself, you are also allowed
	// to update other existing containers.
	//

	updates := []*api.ContainerUpdate{}

	return updates, nil
}

func (p *builtinPlugin) PostUpdateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	p.logG(ctx).Infof("Updated container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())
	return nil
}

func (p *builtinPlugin) StopContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	p.logG(ctx).Infof("Stopped container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())

	//
	// This is the container (post-)stop request handler. You can update any
	// of the remaining running containers. Take a look at the functions in
	// pkg/api/update.go to see the available controls.
	//

	return []*api.ContainerUpdate{}, nil
}

func (p *builtinPlugin) RemoveContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	p.logG(ctx).Infof("Removed container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())
	return nil
}

func (p *builtinPlugin) onClose() {
	p.logG(context.Background()).Infof("Connection to the runtime lost.")
}
