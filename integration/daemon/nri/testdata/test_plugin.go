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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

type config struct {
	EnvVar string `json:"env-var"`
	EnvVal string `json:"env-val"`
}

type plugin struct {
	stub stub.Stub
	cfg  config
}

func (p *plugin) Configure(_ context.Context, config, runtime, version string) (stub.EventMask, error) {
	if config == "" {
		return 0, nil
	}

	err := json.Unmarshal([]byte(config), &p.cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return api.MustParseEventMask("CreateContainer"), nil
}

func (p *plugin) CreateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	env := []*api.KeyValue{{Key: "NRI_TEST_PLUGIN", Value: "wozere"}}
	if p.cfg.EnvVar != "" {
		env = append(env, &api.KeyValue{Key: p.cfg.EnvVar, Value: p.cfg.EnvVal})
	}

	adjustment := &api.ContainerAdjustment{Env: env}
	updates := []*api.ContainerUpdate{}

	return adjustment, updates, nil
}

func main() {
	var (
		pluginName string
		pluginIdx  string
		err        error
	)

	flag.StringVar(&pluginName, "name", "", "plugin name to register to NRI")
	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.Parse()

	p := &plugin{}
	opts := []stub.Option{
		stub.WithOnClose(func() { os.Exit(0) }),
	}
	if pluginName != "" {
		opts = append(opts, stub.WithPluginName(pluginName))
	}
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	if p.stub, err = stub.New(p, opts...); err != nil {
		os.Exit(1)
	}
	if err = p.stub.Run(context.Background()); err != nil {
		os.Exit(1)
	}
}
