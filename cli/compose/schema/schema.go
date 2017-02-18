package schema

//go:generate go-bindata -pkg schema -nometadata data

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

const (
	defaultVersion = "1.0"
	versionField   = "version"
)

type portsFormatChecker struct{}

func (checker portsFormatChecker) IsFormat(input string) bool {
	// TODO: implement this
	return true
}

type durationFormatChecker struct{}

func (checker durationFormatChecker) IsFormat(input string) bool {
	_, err := time.ParseDuration(input)
	return err == nil
}

func init() {
	gojsonschema.FormatCheckers.Add("expose", portsFormatChecker{})
	gojsonschema.FormatCheckers.Add("ports", portsFormatChecker{})
	gojsonschema.FormatCheckers.Add("duration", durationFormatChecker{})
}

// Version returns the version of the config, defaulting to version 1.0
func Version(config map[string]interface{}) string {
	version, ok := config[versionField]
	if !ok {
		return defaultVersion
	}
	return normalizeVersion(fmt.Sprintf("%v", version))
}

func normalizeVersion(version string) string {
	switch version {
	case "3":
		return "3.0"
	default:
		return version
	}
}

// Validate uses the jsonschema to validate the configuration
func Validate(config map[string]interface{}, version string) error {
	schemaData, err := Asset(fmt.Sprintf("data/config_schema_v%s.json", version))
	if err != nil {
		return errors.Errorf("unsupported Compose file version: %s", version)
	}

	schemaLoader := gojsonschema.NewStringLoader(string(schemaData))
	dataLoader := gojsonschema.NewGoLoader(config)

	result, err := gojsonschema.Validate(schemaLoader, dataLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		return toError(result)
	}

	return nil
}

func toError(result *gojsonschema.Result) error {
	err := getMostSpecificError(result.Errors())
	description := getDescription(err)
	return fmt.Errorf("%s %s", err.Field(), description)
}

func getDescription(err gojsonschema.ResultError) string {
	if err.Type() == "invalid_type" {
		if expectedType, ok := err.Details()["expected"].(string); ok {
			return fmt.Sprintf("must be a %s", humanReadableType(expectedType))
		}
	}

	return err.Description()
}

func humanReadableType(definition string) string {
	if definition[0:1] == "[" {
		allTypes := strings.Split(definition[1:len(definition)-1], ",")
		for i, t := range allTypes {
			allTypes[i] = humanReadableType(t)
		}
		return fmt.Sprintf(
			"%s or %s",
			strings.Join(allTypes[0:len(allTypes)-1], ", "),
			allTypes[len(allTypes)-1],
		)
	}
	if definition == "object" {
		return "mapping"
	}
	if definition == "array" {
		return "list"
	}
	return definition
}

func getMostSpecificError(errors []gojsonschema.ResultError) gojsonschema.ResultError {
	var mostSpecificError gojsonschema.ResultError

	for _, err := range errors {
		if mostSpecificError == nil {
			mostSpecificError = err
		} else if specificity(err) > specificity(mostSpecificError) {
			mostSpecificError = err
		} else if specificity(err) == specificity(mostSpecificError) {
			// Invalid type errors win in a tie-breaker for most specific field name
			if err.Type() == "invalid_type" && mostSpecificError.Type() != "invalid_type" {
				mostSpecificError = err
			}
		}
	}

	return mostSpecificError
}

func specificity(err gojsonschema.ResultError) int {
	return len(strings.Split(err.Field(), "."))
}
