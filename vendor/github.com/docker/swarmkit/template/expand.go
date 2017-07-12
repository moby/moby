package template

import (
	"fmt"
	"strings"

	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/pkg/errors"
)

// ExpandContainerSpec expands templated fields in the runtime using the task
// state and the node where it is scheduled to run.
// Templating is all evaluated on the agent-side, before execution.
//
// Note that these are projected only on runtime values, since active task
// values are typically manipulated in the manager.
func ExpandContainerSpec(n *api.NodeDescription, t *api.Task) (*api.ContainerSpec, error) {
	container := t.Spec.GetContainer()
	if container == nil {
		return nil, errors.Errorf("task missing ContainerSpec to expand")
	}

	container = container.Copy()
	ctx := NewContext(n, t)

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

func expandPayload(ctx PayloadContext, payload []byte) ([]byte, error) {
	result, err := ctx.Expand(string(payload))
	if err != nil {
		return payload, err
	}
	return []byte(result), nil
}

// ExpandSecretSpec expands the template inside the secret payload, if any.
// Templating is evaluated on the agent-side.
func ExpandSecretSpec(s *api.Secret, node *api.NodeDescription, t *api.Task, dependencies exec.DependencyGetter) (*api.SecretSpec, error) {
	if s.Spec.Templating == nil {
		return &s.Spec, nil
	}
	if s.Spec.Templating.Name == "golang" {
		ctx := NewPayloadContextFromTask(node, t, dependencies)
		secretSpec := s.Spec.Copy()

		var err error
		secretSpec.Data, err = expandPayload(ctx, secretSpec.Data)
		return secretSpec, err
	}
	return &s.Spec, errors.New("unrecognized template type")
}

// ExpandConfigSpec expands the template inside the config payload, if any.
// Templating is evaluated on the agent-side.
func ExpandConfigSpec(c *api.Config, node *api.NodeDescription, t *api.Task, dependencies exec.DependencyGetter) (*api.ConfigSpec, error) {
	if c.Spec.Templating == nil {
		return &c.Spec, nil
	}
	if c.Spec.Templating.Name == "golang" {
		ctx := NewPayloadContextFromTask(node, t, dependencies)
		configSpec := c.Spec.Copy()

		var err error
		configSpec.Data, err = expandPayload(ctx, configSpec.Data)
		return configSpec, err
	}
	return &c.Spec, errors.New("unrecognized template type")
}
