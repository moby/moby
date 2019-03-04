package substitution

// Utility routines to perform property value substitution
// on StackSpecs

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/stacks/pkg/compose/loader"
	"github.com/docker/stacks/pkg/compose/template"
	composetypes "github.com/docker/stacks/pkg/compose/types"
	"github.com/docker/stacks/pkg/types"
)

// DoSubstitution will make a copy of the input StackSpec
// and perform variable substitution on all components based
// on the PropertyValues within the spec.  The original Spec
// is not modified.
func DoSubstitution(spec types.StackSpec) (types.StackSpec, error) {
	// Start with a naive implementation based on round-tripping to json
	var finalSpec types.StackSpec

	// TODO There may be additional corner cases where
	// the structure changes (not a simple string replacement)
	// Those are handled by custom conversion routines here
	// before performing the generic json round-trip conversion
	for _, sub := range []func(spec *types.StackSpec) error{
		doPortSubstitutions,
		doVolumeSubstitutions,
	} {
		err := sub(&spec)
		if err != nil {
			return finalSpec, err
		}
	}

	specPreJSON, err := json.MarshalIndent(spec, "", "    ")
	if err != nil {
		return finalSpec, err
	}

	specPostJSON, err := doSubstitute(string(specPreJSON), &spec)
	if err != nil {
		return finalSpec, err
	}

	err = json.Unmarshal([]byte(specPostJSON), &finalSpec)
	return finalSpec, err
}

// Wrap the template.Substitute function to automatically lookup
// from spec.Properties
func doSubstitute(data string, spec *types.StackSpec) (string, error) {
	envMapping := map[string]string{}
	for _, keyval := range spec.PropertyValues {
		split := strings.SplitN(keyval, "=", 2)
		if len(split) != 2 {
			return "", fmt.Errorf("Malformed property value: %s", keyval)
		}
		envMapping[split[0]] = split[1]
	}
	return template.Substitute(data,
		func(key string) (string, bool) {
			val, found := envMapping[key]
			return val, found
		})
}

func doPortSubstitutions(spec *types.StackSpec) error {
	for si, service := range spec.Services {
		for pi, port := range service.Ports {
			if port.Variable != "" {
				realPortString, err := doSubstitute(port.Variable, spec)
				if err != nil {
					return err
				}

				realPort, err := loader.ToServicePortConfigs(realPortString)
				if err != nil {
					return err
				}
				if len(realPort) == 0 {
					// Is there a valid use-case for zeroing out the port in a variable?
					return fmt.Errorf("malformed port substitution: %s", realPortString)
				}

				// Swap out the original port
				spec.Services[si].Ports[pi] = realPort[0].(composetypes.ServicePortConfig)
				// Append secondary ports to the list
				for i := 1; i < len(realPort); i++ {
					spec.Services[si].Ports = append(spec.Services[si].Ports, realPort[i].(composetypes.ServicePortConfig))
				}
			}
		}
	}
	return nil
}

func doVolumeSubstitutions(spec *types.StackSpec) error {
	for si, service := range spec.Services {
		for vi, volume := range service.Volumes {
			// Check if the Target looks like a variable
			matches := template.DefaultPattern.FindStringSubmatch(volume.Target)
			if matches == nil {
				continue
			}

			realVolumeString, err := doSubstitute(volume.Target, spec)
			if err != nil {
				return err
			}

			realVolume, err := loader.ParseVolume(realVolumeString)
			if err != nil {
				return err
			}

			// Swap out the original volume
			spec.Services[si].Volumes[vi] = realVolume
		}
	}
	return nil
}
