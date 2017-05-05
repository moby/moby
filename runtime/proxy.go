package runtime

import (
	"github.com/docker/docker/api/types/runtime"
	getter "github.com/docker/docker/pkg/plugingetter"
)

type RuntimeDriver struct {
	info   runtime.Info
	plugin getter.CompatPlugin
}

func runtimeDriverFromPlugin(plugin getter.CompatPlugin) (*RuntimeDriver, error) {
	r := &RuntimeDriver{
		info: runtime.Info{
			DefaultRuntime: false,
		},
		plugin: plugin,
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return r, nil
}

func newRuntimeDriver(name string, rt runtime.Runtime, plugin getter.CompatPlugin) *RuntimeDriver {
	return &RuntimeDriver{
		info: runtime.Info{
			Name:           name,
			Runtime:        rt,
			DefaultRuntime: false,
		},
		plugin: plugin,
	}
}

func (r RuntimeDriver) name() string {
	return r.info.Name
}

func (r *RuntimeDriver) init() error {
	if r.plugin == nil {
		return ErrRuntimeMissingPlugin
	}

	r.info.Name = r.plugin.Name()

	// Fetch the runtime path and arguments
	path, err := r.Path()
	if err != nil {
		return err
	}

	args, err := r.Args()
	if err != nil {
		return err
	}

	r.info.Runtime = runtime.Runtime{
		Path: path,
		Args: args,
	}

	return nil
}

func (r *RuntimeDriver) setDefaultRuntime(d bool) {
	r.info.DefaultRuntime = d
}

func (r RuntimeDriver) Info() runtime.Info {
	return r.info
}

func (r RuntimeDriver) Path() (string, error) {
	if r.plugin == nil {
		return "", ErrRuntimeMissingPlugin
	}

	var resp runtime.PluginPathResponse

	if err := r.plugin.Client().Call(runtime.GetPath, nil, &resp); err != nil {
		return "", err
	}

	return resp.Path, nil
}

func (r RuntimeDriver) Args() ([]string, error) {
	var resp runtime.PluginArgsResponse

	if err := r.plugin.Client().Call(runtime.GetArgs, nil, &resp); err != nil {
		return nil, err
	}

	return resp.Args, nil
}
