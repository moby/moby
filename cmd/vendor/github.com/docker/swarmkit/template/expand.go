package template

import (
	"fmt"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/pkg/errors"
)

// ExpandContainerSpec expands templated fields in the runtime using the task
// state. Templating is all evaluated on the agent-side, before execution.
//
// Note that these are projected only on runtime values, since active task
// values are typically manipulated in the manager.
func ExpandContainerSpec(t *api.Task) (*api.ContainerSpec, error) {
	container := t.Spec.GetContainer()
	if container == nil {
		return nil, errors.Errorf("task missing ContainerSpec to expand")
	}

	container = container.Copy()
	ctx := NewContextFromTask(t)

	var err error
	container.Env, err = expandEnv(ctx, container.Env)
	if err != nil {
		return container, errors.Wrap(err, "expanding env failed")
	}

	// For now, we only allow templating of string-based mount fields
	container.Mounts, err = expandMounts(ctx, container.Mounts)
	if err != nil {
		return container, errors.Wrap(err, "expanding mounts failed")
	}

	container.Hostname, err = ctx.Expand(container.Hostname)
	return container, errors.Wrap(err, "expanding hostname failed")
}

func expandMounts(ctx Context, mounts []api.Mount) ([]api.Mount, error) {
	if len(mounts) == 0 {
		return mounts, nil
	}

	expanded := make([]api.Mount, len(mounts))
	for i, mount := range mounts {
		var err error
		mount.Source, err = ctx.Expand(mount.Source)
		if err != nil {
			return mounts, errors.Wrapf(err, "expanding mount source %q", mount.Source)
		}

		mount.Target, err = ctx.Expand(mount.Target)
		if err != nil {
			return mounts, errors.Wrapf(err, "expanding mount target %q", mount.Target)
		}

		if mount.VolumeOptions != nil {
			mount.VolumeOptions.Labels, err = expandMap(ctx, mount.VolumeOptions.Labels)
			if err != nil {
				return mounts, errors.Wrap(err, "expanding volume labels")
			}

			if mount.VolumeOptions.DriverConfig != nil {
				mount.VolumeOptions.DriverConfig.Options, err = expandMap(ctx, mount.VolumeOptions.DriverConfig.Options)
				if err != nil {
					return mounts, errors.Wrap(err, "expanding volume driver config")
				}
			}
		}

		expanded[i] = mount
	}

	return expanded, nil
}

func expandMap(ctx Context, m map[string]string) (map[string]string, error) {
	var (
		n   = make(map[string]string, len(m))
		err error
	)

	for k, v := range m {
		v, err = ctx.Expand(v)
		if err != nil {
			return m, errors.Wrapf(err, "expanding map entry %q=%q", k, v)
		}

		n[k] = v
	}

	return n, nil
}

func expandEnv(ctx Context, values []string) ([]string, error) {
	var result []string
	for _, value := range values {
		var (
			parts = strings.SplitN(value, "=", 2)
			entry = parts[0]
		)

		if len(parts) > 1 {
			expanded, err := ctx.Expand(parts[1])
			if err != nil {
				return values, errors.Wrapf(err, "expanding env %q", value)
			}

			entry = fmt.Sprintf("%s=%s", entry, expanded)
		}

		result = append(result, entry)
	}

	return result, nil
}
