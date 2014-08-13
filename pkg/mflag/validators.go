package mflag

import (
	"fmt"
	"github.com/docker/docker/pkg/parsers"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/api"
)

// HostListVar defines a "list of hostnames" flag with specified name,
// default value, and usage string. The argument p points to a string variable
// in which to store the value of the flag.
func HostListVar(values *[]string, names []string, usage string) {
	Var(Filter((*List)(values), api.ValidateHost), names, usage)
}

// IPListVar defines a "list of IP addresses" flag with specified name,
// default value, and usage string. The argument p points to a list variable
// in which to store the value of the flag.
func IPListVar(values *[]string, names []string, usage string) {
	Var(Filter((*List)(values), validateIPAddress), names, usage)
}

// DnsSearchList defines a "list of DNS search domains" flag with specified name,
// default value, and usage string. The argument p points to a list variable
// in which to store the value of the flag.
func DnsSearchListVar(values *[]string, names []string, usage string) {
	Var(Filter((*List)(values), validateDnsSearch), names, usage)
}

// PathPairListVar defines a "list of path pairs" flag with specified name,
// default value, and usage string. The argument p points to a list variable
// in which to store the value of the flag.
//
// A path pair is a string of the form path1[:path2], where path1 and path2
// are valid filesystem paths.
func PathPairListVar(values *[]string, names []string, usage string) {
	Var(Filter((*List)(values), validatePathPair), names, usage)
}

// PathPairSetVar defines a "set of unique path pairs" flag with specified name,
// default value, and usage string. The argument p points to a map variable
// in which to store the value of the flag.
//
// A path pair is a string of the form path1[:path2], where path1 and path2
// are valid filesystem paths.
func PathPairSetVar(values *map[string]struct{}, names []string, usage string) {
	Var(Filter((*StringSet)(values), validatePathPair), names, usage)
}

// NamePairList defines a "set of unique name pairs" flag with specified name,
// default value, and usage string. The argument p points to a list variable
// in which to store the value of the flag.
//
// A name pair is a string of the form name1:name2.
func NamePairListVar(values *[]string, names []string, usage string) {
	Var(Filter((*List)(values), validateNamePair), names, usage)
}

// EnvVar defines a "environment" flag with specified name,
// default value, and usage string. The argument p points to a list variable
// in which to store the value of the flag.
//
// An environment is a list of strings of the form KEY=[VALUE].
func EnvVar(values *[]string, names []string, usage string) {
	Var(Filter((*List)(values), validateEnv), names, usage)
}

// StreamSetVar defines a "set of unique stream names" flag with specified name,
// default value, and usage string. The argument p points to a map variable
// in which to store the value of the flag.
//
// A stream name is either "stdin", "stdout" or "stderr" (case insensitive).
func StreamSetVar(values *map[string]struct{}, names []string, usage string) {
	Var(Filter((*StringSet)(values), validateStreamName), names, usage)
}

// Validators
type ValidatorFctType func(val string) (string, error)

func validateStreamName(val string) (string, error) {
	s := strings.ToLower(val)
	for _, str := range []string{"stdin", "stdout", "stderr"} {
		if s == str {
			return s, nil
		}
	}
	return val, fmt.Errorf("valid streams are STDIN, STDOUT and STDERR.")
}

func validateNamePair(val string) (string, error) {
	if _, err := parsers.PartParser("name:alias", val); err != nil {
		return val, err
	}
	return val, nil
}

func validatePathPair(val string) (string, error) {
	var containerPath string

	if strings.Count(val, ":") > 2 {
		return val, fmt.Errorf("bad format for volumes: %s", val)
	}

	splited := strings.SplitN(val, ":", 2)
	if len(splited) == 1 {
		containerPath = splited[0]
		val = filepath.Clean(splited[0])
	} else {
		containerPath = splited[1]
		val = fmt.Sprintf("%s:%s", splited[0], filepath.Clean(splited[1]))
	}

	if !filepath.IsAbs(containerPath) {
		return val, fmt.Errorf("%s is not an absolute path", containerPath)
	}
	return val, nil
}

func validateEnv(val string) (string, error) {
	arr := strings.Split(val, "=")
	if len(arr) > 1 {
		return val, nil
	}
	return fmt.Sprintf("%s=%s", val, os.Getenv(val)), nil
}

func validateIPAddress(val string) (string, error) {
	var ip = net.ParseIP(strings.TrimSpace(val))
	if ip != nil {
		return ip.String(), nil
	}
	return "", fmt.Errorf("%s is not an ip address", val)
}

// validates domain for resolvconf search configuration.
// A zero length domain is represented by .
func validateDnsSearch(val string) (string, error) {
	if val = strings.Trim(val, " "); val == "." {
		return val, nil
	}
	return validateDomain(val)
}

func validateDomain(val string) (string, error) {
	alpha := regexp.MustCompile(`[a-zA-Z]`)
	if alpha.FindString(val) == "" {
		return "", fmt.Errorf("%s is not a valid domain", val)
	}
	re := regexp.MustCompile(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
	ns := re.FindSubmatch([]byte(val))
	if len(ns) > 0 {
		return string(ns[1]), nil
	}
	return "", fmt.Errorf("%s is not a valid domain", val)
}
