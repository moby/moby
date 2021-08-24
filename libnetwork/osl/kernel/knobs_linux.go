package kernel

import (
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
)

// writeSystemProperty writes the value to a path under /proc/sys as determined from the key.
// For e.g. net.ipv4.ip_forward translated to /proc/sys/net/ipv4/ip_forward.
func writeSystemProperty(key, value string) error {
	keyPath := strings.Replace(key, ".", "/", -1)
	return os.WriteFile(path.Join("/proc/sys", keyPath), []byte(value), 0644)
}

// readSystemProperty reads the value from the path under /proc/sys and returns it
func readSystemProperty(key string) (string, error) {
	keyPath := strings.Replace(key, ".", "/", -1)
	value, err := os.ReadFile(path.Join("/proc/sys", keyPath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(value)), nil
}

// ApplyOSTweaks applies the configuration values passed as arguments
func ApplyOSTweaks(osConfig map[string]*OSValue) {
	for k, v := range osConfig {
		// read the existing property from disk
		oldv, err := readSystemProperty(k)
		if err != nil {
			logrus.WithError(err).Errorf("error reading the kernel parameter %s", k)
			continue
		}

		if propertyIsValid(oldv, v.Value, v.CheckFn) {
			// write new prop value to disk
			if err := writeSystemProperty(k, v.Value); err != nil {
				logrus.WithError(err).Errorf("error setting the kernel parameter %s = %s, (leaving as %s)", k, v.Value, oldv)
				continue
			}
			logrus.Debugf("updated kernel parameter %s = %s (was %s)", k, v.Value, oldv)
		}
	}
}
