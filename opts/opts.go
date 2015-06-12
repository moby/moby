package opts

import (
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/blkiodev"
	"github.com/docker/docker/pkg/parsers"
)

var (
	alphaRegexp  = regexp.MustCompile(`[a-zA-Z]`)
	domainRegexp = regexp.MustCompile(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
	// DefaultHTTPPort Default HTTP Port used if only the protocol is provided to -H flag e.g. docker daemon -H tcp://
	// TODO Windows. DefaultHTTPPort is only used on Windows if a -H parameter
	// is not supplied. A better longer term solution would be to use a named
	// pipe as the default on the Windows daemon.
	// These are the IANA registered port numbers for use with Docker
	// see http://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.xhtml?search=docker
	DefaultHTTPPort = 2375 // Default HTTP Port
	// DefaultTLSHTTPPort Default HTTP Port used when TLS enabled
	DefaultTLSHTTPPort = 2376 // Default TLS encrypted HTTP Port
	// DefaultUnixSocket Path for the unix socket.
	// Docker daemon by default always listens on the default unix socket
	DefaultUnixSocket = "/var/run/docker.sock"
	// DefaultTCPHost constant defines the default host string used by docker on Windows
	DefaultTCPHost = fmt.Sprintf("tcp://%s:%d", DefaultHTTPHost, DefaultHTTPPort)
	// DefaultTLSHost constant defines the default host string used by docker for TLS sockets
	DefaultTLSHost = fmt.Sprintf("tcp://%s:%d", DefaultHTTPHost, DefaultTLSHTTPPort)
)

// ListOpts holds a list of values and a validation function.
type ListOpts struct {
	values    *[]string
	validator ValidatorFctType
}

// NewListOpts creates a new ListOpts with the specified validator.
func NewListOpts(validator ValidatorFctType) ListOpts {
	var values []string
	return *NewListOptsRef(&values, validator)
}

// NewListOptsRef creates a new ListOpts with the specified values and validator.
func NewListOptsRef(values *[]string, validator ValidatorFctType) *ListOpts {
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

// Delete removes the specified element from the slice.
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
func (opts *ListOpts) GetMap() map[string]struct{} {
	ret := make(map[string]struct{})
	for _, k := range *opts.values {
		ret[k] = struct{}{}
	}
	return ret
}

// GetAll returns the values of slice.
func (opts *ListOpts) GetAll() []string {
	return (*opts.values)
}

// GetAllOrEmpty returns the values of the slice
// or an empty slice when there are no values.
func (opts *ListOpts) GetAllOrEmpty() []string {
	v := *opts.values
	if v == nil {
		return make([]string, 0)
	}
	return v
}

// Get checks the existence of the specified key.
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

//MapOpts holds a map of values and a validation function.
type MapOpts struct {
	values    map[string]string
	validator ValidatorFctType
}

// Set validates if needed the input value and add it to the
// internal map, by splitting on '='.
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

// GetAll returns the values of MapOpts as a map.
func (opts *MapOpts) GetAll() map[string]string {
	return opts.values
}

func (opts *MapOpts) String() string {
	return fmt.Sprintf("%v", map[string]string((opts.values)))
}

// NewMapOpts creates a new MapOpts with the specified map of values and a validator.
func NewMapOpts(values map[string]string, validator ValidatorFctType) *MapOpts {
	if values == nil {
		values = make(map[string]string)
	}
	return &MapOpts{
		values:    values,
		validator: validator,
	}
}

// ValidatorFctType defines a validator function that returns a validated string and/or an error.
type ValidatorFctType func(val string) (string, error)

// ValidatorWeightFctType defines a validator function that returns a validated struct and/or an error.
type ValidatorWeightFctType func(val string) (*blkiodev.WeightDevice, error)

// ValidatorFctListType defines a validator function that returns a validated list of string and/or an error
type ValidatorFctListType func(val string) ([]string, error)

// ValidateAttach validates that the specified string is a valid attach option.
func ValidateAttach(val string) (string, error) {
	s := strings.ToLower(val)
	for _, str := range []string{"stdin", "stdout", "stderr"} {
		if s == str {
			return s, nil
		}
	}
	return val, fmt.Errorf("valid streams are STDIN, STDOUT and STDERR")
}

// ValidateWeightDevice validates that the specified string has a valid device-weight format.
func ValidateWeightDevice(val string) (*blkiodev.WeightDevice, error) {
	split := strings.SplitN(val, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("bad format: %s", val)
	}
	if !strings.HasPrefix(split[0], "/dev/") {
		return nil, fmt.Errorf("bad format for device path: %s", val)
	}
	weight, err := strconv.ParseUint(split[1], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("invalid weight for device: %s", val)
	}
	if weight > 0 && (weight < 10 || weight > 1000) {
		return nil, fmt.Errorf("invalid weight for device: %s", val)
	}

	return &blkiodev.WeightDevice{
		Path:   split[0],
		Weight: uint16(weight),
	}, nil
}

// ValidateLink validates that the specified string has a valid link format (containerName:alias).
func ValidateLink(val string) (string, error) {
	if _, _, err := parsers.ParseLink(val); err != nil {
		return val, err
	}
	return val, nil
}

// ValidDeviceMode checks if the mode for device is valid or not.
// Valid mode is a composition of r (read), w (write), and m (mknod).
func ValidDeviceMode(mode string) bool {
	var legalDeviceMode = map[rune]bool{
		'r': true,
		'w': true,
		'm': true,
	}
	if mode == "" {
		return false
	}
	for _, c := range mode {
		if !legalDeviceMode[c] {
			return false
		}
		legalDeviceMode[c] = false
	}
	return true
}

// ValidateDevice validates a path for devices
// It will make sure 'val' is in the form:
//    [host-dir:]container-path[:mode]
// It also validates the device mode.
func ValidateDevice(val string) (string, error) {
	return validatePath(val, ValidDeviceMode)
}

func validatePath(val string, validator func(string) bool) (string, error) {
	var containerPath string
	var mode string

	if strings.Count(val, ":") > 2 {
		return val, fmt.Errorf("bad format for path: %s", val)
	}

	split := strings.SplitN(val, ":", 3)
	if split[0] == "" {
		return val, fmt.Errorf("bad format for path: %s", val)
	}
	switch len(split) {
	case 1:
		containerPath = split[0]
		val = path.Clean(containerPath)
	case 2:
		if isValid := validator(split[1]); isValid {
			containerPath = split[0]
			mode = split[1]
			val = fmt.Sprintf("%s:%s", path.Clean(containerPath), mode)
		} else {
			containerPath = split[1]
			val = fmt.Sprintf("%s:%s", split[0], path.Clean(containerPath))
		}
	case 3:
		containerPath = split[1]
		mode = split[2]
		if isValid := validator(split[2]); !isValid {
			return val, fmt.Errorf("bad mode specified: %s", mode)
		}
		val = fmt.Sprintf("%s:%s:%s", split[0], containerPath, mode)
	}

	if !path.IsAbs(containerPath) {
		return val, fmt.Errorf("%s is not an absolute path", containerPath)
	}
	return val, nil
}

// ValidateEnv validates an environment variable and returns it.
// If no value is specified, it returns the current value using os.Getenv.
//
// As on ParseEnvFile and related to #16585, environment variable names
// are not validate what so ever, it's up to application inside docker
// to validate them or not.
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

// ValidateIPAddress validates an Ip address.
func ValidateIPAddress(val string) (string, error) {
	var ip = net.ParseIP(strings.TrimSpace(val))
	if ip != nil {
		return ip.String(), nil
	}
	return "", fmt.Errorf("%s is not an ip address", val)
}

// ValidateMACAddress validates a MAC address.
func ValidateMACAddress(val string) (string, error) {
	_, err := net.ParseMAC(strings.TrimSpace(val))
	if err != nil {
		return "", err
	}
	return val, nil
}

// ValidateDNSSearch validates domain for resolvconf search configuration.
// A zero length domain is represented by a dot (.).
func ValidateDNSSearch(val string) (string, error) {
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

// ValidateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost are in the form of name:ip where the ip has to be a valid ip (ipv4 or ipv6).
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

// ValidateLabel validates that the specified string is a valid label, and returns it.
// Labels are in the form on key=value.
func ValidateLabel(val string) (string, error) {
	if strings.Count(val, "=") < 1 {
		return "", fmt.Errorf("bad attribute format: %s", val)
	}
	return val, nil
}

// ValidateHost validates that the specified string is a valid host and returns it.
func ValidateHost(val string) (string, error) {
	_, err := parsers.ParseDockerDaemonHost(DefaultTCPHost, DefaultTLSHost, DefaultUnixSocket, "", val)
	if err != nil {
		return val, err
	}
	// Note: unlike most flag validators, we don't return the mutated value here
	//       we need to know what the user entered later (using ParseHost) to adjust for tls
	return val, nil
}

// ParseHost and set defaults for a Daemon host string
func ParseHost(defaultHost, val string) (string, error) {
	host, err := parsers.ParseDockerDaemonHost(DefaultTCPHost, DefaultTLSHost, DefaultUnixSocket, defaultHost, val)
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
