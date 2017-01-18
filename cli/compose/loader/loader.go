package loader

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/docker/docker/cli/compose/interpolation"
	"github.com/docker/docker/cli/compose/schema"
	"github.com/docker/docker/cli/compose/types"
	"github.com/docker/docker/runconfig/opts"
	units "github.com/docker/go-units"
	shellwords "github.com/mattn/go-shellwords"
	"github.com/mitchellh/mapstructure"
	yaml "gopkg.in/yaml.v2"
)

var (
	fieldNameRegexp = regexp.MustCompile("[A-Z][a-z0-9]+")
)

// ParseYAML reads the bytes from a file, parses the bytes into a mapping
// structure, and returns it.
func ParseYAML(source []byte) (types.Dict, error) {
	var cfg interface{}
	if err := yaml.Unmarshal(source, &cfg); err != nil {
		return nil, err
	}
	cfgMap, ok := cfg.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("Top-level object must be a mapping")
	}
	converted, err := convertToStringKeysRecursive(cfgMap, "")
	if err != nil {
		return nil, err
	}
	return converted.(types.Dict), nil
}

// Load reads a ConfigDetails and returns a fully loaded configuration
func Load(configDetails types.ConfigDetails) (*types.Config, error) {
	if len(configDetails.ConfigFiles) < 1 {
		return nil, fmt.Errorf("No files specified")
	}
	if len(configDetails.ConfigFiles) > 1 {
		return nil, fmt.Errorf("Multiple files are not yet supported")
	}

	configDict := getConfigDict(configDetails)

	if services, ok := configDict["services"]; ok {
		if servicesDict, ok := services.(types.Dict); ok {
			forbidden := getProperties(servicesDict, types.ForbiddenProperties)

			if len(forbidden) > 0 {
				return nil, &ForbiddenPropertiesError{Properties: forbidden}
			}
		}
	}

	if err := schema.Validate(configDict, schema.Version(configDict)); err != nil {
		return nil, err
	}

	cfg := types.Config{}
	if services, ok := configDict["services"]; ok {
		servicesConfig, err := interpolation.Interpolate(services.(types.Dict), "service", os.LookupEnv)
		if err != nil {
			return nil, err
		}

		servicesList, err := loadServices(servicesConfig, configDetails.WorkingDir)
		if err != nil {
			return nil, err
		}

		cfg.Services = servicesList
	}

	if networks, ok := configDict["networks"]; ok {
		networksConfig, err := interpolation.Interpolate(networks.(types.Dict), "network", os.LookupEnv)
		if err != nil {
			return nil, err
		}

		networksMapping, err := loadNetworks(networksConfig)
		if err != nil {
			return nil, err
		}

		cfg.Networks = networksMapping
	}

	if volumes, ok := configDict["volumes"]; ok {
		volumesConfig, err := interpolation.Interpolate(volumes.(types.Dict), "volume", os.LookupEnv)
		if err != nil {
			return nil, err
		}

		volumesMapping, err := loadVolumes(volumesConfig)
		if err != nil {
			return nil, err
		}

		cfg.Volumes = volumesMapping
	}

	if secrets, ok := configDict["secrets"]; ok {
		secretsConfig, err := interpolation.Interpolate(secrets.(types.Dict), "secret", os.LookupEnv)
		if err != nil {
			return nil, err
		}

		secretsMapping, err := loadSecrets(secretsConfig, configDetails.WorkingDir)
		if err != nil {
			return nil, err
		}

		cfg.Secrets = secretsMapping
	}

	return &cfg, nil
}

// GetUnsupportedProperties returns the list of any unsupported properties that are
// used in the Compose files.
func GetUnsupportedProperties(configDetails types.ConfigDetails) []string {
	unsupported := map[string]bool{}

	for _, service := range getServices(getConfigDict(configDetails)) {
		serviceDict := service.(types.Dict)
		for _, property := range types.UnsupportedProperties {
			if _, isSet := serviceDict[property]; isSet {
				unsupported[property] = true
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
func GetDeprecatedProperties(configDetails types.ConfigDetails) map[string]string {
	return getProperties(getServices(getConfigDict(configDetails)), types.DeprecatedProperties)
}

func getProperties(services types.Dict, propertyMap map[string]string) map[string]string {
	output := map[string]string{}

	for _, service := range services {
		if serviceDict, ok := service.(types.Dict); ok {
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

// TODO: resolve multiple files into a single config
func getConfigDict(configDetails types.ConfigDetails) types.Dict {
	return configDetails.ConfigFiles[0].Config
}

func getServices(configDict types.Dict) types.Dict {
	if services, ok := configDict["services"]; ok {
		if servicesDict, ok := services.(types.Dict); ok {
			return servicesDict
		}
	}

	return types.Dict{}
}

func transform(source map[string]interface{}, target interface{}) error {
	data := mapstructure.Metadata{}
	config := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			transformHook,
			mapstructure.StringToTimeDurationHookFunc()),
		Result:   target,
		Metadata: &data,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	err = decoder.Decode(source)
	// TODO: log unused keys
	return err
}

func transformHook(
	source reflect.Type,
	target reflect.Type,
	data interface{},
) (interface{}, error) {
	switch target {
	case reflect.TypeOf(types.External{}):
		return transformExternal(data)
	case reflect.TypeOf(make(map[string]string, 0)):
		return transformMapStringString(source, target, data)
	case reflect.TypeOf(types.UlimitsConfig{}):
		return transformUlimits(data)
	case reflect.TypeOf(types.UnitBytes(0)):
		return loadSize(data)
	case reflect.TypeOf(types.ServiceSecretConfig{}):
		return transformServiceSecret(data)
	}
	switch target.Kind() {
	case reflect.Struct:
		return transformStruct(source, target, data)
	}
	return data, nil
}

// keys needs to be converted to strings for jsonschema
// TODO: don't use types.Dict
func convertToStringKeysRecursive(value interface{}, keyPrefix string) (interface{}, error) {
	if mapping, ok := value.(map[interface{}]interface{}); ok {
		dict := make(types.Dict)
		for key, entry := range mapping {
			str, ok := key.(string)
			if !ok {
				var location string
				if keyPrefix == "" {
					location = "at top level"
				} else {
					location = fmt.Sprintf("in %s", keyPrefix)
				}
				return nil, fmt.Errorf("Non-string key %s: %#v", location, key)
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

func loadServices(servicesDict types.Dict, workingDir string) ([]types.ServiceConfig, error) {
	var services []types.ServiceConfig

	for name, serviceDef := range servicesDict {
		serviceConfig, err := loadService(name, serviceDef.(types.Dict), workingDir)
		if err != nil {
			return nil, err
		}
		services = append(services, *serviceConfig)
	}

	return services, nil
}

func loadService(name string, serviceDict types.Dict, workingDir string) (*types.ServiceConfig, error) {
	serviceConfig := &types.ServiceConfig{}
	if err := transform(serviceDict, serviceConfig); err != nil {
		return nil, err
	}
	serviceConfig.Name = name

	if err := resolveEnvironment(serviceConfig, serviceDict, workingDir); err != nil {
		return nil, err
	}

	if err := resolveVolumePaths(serviceConfig.Volumes, workingDir); err != nil {
		return nil, err
	}

	return serviceConfig, nil
}

func resolveEnvironment(serviceConfig *types.ServiceConfig, serviceDict types.Dict, workingDir string) error {
	environment := make(map[string]string)

	if envFileVal, ok := serviceDict["env_file"]; ok {
		envFiles := loadStringOrListOfStrings(envFileVal)

		var envVars []string

		for _, file := range envFiles {
			filePath := absPath(workingDir, file)
			fileVars, err := opts.ParseEnvFile(filePath)
			if err != nil {
				return err
			}
			envVars = append(envVars, fileVars...)
		}

		for k, v := range opts.ConvertKVStringsToMap(envVars) {
			environment[k] = v
		}
	}

	for k, v := range serviceConfig.Environment {
		environment[k] = v
	}

	serviceConfig.Environment = environment

	return nil
}

func resolveVolumePaths(volumes []string, workingDir string) error {
	for i, mapping := range volumes {
		parts := strings.SplitN(mapping, ":", 2)
		if len(parts) == 1 {
			continue
		}

		if strings.HasPrefix(parts[0], ".") {
			parts[0] = absPath(workingDir, parts[0])
		}
		parts[0] = expandUser(parts[0])

		volumes[i] = strings.Join(parts, ":")
	}

	return nil
}

// TODO: make this more robust
func expandUser(path string) string {
	if strings.HasPrefix(path, "~") {
		return strings.Replace(path, "~", os.Getenv("HOME"), 1)
	}
	return path
}

func transformUlimits(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case int:
		return types.UlimitsConfig{Single: value}, nil
	case types.Dict:
		ulimit := types.UlimitsConfig{}
		ulimit.Soft = value["soft"].(int)
		ulimit.Hard = value["hard"].(int)
		return ulimit, nil
	default:
		return data, fmt.Errorf("invalid type %T for ulimits", value)
	}
}

func loadNetworks(source types.Dict) (map[string]types.NetworkConfig, error) {
	networks := make(map[string]types.NetworkConfig)
	err := transform(source, &networks)
	if err != nil {
		return networks, err
	}
	for name, network := range networks {
		if network.External.External && network.External.Name == "" {
			network.External.Name = name
			networks[name] = network
		}
	}
	return networks, nil
}

func loadVolumes(source types.Dict) (map[string]types.VolumeConfig, error) {
	volumes := make(map[string]types.VolumeConfig)
	err := transform(source, &volumes)
	if err != nil {
		return volumes, err
	}
	for name, volume := range volumes {
		if volume.External.External && volume.External.Name == "" {
			volume.External.Name = name
			volumes[name] = volume
		}
	}
	return volumes, nil
}

// TODO: remove duplicate with networks/volumes
func loadSecrets(source types.Dict, workingDir string) (map[string]types.SecretConfig, error) {
	secrets := make(map[string]types.SecretConfig)
	if err := transform(source, &secrets); err != nil {
		return secrets, err
	}
	for name, secret := range secrets {
		if secret.External.External && secret.External.Name == "" {
			secret.External.Name = name
			secrets[name] = secret
		}
		if secret.File != "" {
			secret.File = absPath(workingDir, secret.File)
		}
	}
	return secrets, nil
}

func absPath(workingDir string, filepath string) string {
	if path.IsAbs(filepath) {
		return filepath
	}
	return path.Join(workingDir, filepath)
}

func transformStruct(
	source reflect.Type,
	target reflect.Type,
	data interface{},
) (interface{}, error) {
	structValue, ok := data.(map[string]interface{})
	if !ok {
		// FIXME: this is necessary because of convertToStringKeysRecursive
		structValue, ok = data.(types.Dict)
		if !ok {
			panic(fmt.Sprintf(
				"transformStruct called with non-map type: %T, %s", data, data))
		}
	}

	var err error
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		fieldTag := field.Tag.Get("compose")

		yamlName := toYAMLName(field.Name)
		value, ok := structValue[yamlName]
		if !ok {
			continue
		}

		structValue[yamlName], err = convertField(
			fieldTag, reflect.TypeOf(value), field.Type, value)
		if err != nil {
			return nil, fmt.Errorf("field %s: %s", yamlName, err.Error())
		}
	}
	return structValue, nil
}

func transformMapStringString(
	source reflect.Type,
	target reflect.Type,
	data interface{},
) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		return toMapStringString(value), nil
	case types.Dict:
		return toMapStringString(value), nil
	case map[string]string:
		return value, nil
	default:
		return data, fmt.Errorf("invalid type %T for map[string]string", value)
	}
}

func convertField(
	fieldTag string,
	source reflect.Type,
	target reflect.Type,
	data interface{},
) (interface{}, error) {
	switch fieldTag {
	case "":
		return data, nil
	case "healthcheck":
		return loadHealthcheck(data)
	case "list_or_dict_equals":
		return loadMappingOrList(data, "="), nil
	case "list_or_dict_colon":
		return loadMappingOrList(data, ":"), nil
	case "list_or_struct_map":
		return loadListOrStructMap(data, target)
	case "string_or_list":
		return loadStringOrListOfStrings(data), nil
	case "list_of_strings_or_numbers":
		return loadListOfStringsOrNumbers(data), nil
	case "shell_command":
		return loadShellCommand(data)
	case "size":
		return loadSize(data)
	case "-":
		return nil, nil
	}
	return data, nil
}

func transformExternal(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case bool:
		return map[string]interface{}{"external": value}, nil
	case types.Dict:
		return map[string]interface{}{"external": true, "name": value["name"]}, nil
	case map[string]interface{}:
		return map[string]interface{}{"external": true, "name": value["name"]}, nil
	default:
		return data, fmt.Errorf("invalid type %T for external", value)
	}
}

func transformServiceSecret(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return map[string]interface{}{"source": value}, nil
	case types.Dict:
		return data, nil
	case map[string]interface{}:
		return data, nil
	default:
		return data, fmt.Errorf("invalid type %T for external", value)
	}

}

func toYAMLName(name string) string {
	nameParts := fieldNameRegexp.FindAllString(name, -1)
	for i, p := range nameParts {
		nameParts[i] = strings.ToLower(p)
	}
	return strings.Join(nameParts, "_")
}

func loadListOrStructMap(value interface{}, target reflect.Type) (interface{}, error) {
	if list, ok := value.([]interface{}); ok {
		mapValue := map[interface{}]interface{}{}
		for _, name := range list {
			mapValue[name] = nil
		}
		return mapValue, nil
	}

	return value, nil
}

func loadListOfStringsOrNumbers(value interface{}) []string {
	list := value.([]interface{})
	result := make([]string, len(list))
	for i, item := range list {
		result[i] = fmt.Sprint(item)
	}
	return result
}

func loadStringOrListOfStrings(value interface{}) []string {
	if list, ok := value.([]interface{}); ok {
		result := make([]string, len(list))
		for i, item := range list {
			result[i] = fmt.Sprint(item)
		}
		return result
	}
	return []string{value.(string)}
}

func loadMappingOrList(mappingOrList interface{}, sep string) map[string]string {
	if mapping, ok := mappingOrList.(types.Dict); ok {
		return toMapStringString(mapping)
	}
	if list, ok := mappingOrList.([]interface{}); ok {
		result := make(map[string]string)
		for _, value := range list {
			parts := strings.SplitN(value.(string), sep, 2)
			if len(parts) == 1 {
				result[parts[0]] = ""
			} else {
				result[parts[0]] = parts[1]
			}
		}
		return result
	}
	panic(fmt.Errorf("expected a map or a slice, got: %#v", mappingOrList))
}

func loadShellCommand(value interface{}) (interface{}, error) {
	if str, ok := value.(string); ok {
		return shellwords.Parse(str)
	}
	return value, nil
}

func loadHealthcheck(value interface{}) (interface{}, error) {
	if str, ok := value.(string); ok {
		return append([]string{"CMD-SHELL"}, str), nil
	}
	return value, nil
}

func loadSize(value interface{}) (int64, error) {
	switch value := value.(type) {
	case int:
		return int64(value), nil
	case string:
		return units.RAMInBytes(value)
	}
	panic(fmt.Errorf("invalid type for size %T", value))
}

func toMapStringString(value map[string]interface{}) map[string]string {
	output := make(map[string]string)
	for key, value := range value {
		output[key] = toString(value)
	}
	return output
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
