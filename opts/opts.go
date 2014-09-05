package opts

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/api"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
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

func ValidateEnv(val string) (string, error) {
	arr := strings.Split(val, "=")
	if len(arr) > 1 {
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

// Validates domain for resolvconf search configuration.
// A zero length domain is represented by .
func ValidateDnsSearch(val string) (string, error) {
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
