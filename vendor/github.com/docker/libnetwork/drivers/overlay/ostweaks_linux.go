package overlay

import (
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type conditionalCheck func(val1, val2 string) bool

type osValue struct {
	value   string
	checkFn conditionalCheck
}

var osConfig = map[string]osValue{
	"net.ipv4.neigh.default.gc_thresh1": {"8192", checkHigher},
	"net.ipv4.neigh.default.gc_thresh2": {"49152", checkHigher},
	"net.ipv4.neigh.default.gc_thresh3": {"65536", checkHigher},
}

func propertyIsValid(val1, val2 string, check conditionalCheck) bool {
	if check == nil || check(val1, val2) {
		return true
	}
	return false
}

func checkHigher(val1, val2 string) bool {
	val1Int, _ := strconv.ParseInt(val1, 10, 32)
	val2Int, _ := strconv.ParseInt(val2, 10, 32)
	return val1Int < val2Int
}

// writeSystemProperty writes the value to a path under /proc/sys as determined from the key.
// For e.g. net.ipv4.ip_forward translated to /proc/sys/net/ipv4/ip_forward.
func writeSystemProperty(key, value string) error {
	keyPath := strings.Replace(key, ".", "/", -1)
	return ioutil.WriteFile(path.Join("/proc/sys", keyPath), []byte(value), 0644)
}

func readSystemProperty(key string) (string, error) {
	keyPath := strings.Replace(key, ".", "/", -1)
	value, err := ioutil.ReadFile(path.Join("/proc/sys", keyPath))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func applyOStweaks() {
	for k, v := range osConfig {
		// read the existing property from disk
		oldv, err := readSystemProperty(k)
		if err != nil {
			logrus.Errorf("error reading the kernel parameter %s, error: %s", k, err)
			continue
		}

		if propertyIsValid(oldv, v.value, v.checkFn) {
			// write new prop value to disk
			if err := writeSystemProperty(k, v.value); err != nil {
				logrus.Errorf("error setting the kernel parameter %s = %s, (leaving as %s) error: %s", k, v.value, oldv, err)
				continue
			}
			logrus.Debugf("updated kernel parameter %s = %s (was %s)", k, v.value, oldv)
		}
	}
}
