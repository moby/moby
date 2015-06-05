package opts

import (
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strings"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/ulimit"
)

var (
	alphaRegexp     = regexp.MustCompile(`[a-zA-Z]`)
	domainRegexp    = regexp.MustCompile(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
	DefaultHTTPHost = "127.0.0.1" // Default HTTP Host used if only port is provided to -H flag e.g. docker -d -H tcp://:8080
	// TODO Windows. DefaultHTTPPort is only used on Windows if a -H parameter
	// is not supplied. A better longer term solution would be to use a named
	// pipe as the default on the Windows daemon.
	DefaultHTTPPort   = 2375                   // Default HTTP Port
	DefaultUnixSocket = "/var/run/docker.sock" // Docker daemon by default always listens on the default unix socket
)

func ListVar(values *[]string, names []string, usage string) {
	flag.Var(newListOptsRef(values, nil), names, usage)
}

func MapVar(values map[string]string, names []string, usage string) {
	flag.Var(newMapOpt(values, nil), names, usage)
}

func LogOptsVar(values map[string]string, names []string, usage string) {
	flag.Var(newMapOpt(values, nil), names, usage)
}

func HostListVar(values *[]string, names []string, usage string) {
	flag.Var(newListOptsRef(values, ValidateHost), names, usage)
}

func IPListVar(values *[]string, names []string, usage string) {
	flag.Var(newListOptsRef(values, ValidateIPAddress), names, usage)
}

func DnsSearchListVar(values *[]string, names []string, usage string) {
	flag.Var(newListOptsRef(values, ValidateDnsSearch), names, usage)
}

func IPVar(value *net.IP, names []string, defaultValue, usage string) {
	flag.Var(NewIpOpt(value, defaultValue), names, usage)
}

func LabelListVar(values *[]string, names []string, usage string) {
	flag.Var(newListOptsRef(values, ValidateLabel), names, usage)
}

func UlimitMapVar(values map[string]*ulimit.Ulimit, names []string, usage string) {
	flag.Var(NewUlimitOpt(values), names, usage)
}

// ListOpts type
type ListOpts struct {
	values    *[]string
	validator ValidatorFctType
}

func NewListOpts(validator ValidatorFctType) ListOpts {
	var values []string
	return *newListOptsRef(&values, validator)
}

func newListOptsRef(values *[]string, validator ValidatorFctType) *ListOpts {
	return &ListOpts{
		values:    values,
		validator: validator,
	}
}

func (opts *ListOpts) String() string {
	return fmt.Sprintf("%v", []string((*opts.values)))
}

// Set validates if needed the input value and add it to the
// internal slice.
func (opts *ListOpts) Set(value string) error {
	if opts.validator != nil {
		v, err := opts.validator(value)
		if err != nil {
			return err
		}
		value = v
	}
	(*opts.values) = append((*opts.values), value)
	return nil
}

// Delete remove the given element from the slice.
func (opts *ListOpts) Delete(key string) {
	for i, k := range *opts.values {
		if k == key {
			(*opts.values) = append((*opts.values)[:i], (*opts.values)[i+1:]...)
			return
		}
	}
}

// GetMap returns the content of values in a map in order to avoid
// duplicates.
// FIXME: can we remove this?
func (opts *ListOpts) GetMap() map[string]struct{} {
	ret := make(map[string]struct{})
	for _, k := range *opts.values {
		ret[k] = struct{}{}
	}
	return ret
}

// GetAll returns the values' slice.
// FIXME: Can we remove this?
func (opts *ListOpts) GetAll() []string {
	return (*opts.values)
}

// Get checks the existence of the given key.
func (opts *ListOpts) Get(key string) bool {
	for _, k := range *opts.values {
		if k == key {
			return true
		}
	}
	return false
}

// Len returns the amount of element in the slice.
func (opts *ListOpts) Len() int {
	return len((*opts.values))
}

//MapOpts type
type MapOpts struct {
	values    map[string]string
	validator ValidatorFctType
}

func (opts *MapOpts) Set(value string) error {
	if opts.validator != nil {
		v, err := opts.validator(value)
		if err != nil {
			return err
		}
		value = v
	}
	vals := strings.SplitN(value, "=", 2)
	if len(vals) == 1 {
		(opts.values)[vals[0]] = ""
	} else {
		(opts.values)[vals[0]] = vals[1]
	}
	return nil
}

func (opts *MapOpts) String() string {
	return fmt.Sprintf("%v", map[string]string((opts.values)))
}

func newMapOpt(values map[string]string, validator ValidatorFctType) *MapOpts {
	return &MapOpts{
		values:    values,
		validator: validator,
	}
}

// Validators
type ValidatorFctType func(val string) (string, error)
type ValidatorFctListType func(val string) ([]string, error)

func ValidateAttach(val string) (string, error) {
	s := strings.ToLower(val)
	for _, str := range []string{"stdin", "stdout", "stderr"} {
		if s == str {
			return s, nil
		}
	}
	return val, fmt.Errorf("valid streams are STDIN, STDOUT and STDERR.")
}

func ValidateLink(val string) (string, error) {
	if _, _, err := parsers.ParseLink(val); err != nil {
		return val, err
	}
	return val, nil
}

// ValidatePath will make sure 'val' is in the form:
//    [host-dir:]container-path[:rw|ro]  - but doesn't validate the mode part
func ValidatePath(val string) (string, error) {
	var containerPath string

	if strings.Count(val, ":") > 2 {
		return val, fmt.Errorf("bad format for volumes: %s", val)
	}

	splited := strings.SplitN(val, ":", 2)
	if len(splited) == 1 {
		containerPath = splited[0]
		val = path.Clean(splited[0])
	} else {
		containerPath = splited[1]
		val = fmt.Sprintf("%s:%s", splited[0], path.Clean(splited[1]))
	}

	if !path.IsAbs(containerPath) {
		return val, fmt.Errorf("%s is not an absolute path", containerPath)
	}
	return val, nil
}

func ValidateEnv(val string) (string, error) {
	arr := strings.Split(val, "=")
	if len(arr) > 1 {
		return val, nil
	}
	if !doesEnvExist(val) {
		return val, nil
	}
	return fmt.Sprintf("%s=%s", val, os.Getenv(val)), nil
}

func ValidateIPAddress(val string) (string, error) {
	var ip = net.ParseIP(strings.TrimSpace(val))
	if ip != nil {
		return ip.String(), nil
	}
	return "", fmt.Errorf("%s is not an ip address", val)
}

func ValidateMACAddress(val string) (string, error) {
	_, err := net.ParseMAC(strings.TrimSpace(val))
	if err != nil {
		return "", err
	}
	return val, nil
}

// Validates domain for resolvconf search configuration.
// A zero length domain is represented by .
func ValidateDnsSearch(val string) (string, error) {
	if val = strings.Trim(val, " "); val == "." {
		return val, nil
	}
	return validateDomain(val)
}

func validateDomain(val string) (string, error) {
	if alphaRegexp.FindString(val) == "" {
		return "", fmt.Errorf("%s is not a valid domain", val)
	}
	ns := domainRegexp.FindSubmatch([]byte(val))
	if len(ns) > 0 && len(ns[1]) < 255 {
		return string(ns[1]), nil
	}
	return "", fmt.Errorf("%s is not a valid domain", val)
}

func ValidateExtraHost(val string) (string, error) {
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return "", fmt.Errorf("bad format for add-host: %q", val)
	}
	if _, err := ValidateIPAddress(arr[1]); err != nil {
		return "", fmt.Errorf("invalid IP address in add-host: %q", arr[1])
	}
	return val, nil
}

func ValidateLabel(val string) (string, error) {
	if strings.Count(val, "=") != 1 {
		return "", fmt.Errorf("bad attribute format: %s", val)
	}
	return val, nil
}

func ValidateHost(val string) (string, error) {
	host, err := parsers.ParseHost(DefaultHTTPHost, DefaultUnixSocket, val)
	if err != nil {
		return val, err
	}
	return host, nil
}

func doesEnvExist(name string) bool {
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if parts[0] == name {
			return true
		}
	}
	return false
}
