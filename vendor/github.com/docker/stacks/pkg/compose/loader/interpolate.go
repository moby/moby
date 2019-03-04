package loader

import (
	"strconv"
	"strings"

	interp "github.com/docker/stacks/pkg/compose/interpolation"
	"github.com/pkg/errors"
)

var interpolateTypeCastMapping = map[interp.Path]interp.Cast{
	servicePath("configs", interp.PathMatchList, "mode"):             toInt,
	servicePath("secrets", interp.PathMatchList, "mode"):             toInt,
	servicePath("healthcheck", "retries"):                            toInt,
	servicePath("healthcheck", "disable"):                            toBoolean,
	servicePath("deploy", "replicas"):                                toInt,
	servicePath("deploy", "update_config", "parallelism"):            toInt,
	servicePath("deploy", "update_config", "max_failure_ratio"):      toFloat,
	servicePath("deploy", "restart_policy", "max_attempts"):          toInt,
	servicePath("ports", interp.PathMatchList, "target"):             toInt,
	servicePath("ports", interp.PathMatchList, "published"):          toInt,
	servicePath("ulimits", interp.PathMatchAll):                      toInt,
	servicePath("ulimits", interp.PathMatchAll, "hard"):              toInt,
	servicePath("ulimits", interp.PathMatchAll, "soft"):              toInt,
	servicePath("privileged"):                                        toBoolean,
	servicePath("read_only"):                                         toBoolean,
	servicePath("stdin_open"):                                        toBoolean,
	servicePath("tty"):                                               toBoolean,
	servicePath("volumes", interp.PathMatchList, "read_only"):        toBoolean,
	servicePath("volumes", interp.PathMatchList, "volume", "nocopy"): toBoolean,
	iPath("networks", interp.PathMatchAll, "external"):               toBoolean,
	iPath("networks", interp.PathMatchAll, "internal"):               toBoolean,
	iPath("networks", interp.PathMatchAll, "attachable"):             toBoolean,
	iPath("volumes", interp.PathMatchAll, "external"):                toBoolean,
	iPath("secrets", interp.PathMatchAll, "external"):                toBoolean,
	iPath("configs", interp.PathMatchAll, "external"):                toBoolean,
}

func iPath(parts ...string) interp.Path {
	return interp.NewPath(parts...)
}

func servicePath(parts ...string) interp.Path {
	return iPath(append([]string{"services", interp.PathMatchAll}, parts...)...)
}

func toInt(value string) (interface{}, error) {
	return strconv.Atoi(value)
}

func toFloat(value string) (interface{}, error) {
	return strconv.ParseFloat(value, 64)
}

// should match http://yaml.org/type/bool.html
func toBoolean(value string) (interface{}, error) {
	switch strings.ToLower(value) {
	case "y", "yes", "true", "on":
		return true, nil
	case "n", "no", "false", "off":
		return false, nil
	default:
		return nil, errors.Errorf("invalid boolean: %s", value)
	}
}

func interpolateConfig(configDict map[string]interface{}, opts interp.Options) (map[string]interface{}, error) {
	return interp.Interpolate(configDict, opts)
}
