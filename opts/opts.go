package opts

import (
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/docker/docker/api"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/utils"
)

var (
	alphaRegexp  = regexp.MustCompile(`[a-zA-Z]`)
	domainRegexp = regexp.MustCompile(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
)

func ListVar(values *[]string, names []string, usage string) {
	flag.Var(newListOptsRef(values, nil), names, usage)
}

func HostListVar(values *[]string, names []string, usage string) {
	flag.Var(newListOptsRef(values, api.ValidateHost), names, usage)
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
	if _, err := parsers.PartParser("name:alias", val); err != nil {
		return val, err
	}
	return val, nil
}

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
	if !utils.DoesEnvExist(val) {
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
	} else {
		return val, nil
	}
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
