package system

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/inspect"
	apiclient "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type inspectOptions struct {
	format      string
	inspectType string
	size        bool
	ids         []string
}

type inspectionResult struct {
	objectType string
	v          interface{}
	raw        []byte
	err        error
}

// NewInspectCommand creates a new cobra.Command for `docker inspect`
func NewInspectCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] NAME|ID [NAME|ID...]",
		Short: "Return low-level information on Docker objects",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ids = args
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given Go template")
	flags.StringVar(&opts.inspectType, "type", "", "Return JSON for specified type")
	flags.BoolVarP(&opts.size, "size", "s", false, "Display total file sizes if the type is container")

	return cmd
}

func runInspect(dockerCli *command.DockerCli, opts inspectOptions) error {
	expectedType := ""
	var refObjects = make(map[string]inspectionResult)
	switch opts.inspectType {
	case "", "container", "image", "node", "network", "service", "volume", "task", "plugin", "secret":
		for _, ref := range opts.ids {
			result := inspectAll(context.Background(), dockerCli, ref, opts.size, opts.inspectType)

			if result.err == nil && expectedType != "" && result.objectType != expectedType {
				result.err = errors.Errorf("Error: all objects to inspect were expected to be of type %s (same as %s), but %s is of type %s", expectedType, opts.ids[0], ref, result.objectType)
			}

			refObjects[ref] = result
			expectedType = result.objectType
		}
	default:
		return errors.Errorf("%q is not a valid value for --type", opts.inspectType)
	}
	getInspectionResult := func(ref string) (interface{}, []byte, error) {
		obj := refObjects[ref]
		return obj.v, obj.raw, obj.err
	}
	return inspect.Inspect(dockerCli.Out(), opts.ids, opts.format, getInspectionResult)
}

func inspectContainers(ctx context.Context, dockerCli *command.DockerCli, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ContainerInspectWithRaw(ctx, ref, getSize)
	}
}

func inspectImages(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ImageInspectWithRaw(ctx, ref)
	}
}

func inspectNetwork(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().NetworkInspectWithRaw(ctx, ref, false)
	}
}

func inspectNode(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().NodeInspectWithRaw(ctx, ref)
	}
}

func inspectService(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		// Service inspect shows defaults values in empty fields.
		return dockerCli.Client().ServiceInspectWithRaw(ctx, ref, types.ServiceInspectOptions{InsertDefaults: true})
	}
}

func inspectTasks(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().TaskInspectWithRaw(ctx, ref)
	}
}

func inspectVolume(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().VolumeInspectWithRaw(ctx, ref)
	}
}

func inspectPlugin(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().PluginInspectWithRaw(ctx, ref)
	}
}

func inspectSecret(ctx context.Context, dockerCli *command.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().SecretInspectWithRaw(ctx, ref)
	}
}

func inspectAll(ctx context.Context, dockerCli *command.DockerCli, ref string, getSize bool, typeConstraint string) inspectionResult {
	var inspectAutodetect = []struct {
		objectType      string
		isSizeSupported bool
		isSwarmObject   bool
		objectInspector func(string) (interface{}, []byte, error)
	}{
		{
			objectType:      "container",
			isSizeSupported: true,
			objectInspector: inspectContainers(ctx, dockerCli, getSize),
		},
		{
			objectType:      "image",
			objectInspector: inspectImages(ctx, dockerCli),
		},
		{
			objectType:      "network",
			objectInspector: inspectNetwork(ctx, dockerCli),
		},
		{
			objectType:      "volume",
			objectInspector: inspectVolume(ctx, dockerCli),
		},
		{
			objectType:      "service",
			isSwarmObject:   true,
			objectInspector: inspectService(ctx, dockerCli),
		},
		{
			objectType:      "task",
			isSwarmObject:   true,
			objectInspector: inspectTasks(ctx, dockerCli),
		},
		{
			objectType:      "node",
			isSwarmObject:   true,
			objectInspector: inspectNode(ctx, dockerCli),
		},
		{
			objectType:      "plugin",
			objectInspector: inspectPlugin(ctx, dockerCli),
		},
		{
			objectType:      "secret",
			isSwarmObject:   true,
			objectInspector: inspectSecret(ctx, dockerCli),
		},
	}

	// isSwarmManager does an Info API call to verify that the daemon is
	// a swarm manager.
	isSwarmManager := func() bool {
		info, err := dockerCli.Client().Info(ctx)
		if err != nil {
			fmt.Fprintln(dockerCli.Err(), err)
			return false
		}
		return info.Swarm.ControlAvailable
	}

	isErrNotSupported := func(err error) bool {
		return strings.Contains(err.Error(), "not supported")
	}

	const (
		swarmSupportUnknown = iota
		swarmSupported
		swarmUnsupported
	)

	isSwarmSupported := swarmSupportUnknown

	result := inspectionResult{}
	alreadyFound := false
	for _, inspectData := range inspectAutodetect {
		if typeConstraint != "" && inspectData.objectType != typeConstraint {
			continue
		}
		if typeConstraint == "" && inspectData.isSwarmObject {
			if isSwarmSupported == swarmSupportUnknown {
				if isSwarmManager() {
					isSwarmSupported = swarmSupported
				} else {
					isSwarmSupported = swarmUnsupported
				}
			}
			if isSwarmSupported == swarmUnsupported {
				continue
			}
		}
		v, raw, err := inspectData.objectInspector(ref)
		if err != nil {
			if typeConstraint == "" && (apiclient.IsErrNotFound(err) || isErrNotSupported(err)) {
				continue
			}
			return inspectionResult{"", v, raw, err}
		}

		if getSize && !inspectData.isSizeSupported {
			fmt.Fprintf(dockerCli.Err(), "WARNING: --size ignored for %s\n", inspectData.objectType)
		}

		if alreadyFound {
			err := errors.Errorf("Error: multiple types returned for object %q, use --type option to inspect only the type that is needed.", ref)
			return inspectionResult{"", nil, nil, err}
		}
		alreadyFound = true
		result = inspectionResult{inspectData.objectType, v, raw, err}
	}

	if !alreadyFound {
		return inspectionResult{"", nil, nil, errors.Errorf("Error: No such object: %s", ref)}
	}
	return result
}
