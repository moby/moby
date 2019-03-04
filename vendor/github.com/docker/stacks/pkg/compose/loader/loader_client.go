package loader

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/docker/stacks/pkg/compose/defaults"
	"github.com/docker/stacks/pkg/compose/interpolation"
	"github.com/docker/stacks/pkg/compose/schema"
	composetypes "github.com/docker/stacks/pkg/compose/types"
	"github.com/docker/stacks/pkg/types"

	"github.com/pkg/errors"
)

// TODO - this file needs some refactoring

// LoadComposefile will load the compose files into ComposeInput which can be sent to the server
// for parsing into a Stack representation
func LoadComposefile(composefiles []string) (*types.ComposeInput, error) {
	input := types.ComposeInput{}
	for _, filename := range composefiles {
		bytes, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		input.ComposeFiles = append(input.ComposeFiles, string(bytes))
	}
	return &input, nil
}

// TODO Remainder of this is server side logic that should move someplace else...

// ParseComposeInput will convert the ComposeInput into the StackCreate type
// If the ComposeInput contains any variables, those will be
// listed in the StackSpec.PropertyValues field, so they can be filled
// in prior to sending the StackCreate to the Create API.  If defaults
// are defined in the compose file(s) those defaults will be included.
func ParseComposeInput(input types.ComposeInput) (*types.StackCreate, error) {
	if len(input.ComposeFiles) == 0 {
		return nil, nil
	}

	configDetails, err := getConfigDetails(input)
	if err != nil {
		return nil, err
	}

	dicts := getDictsFrom(configDetails.ConfigFiles)

	// Wire up interpolation as a no-op so we can track the variables in play and default values
	propertiesMap := map[string]string{}
	interpolateOpts := interpolation.Options{
		LookupValue: func(key string) (string, bool) {
			vals := strings.SplitN(key, "=", 2)
			if len(vals) > 1 {
				propertiesMap[vals[0]] = vals[1]
			} else if _, exists := propertiesMap[vals[0]]; !exists {
				propertiesMap[vals[0]] = ""
			}
			return "", false
		},
		Substitute: defaults.RecordVariablesWithDefaults,
	}
	config, err := Load(configDetails, func(opts *Options) {
		opts.Interpolate = &interpolateOpts
		opts.SkipValidation = true
	})
	if err != nil {
		if fpe, ok := err.(*ForbiddenPropertiesError); ok {
			return nil, errors.Errorf("Compose file contains unsupported options:\n\n%s\n",
				propertyWarnings(fpe.Properties))
		}

		return nil, err
	}

	unsupportedProperties := GetUnsupportedProperties(dicts...)
	if len(unsupportedProperties) > 0 {
		fmt.Printf("Ignoring unsupported options: %s\n\n",
			strings.Join(unsupportedProperties, ", "))
	}

	deprecatedProperties := GetDeprecatedProperties(dicts...)
	if len(deprecatedProperties) > 0 {
		fmt.Printf("Ignoring deprecated options:\n\n%s\n\n",
			propertyWarnings(deprecatedProperties))
	}
	properties := []string{}
	for key, value := range propertiesMap {
		if len(value) > 0 {
			properties = append(properties, fmt.Sprintf("%s=%s", key, value))
		} else {
			properties = append(properties, key)
		}
	}
	return &types.StackCreate{
		Spec: types.StackSpec{
			Services:       config.Services,
			Secrets:        config.Secrets,
			Configs:        config.Configs,
			Networks:       config.Networks,
			Volumes:        config.Volumes,
			PropertyValues: properties,
		},
	}, nil

}

func getDictsFrom(configFiles []composetypes.ConfigFile) []map[string]interface{} {
	dicts := []map[string]interface{}{}

	for _, configFile := range configFiles {
		dicts = append(dicts, configFile.Config)
	}

	return dicts
}

func propertyWarnings(properties map[string]string) string {
	var msgs []string
	for name, description := range properties {
		msgs = append(msgs, fmt.Sprintf("%s: %s", name, description))
	}
	sort.Strings(msgs)
	return strings.Join(msgs, "\n\n")
}

func getConfigDetails(input types.ComposeInput) (composetypes.ConfigDetails, error) {
	var details composetypes.ConfigDetails

	var err error
	details.ConfigFiles, err = loadConfigFiles(input)
	if err != nil {
		return details, err
	}
	// Take the first file version (2 files can't have different version)
	details.Version = schema.Version(details.ConfigFiles[0].Config)
	return details, err
}

func loadConfigFiles(input types.ComposeInput) ([]composetypes.ConfigFile, error) {
	var configFiles []composetypes.ConfigFile

	for _, data := range input.ComposeFiles {
		configFile, err := loadConfigFile(data)
		if err != nil {
			return configFiles, err
		}
		configFiles = append(configFiles, *configFile)
	}

	return configFiles, nil
}

func loadConfigFile(data string) (*composetypes.ConfigFile, error) {
	var err error

	config, err := ParseYAML([]byte(data))
	if err != nil {
		return nil, err
	}

	return &composetypes.ConfigFile{
		Config: config,
	}, nil
}
