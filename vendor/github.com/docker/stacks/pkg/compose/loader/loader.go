package loader

import (
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/go-connections/nat"
	units "github.com/docker/go-units"
	interp "github.com/docker/stacks/pkg/compose/interpolation"
	"github.com/docker/stacks/pkg/compose/schema"
	"github.com/docker/stacks/pkg/compose/template"
	"github.com/docker/stacks/pkg/compose/types"
	"github.com/docker/stacks/pkg/opts"
	shellwords "github.com/mattn/go-shellwords"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

// Options supported by Load
type Options struct {
	// Skip schema validation
	SkipValidation bool
	// Skip interpolation
	SkipInterpolation bool
	// Interpolation options
	Interpolate *interp.Options
}

// ParseYAML reads the bytes from a file, parses the bytes into a mapping
// structure, and returns it.
func ParseYAML(source []byte) (map[string]interface{}, error) {
	var cfg interface{}
	if err := yaml.Unmarshal(source, &cfg); err != nil {
		return nil, err
	}
	cfgMap, ok := cfg.(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("Top-level object must be a mapping")
	}
	converted, err := convertToStringKeysRecursive(cfgMap, "")
	if err != nil {
		return nil, err
	}
	return converted.(map[string]interface{}), nil
}

// Load reads a ConfigDetails and returns a fully loaded configuration
func Load(configDetails types.ConfigDetails, options ...func(*Options)) (*types.Config, error) {
	if len(configDetails.ConfigFiles) < 1 {
		return nil, errors.Errorf("No files specified")
	}

	opts := &Options{
		Interpolate: &interp.Options{
			Substitute:      template.Substitute,
			LookupValue:     configDetails.LookupEnv,
			TypeCastMapping: interpolateTypeCastMapping,
		},
	}

	for _, op := range options {
		op(opts)
	}

	configs := []*types.Config{}
	var err error

	for _, file := range configDetails.ConfigFiles {
		configDict := file.Config
		version := schema.Version(configDict)
		if configDetails.Version == "" {
			configDetails.Version = version
		}
		if configDetails.Version != version {
			return nil, errors.Errorf("version mismatched between two composefiles : %v and %v", configDetails.Version, version)
		}

		if err := validateForbidden(configDict); err != nil {
			return nil, err
		}

		if !opts.SkipInterpolation {
			configDict, err = interpolateConfig(configDict, *opts.Interpolate)
			if err != nil {
				return nil, err
			}
		}

		if !opts.SkipValidation {
			if err := schema.Validate(configDict, configDetails.Version); err != nil {
				return nil, err
			}
		}

		cfg, err := loadSections(configDict, configDetails)
		if err != nil {
			return nil, err
		}
		cfg.Filename = file.Filename

		configs = append(configs, cfg)
	}

	return merge(configs)
}

func validateForbidden(configDict map[string]interface{}) error {
	servicesDict, ok := configDict["services"].(map[string]interface{})
	if !ok {
		return nil
	}
	forbidden := getProperties(servicesDict, types.ForbiddenProperties)
	if len(forbidden) > 0 {
		return &ForbiddenPropertiesError{Properties: forbidden}
	}
	return nil
}

func loadSections(config map[string]interface{}, configDetails types.ConfigDetails) (*types.Config, error) {
	var err error
	cfg := types.Config{
		Version: schema.Version(config),
	}

	var loaders = []struct {
		key string
		fnc func(config map[string]interface{}) error
	}{
		{
			key: "services",
			fnc: func(config map[string]interface{}) error {
				cfg.Services, err = LoadServices(config, configDetails.WorkingDir, configDetails.LookupEnv)
				return err
			},
		},
		{
			key: "networks",
			fnc: func(config map[string]interface{}) error {
				cfg.Networks, err = LoadNetworks(config, configDetails.Version)
				return err
			},
		},
		{
			key: "volumes",
			fnc: func(config map[string]interface{}) error {
				cfg.Volumes, err = LoadVolumes(config, configDetails.Version)
				return err
			},
		},
		{
			key: "secrets",
			fnc: func(config map[string]interface{}) error {
				cfg.Secrets, err = LoadSecrets(config, configDetails)
				return err
			},
		},
		{
			key: "configs",
			fnc: func(config map[string]interface{}) error {
				cfg.Configs, err = LoadConfigObjs(config, configDetails)
				return err
			},
		},
	}
	for _, loader := range loaders {
		if err := loader.fnc(getSection(config, loader.key)); err != nil {
			return nil, err
		}
	}
	cfg.Extras = getExtras(config)
	return &cfg, nil
}

func getSection(config map[string]interface{}, key string) map[string]interface{} {
	section, ok := config[key]
	if !ok {
		return make(map[string]interface{})
	}
	return section.(map[string]interface{})
}

// GetUnsupportedProperties returns the list of any unsupported properties that are
// used in the Compose files.
func GetUnsupportedProperties(configDicts ...map[string]interface{}) []string {
	unsupported := map[string]bool{}

	for _, configDict := range configDicts {
		for _, service := range getServices(configDict) {
			serviceDict := service.(map[string]interface{})
			for _, property := range types.UnsupportedProperties {
				if _, isSet := serviceDict[property]; isSet {
					unsupported[property] = true
				}
			}
		}
	}

	return sortedKeys(unsupported)
}

func sortedKeys(set map[string]bool) []string {
	var keys []string
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// GetDeprecatedProperties returns the list of any deprecated properties that
// are used in the compose files.
func GetDeprecatedProperties(configDicts ...map[string]interface{}) map[string]string {
	deprecated := map[string]string{}

	for _, configDict := range configDicts {
		deprecatedProperties := getProperties(getServices(configDict), types.DeprecatedProperties)
		for key, value := range deprecatedProperties {
			deprecated[key] = value
		}
	}

	return deprecated
}

func getProperties(services map[string]interface{}, propertyMap map[string]string) map[string]string {
	output := map[string]string{}

	for _, service := range services {
		if serviceDict, ok := service.(map[string]interface{}); ok {
			for property, description := range propertyMap {
				if _, isSet := serviceDict[property]; isSet {
					output[property] = description
				}
			}
		}
	}

	return output
}

// ForbiddenPropertiesError is returned when there are properties in the Compose
// file that are forbidden.
type ForbiddenPropertiesError struct {
	Properties map[string]string
}

func (e *ForbiddenPropertiesError) Error() string {
	return "Configuration contains forbidden properties"
}

func getServices(configDict map[string]interface{}) map[string]interface{} {
	if services, ok := configDict["services"]; ok {
		if servicesDict, ok := services.(map[string]interface{}); ok {
			return servicesDict
		}
	}

	return map[string]interface{}{}
}

// Transform converts the source into the target struct with compose types transformer
// and the specified transformers if any.
func Transform(source interface{}, target interface{}, additionalTransformers ...Transformer) error {
	data := mapstructure.Metadata{}
	config := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			createTransformHook(additionalTransformers...),
			mapstructure.StringToTimeDurationHookFunc()),
		Result:   target,
		Metadata: &data,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	return decoder.Decode(source)
}

// Transformer defines a map to type transformer
type Transformer struct {
	TypeOf reflect.Type
	Func   func(interface{}) (interface{}, error)
}

func createTransformHook(additionalTransformers ...Transformer) mapstructure.DecodeHookFuncType {
	transforms := map[reflect.Type]func(interface{}) (interface{}, error){
		reflect.TypeOf(types.External{}):                         transformExternal,
		reflect.TypeOf(types.HealthCheckTest{}):                  transformHealthCheckTest,
		reflect.TypeOf(types.ShellCommand{}):                     transformShellCommand,
		reflect.TypeOf(types.StringList{}):                       transformStringList,
		reflect.TypeOf(map[string]string{}):                      transformMapStringString,
		reflect.TypeOf(types.UlimitsConfig{}):                    transformUlimits,
		reflect.TypeOf(types.UnitBytes(0)):                       transformSize,
		reflect.TypeOf([]types.ServicePortConfig{}):              transformServicePort,
		reflect.TypeOf(types.ServiceSecretConfig{}):              transformStringSourceMap,
		reflect.TypeOf(types.ServiceConfigObjConfig{}):           transformStringSourceMap,
		reflect.TypeOf(types.StringOrNumberList{}):               transformStringOrNumberList,
		reflect.TypeOf(map[string]*types.ServiceNetworkConfig{}): transformServiceNetworkMap,
		reflect.TypeOf(types.MappingWithEquals{}):                transformMappingOrListFunc("=", true),
		reflect.TypeOf(types.Labels{}):                           transformMappingOrListFunc("=", false),
		reflect.TypeOf(types.MappingWithColon{}):                 transformMappingOrListFunc(":", false),
		reflect.TypeOf(types.HostsList{}):                        transformListOrMappingFunc(":", false),
		reflect.TypeOf(types.ServiceVolumeConfig{}):              transformServiceVolumeConfig,
		reflect.TypeOf(types.BuildConfig{}):                      transformBuildConfig,
		reflect.TypeOf(types.Duration(0)):                        transformStringToDuration,
	}

	for _, transformer := range additionalTransformers {
		transforms[transformer.TypeOf] = transformer.Func
	}

	return func(_ reflect.Type, target reflect.Type, data interface{}) (interface{}, error) {
		transform, ok := transforms[target]
		if !ok {
			return data, nil
		}
		return transform(data)
	}
}

// keys needs to be converted to strings for jsonschema
func convertToStringKeysRecursive(value interface{}, keyPrefix string) (interface{}, error) {
	if mapping, ok := value.(map[interface{}]interface{}); ok {
		dict := make(map[string]interface{})
		for key, entry := range mapping {
			str, ok := key.(string)
			if !ok {
				return nil, formatInvalidKeyError(keyPrefix, key)
			}
			var newKeyPrefix string
			if keyPrefix == "" {
				newKeyPrefix = str
			} else {
				newKeyPrefix = fmt.Sprintf("%s.%s", keyPrefix, str)
			}
			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}
			dict[str] = convertedEntry
		}
		return dict, nil
	}
	if list, ok := value.([]interface{}); ok {
		var convertedList []interface{}
		for index, entry := range list {
			newKeyPrefix := fmt.Sprintf("%s[%d]", keyPrefix, index)
			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}
			convertedList = append(convertedList, convertedEntry)
		}
		return convertedList, nil
	}
	return value, nil
}

func formatInvalidKeyError(keyPrefix string, key interface{}) error {
	var location string
	if keyPrefix == "" {
		location = "at top level"
	} else {
		location = fmt.Sprintf("in %s", keyPrefix)
	}
	return errors.Errorf("Non-string key %s: %#v", location, key)
}

// LoadServices produces a ServiceConfig map from a compose file Dict
// the servicesDict is not validated if directly used. Use Load() to enable validation
func LoadServices(servicesDict map[string]interface{}, workingDir string, lookupEnv template.Mapping) ([]types.ServiceConfig, error) {
	var services []types.ServiceConfig

	for name, serviceDef := range servicesDict {
		serviceConfig, err := LoadService(name, serviceDef.(map[string]interface{}), workingDir, lookupEnv)
		if err != nil {
			return nil, err
		}
		services = append(services, *serviceConfig)
	}

	return services, nil
}

// LoadService produces a single ServiceConfig from a compose file Dict
// the serviceDict is not validated if directly used. Use Load() to enable validation
func LoadService(name string, serviceDict map[string]interface{}, workingDir string, lookupEnv template.Mapping) (*types.ServiceConfig, error) {
	serviceConfig := &types.ServiceConfig{}
	if err := Transform(serviceDict, serviceConfig); err != nil {
		return nil, err
	}
	serviceConfig.Name = name

	if err := resolveEnvironment(serviceConfig, workingDir, lookupEnv); err != nil {
		return nil, err
	}

	if err := resolveVolumePaths(serviceConfig.Volumes, workingDir, lookupEnv); err != nil {
		return nil, err
	}

	serviceConfig.Extras = getExtras(serviceDict)

	return serviceConfig, nil
}

func loadExtras(name string, source map[string]interface{}) map[string]interface{} {
	if dict, ok := source[name].(map[string]interface{}); ok {
		return getExtras(dict)
	}
	return nil
}

func getExtras(dict map[string]interface{}) map[string]interface{} {
	extras := map[string]interface{}{}
	for key, value := range dict {
		if strings.HasPrefix(key, "x-") {
			extras[key] = value
		}
	}
	if len(extras) == 0 {
		return nil
	}
	return extras
}

func updateEnvironment(environment map[string]*string, vars map[string]*string, lookupEnv template.Mapping) {
	for k, v := range vars {
		interpolatedV, ok := lookupEnv(k)
		if (v == nil || *v == "") && ok {
			// lookupEnv is prioritized over vars
			environment[k] = &interpolatedV
		} else {
			environment[k] = v
		}
	}
}

func resolveEnvironment(serviceConfig *types.ServiceConfig, workingDir string, lookupEnv template.Mapping) error {
	environment := make(map[string]*string)

	if len(serviceConfig.EnvFile) > 0 {
		var envVars []string

		for _, file := range serviceConfig.EnvFile {
			filePath := absPath(workingDir, file)
			fileVars, err := opts.ParseEnvFile(filePath)
			if err != nil {
				return err
			}
			envVars = append(envVars, fileVars...)
		}
		updateEnvironment(environment,
			opts.ConvertKVStringsToMapWithNil(envVars), lookupEnv)
	}

	updateEnvironment(environment, serviceConfig.Environment, lookupEnv)
	serviceConfig.Environment = environment
	return nil
}

func resolveVolumePaths(volumes []types.ServiceVolumeConfig, workingDir string, lookupEnv template.Mapping) error {
	for i, volume := range volumes {
		if volume.Type != "bind" {
			continue
		}

		if volume.Source == "" {
			return errors.New(`invalid mount config for type "bind": field Source must not be empty`)
		}

		filePath := expandUser(volume.Source, lookupEnv)
		// Check for a Unix absolute path first, to handle a Windows client
		// with a Unix daemon. This handles a Windows client connecting to a
		// Unix daemon. Note that this is not required for Docker for Windows
		// when specifying a local Windows path, because Docker for Windows
		// translates the Windows path into a valid path within the VM.
		if !path.IsAbs(filePath) {
			filePath = absPath(workingDir, filePath)
		}
		volume.Source = filePath
		volumes[i] = volume
	}
	return nil
}

// TODO: make this more robust
func expandUser(path string, lookupEnv template.Mapping) string {
	if strings.HasPrefix(path, "~") {
		home, ok := lookupEnv("HOME")
		if !ok {
			logrus.Warn("cannot expand '~', because the environment lacks HOME")
			return path
		}
		return strings.Replace(path, "~", home, 1)
	}
	return path
}

func transformUlimits(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case int:
		return types.UlimitsConfig{Single: value}, nil
	case map[string]interface{}:
		ulimit := types.UlimitsConfig{}
		ulimit.Soft = value["soft"].(int)
		ulimit.Hard = value["hard"].(int)
		return ulimit, nil
	default:
		return data, errors.Errorf("invalid type %T for ulimits", value)
	}
}

// LoadNetworks produces a NetworkConfig map from a compose file Dict
// the source Dict is not validated if directly used. Use Load() to enable validation
func LoadNetworks(source map[string]interface{}, version string) (map[string]types.NetworkConfig, error) {
	networks := make(map[string]types.NetworkConfig)
	err := Transform(source, &networks)
	if err != nil {
		return networks, err
	}
	for name, network := range networks {
		if !network.External.External {
			continue
		}
		switch {
		case network.External.Name != "":
			if network.Name != "" {
				return nil, errors.Errorf("network %s: network.external.name and network.name conflict; only use network.name", name)
			}
			if versions.GreaterThanOrEqualTo(version, "3.5") {
				logrus.Warnf("network %s: network.external.name is deprecated in favor of network.name", name)
			}
			network.Name = network.External.Name
			network.External.Name = ""
		case network.Name == "":
			network.Name = name
		}
		network.Extras = loadExtras(name, source)
		networks[name] = network
	}
	return networks, nil
}

func externalVolumeError(volume, key string) error {
	return errors.Errorf(
		"conflicting parameters \"external\" and %q specified for volume %q",
		key, volume)
}

// LoadVolumes produces a VolumeConfig map from a compose file Dict
// the source Dict is not validated if directly used. Use Load() to enable validation
func LoadVolumes(source map[string]interface{}, version string) (map[string]types.VolumeConfig, error) {
	volumes := make(map[string]types.VolumeConfig)
	if err := Transform(source, &volumes); err != nil {
		return volumes, err
	}

	for name, volume := range volumes {
		if !volume.External.External {
			continue
		}
		switch {
		case volume.Driver != "":
			return nil, externalVolumeError(name, "driver")
		case len(volume.DriverOpts) > 0:
			return nil, externalVolumeError(name, "driver_opts")
		case len(volume.Labels) > 0:
			return nil, externalVolumeError(name, "labels")
		case volume.External.Name != "":
			if volume.Name != "" {
				return nil, errors.Errorf("volume %s: volume.external.name and volume.name conflict; only use volume.name", name)
			}
			if versions.GreaterThanOrEqualTo(version, "3.4") {
				logrus.Warnf("volume %s: volume.external.name is deprecated in favor of volume.name", name)
			}
			volume.Name = volume.External.Name
			volume.External.Name = ""
		case volume.Name == "":
			volume.Name = name
		}
		volume.Extras = loadExtras(name, source)
		volumes[name] = volume
	}
	return volumes, nil
}

// LoadSecrets produces a SecretConfig map from a compose file Dict
// the source Dict is not validated if directly used. Use Load() to enable validation
func LoadSecrets(source map[string]interface{}, details types.ConfigDetails) (map[string]types.SecretConfig, error) {
	secrets := make(map[string]types.SecretConfig)
	if err := Transform(source, &secrets); err != nil {
		return secrets, err
	}
	for name, secret := range secrets {
		obj, err := loadFileObjectConfig(name, "secret", types.FileObjectConfig(secret), details)
		if err != nil {
			return nil, err
		}
		secretConfig := types.SecretConfig(obj)
		secretConfig.Extras = loadExtras(name, source)
		secrets[name] = secretConfig
	}
	return secrets, nil
}

// LoadConfigObjs produces a ConfigObjConfig map from a compose file Dict
// the source Dict is not validated if directly used. Use Load() to enable validation
func LoadConfigObjs(source map[string]interface{}, details types.ConfigDetails) (map[string]types.ConfigObjConfig, error) {
	configs := make(map[string]types.ConfigObjConfig)
	if err := Transform(source, &configs); err != nil {
		return configs, err
	}
	for name, config := range configs {
		obj, err := loadFileObjectConfig(name, "config", types.FileObjectConfig(config), details)
		if err != nil {
			return nil, err
		}
		configConfig := types.ConfigObjConfig(obj)
		configConfig.Extras = loadExtras(name, source)
		configs[name] = configConfig
	}
	return configs, nil
}

func loadFileObjectConfig(name string, objType string, obj types.FileObjectConfig, details types.ConfigDetails) (types.FileObjectConfig, error) {
	// if "external: true"
	if obj.External.External {
		// handle deprecated external.name
		if obj.External.Name != "" {
			if obj.Name != "" {
				return obj, errors.Errorf("%[1]s %[2]s: %[1]s.external.name and %[1]s.name conflict; only use %[1]s.name", objType, name)
			}
			if versions.GreaterThanOrEqualTo(details.Version, "3.5") {
				logrus.Warnf("%[1]s %[2]s: %[1]s.external.name is deprecated in favor of %[1]s.name", objType, name)
			}
			obj.Name = obj.External.Name
			obj.External.Name = ""
		} else {
			if obj.Name == "" {
				obj.Name = name
			}
		}
		// if not "external: true"
	} else {
		obj.File = absPath(details.WorkingDir, obj.File)
	}

	return obj, nil
}

func absPath(workingDir string, filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(workingDir, filePath)
}

func transformMapStringString(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		return toMapStringString(value, false), nil
	case map[string]string:
		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for map[string]string", value)
	}
}

func transformExternal(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case bool:
		return map[string]interface{}{"external": value}, nil
	case map[string]interface{}:
		return map[string]interface{}{"external": true, "name": value["name"]}, nil
	default:
		return data, errors.Errorf("invalid type %T for external", value)
	}
}

func transformServicePort(data interface{}) (interface{}, error) {
	switch entries := data.(type) {
	case []interface{}:
		// We process the list instead of individual items here.
		// The reason is that one entry might be mapped to multiple ServicePortConfig.
		// Therefore we take an input of a list and return an output of a list.
		ports := []interface{}{}
		for _, entry := range entries {
			switch value := entry.(type) {
			case int:
				v, err := ToServicePortConfigs(fmt.Sprint(value))
				if err != nil {
					return data, err
				}
				ports = append(ports, v...)
			case string:
				v, err := ToServicePortConfigs(value)
				if err != nil {
					return data, err
				}
				ports = append(ports, v...)
			case map[string]interface{}:
				ports = append(ports, value)
			default:
				return data, errors.Errorf("invalid type %T for port", value)
			}
		}
		return ports, nil
	default:
		return data, errors.Errorf("invalid type %T for port", entries)
	}
}

func transformStringSourceMap(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return map[string]interface{}{"source": value}, nil
	case map[string]interface{}:
		return data, nil
	default:
		return data, errors.Errorf("invalid type %T for secret", value)
	}
}

func transformBuildConfig(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return map[string]interface{}{"context": value}, nil
	case map[string]interface{}:
		return data, nil
	default:
		return data, errors.Errorf("invalid type %T for service build", value)
	}
}

func transformServiceVolumeConfig(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return ParseVolume(value)
	case map[string]interface{}:
		return data, nil
	default:
		return data, errors.Errorf("invalid type %T for service volume", value)
	}
}

func transformServiceNetworkMap(value interface{}) (interface{}, error) {
	if list, ok := value.([]interface{}); ok {
		mapValue := map[interface{}]interface{}{}
		for _, name := range list {
			mapValue[name] = nil
		}
		return mapValue, nil
	}
	return value, nil
}

func transformStringOrNumberList(value interface{}) (interface{}, error) {
	list := value.([]interface{})
	result := make([]string, len(list))
	for i, item := range list {
		result[i] = fmt.Sprint(item)
	}
	return result, nil
}

func transformStringList(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return []string{value}, nil
	case []interface{}:
		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for string list", value)
	}
}

func transformMappingOrListFunc(sep string, allowNil bool) func(interface{}) (interface{}, error) {
	return func(data interface{}) (interface{}, error) {
		return transformMappingOrList(data, sep, allowNil), nil
	}
}

func transformListOrMappingFunc(sep string, allowNil bool) func(interface{}) (interface{}, error) {
	return func(data interface{}) (interface{}, error) {
		return transformListOrMapping(data, sep, allowNil), nil
	}
}

func transformListOrMapping(listOrMapping interface{}, sep string, allowNil bool) interface{} {
	switch value := listOrMapping.(type) {
	case map[string]interface{}:
		return toStringList(value, sep, allowNil)
	case []interface{}:
		return listOrMapping
	}
	panic(errors.Errorf("expected a map or a list, got %T: %#v", listOrMapping, listOrMapping))
}

func transformMappingOrList(mappingOrList interface{}, sep string, allowNil bool) interface{} {
	switch value := mappingOrList.(type) {
	case map[string]interface{}:
		return toMapStringString(value, allowNil)
	case ([]interface{}):
		result := make(map[string]interface{})
		for _, value := range value {
			parts := strings.SplitN(value.(string), sep, 2)
			key := parts[0]
			switch {
			case len(parts) == 1 && allowNil:
				result[key] = nil
			case len(parts) == 1 && !allowNil:
				result[key] = ""
			default:
				result[key] = parts[1]
			}
		}
		return result
	}
	panic(errors.Errorf("expected a map or a list, got %T: %#v", mappingOrList, mappingOrList))
}

func transformShellCommand(value interface{}) (interface{}, error) {
	if str, ok := value.(string); ok {
		return shellwords.Parse(str)
	}
	return value, nil
}

func transformHealthCheckTest(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return append([]string{"CMD-SHELL"}, value), nil
	case []interface{}:
		return value, nil
	default:
		return value, errors.Errorf("invalid type %T for healthcheck.test", value)
	}
}

func transformSize(value interface{}) (interface{}, error) {
	switch value := value.(type) {
	case int:
		return int64(value), nil
	case string:
		return units.RAMInBytes(value)
	}
	panic(errors.Errorf("invalid type for size %T", value))
}

func transformStringToDuration(value interface{}) (interface{}, error) {
	switch value := value.(type) {
	case string:
		d, err := time.ParseDuration(value)
		if err != nil {
			return value, err
		}
		return types.Duration(d), nil
	default:
		return value, errors.Errorf("invalid type %T for duration", value)
	}
}

// ToServicePortConfigs converts a port definition to []ServicePortConfig
func ToServicePortConfigs(value string) ([]interface{}, error) {
	var portConfigs []interface{}

	// Check for variable before interpreting
	matches := template.DefaultPattern.FindStringSubmatch(value)
	if matches != nil {
		portConfigs = append(portConfigs, types.ServicePortConfig{
			Variable: value,
		})
		return portConfigs, nil
	}

	ports, portBindings, err := nat.ParsePortSpecs([]string{value})
	if err != nil {
		return nil, err
	}
	// We need to sort the key of the ports to make sure it is consistent
	keys := []string{}
	for port := range ports {
		keys = append(keys, string(port))
	}
	sort.Strings(keys)

	for _, key := range keys {
		// Reuse ConvertPortToPortConfig so that it is consistent
		portConfig, err := opts.ConvertPortToPortConfig(nat.Port(key), portBindings)
		if err != nil {
			return nil, err
		}
		for _, p := range portConfig {
			portConfigs = append(portConfigs, types.ServicePortConfig{
				Protocol:  string(p.Protocol),
				Target:    p.TargetPort,
				Published: p.PublishedPort,
				Mode:      string(p.PublishMode),
			})
		}
	}

	return portConfigs, nil
}

func toMapStringString(value map[string]interface{}, allowNil bool) map[string]interface{} {
	output := make(map[string]interface{})
	for key, value := range value {
		output[key] = toString(value, allowNil)
	}
	return output
}

func toString(value interface{}, allowNil bool) interface{} {
	switch {
	case value != nil:
		return fmt.Sprint(value)
	case allowNil:
		return nil
	default:
		return ""
	}
}

func toStringList(value map[string]interface{}, separator string, allowNil bool) []string {
	output := []string{}
	for key, value := range value {
		if value == nil && !allowNil {
			continue
		}
		output = append(output, fmt.Sprintf("%s%s%s", key, separator, value))
	}
	sort.Strings(output)
	return output
}
