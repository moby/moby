package cache

import (
	"strconv"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/local"
	"github.com/pkg/errors"
)

func init() {
	for k, v := range local.LogOptKeys {
		builtInCacheLogOpts[cachePrefix+k] = v
	}
	logger.AddBuiltinLogOpts(builtInCacheLogOpts)
	logger.RegisterExternalValidator(validateLogCacheOpts)
}

func validateLogCacheOpts(cfg map[string]string) error {
	if v := cfg[cacheDisabledKey]; v != "" {
		_, err := strconv.ParseBool(v)
		if err != nil {
			return errors.Errorf("invalid value for option %s: %s", cacheDisabledKey, cfg[cacheDisabledKey])
		}
	}
	return nil
}

// MergeDefaultLogConfig reads the default log opts and makes sure that any caching related keys that exist there are
// added to dst.
func MergeDefaultLogConfig(dst, defaults map[string]string) {
	for k, v := range defaults {
		if !builtInCacheLogOpts[k] {
			continue
		}
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}
