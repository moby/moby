package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"os"
	"path/filepath"
	"strings"
)

// ListOpts type
type ListOpts struct {
	values    []string
	validator ValidatorFctType
}

func NewListOpts(validator ValidatorFctType) ListOpts {
	return ListOpts{
		validator: validator,
	}
}

func (opts *ListOpts) String() string {
	return fmt.Sprintf("%v", []string(opts.values))
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
	opts.values = append(opts.values, value)
	return nil
}

// Delete remove the given element from the slice.
func (opts *ListOpts) Delete(key string) {
	for i, k := range opts.values {
		if k == key {
			opts.values = append(opts.values[:i], opts.values[i+1:]...)
			return
		}
	}
}

// GetMap returns the content of values in a map in order to avoid
// duplicates.
// FIXME: can we remove this?
func (opts *ListOpts) GetMap() map[string]struct{} {
	ret := make(map[string]struct{})
	for _, k := range opts.values {
		ret[k] = struct{}{}
	}
	return ret
}

// GetAll returns the values' slice.
// FIXME: Can we remove this?
func (opts *ListOpts) GetAll() []string {
	return opts.values
}

// Get checks the existence of the given key.
func (opts *ListOpts) Get(key string) bool {
	for _, k := range opts.values {
		if k == key {
			return true
		}
	}
	return false
}

// Len returns the amount of element in the slice.
func (opts *ListOpts) Len() int {
	return len(opts.values)
}

// Validators
type ValidatorFctType func(val string) (string, error)

func ValidateAttach(val string) (string, error) {
	if val != "stdin" && val != "stdout" && val != "stderr" {
		return val, fmt.Errorf("Unsupported stream name: %s", val)
	}
	return val, nil
}

func ValidateLink(val string) (string, error) {
	if _, err := parseLink(val); err != nil {
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

func ValidateHost(val string) (string, error) {
	host, err := utils.ParseHost(DEFAULTHTTPHOST, DEFAULTHTTPPORT, val)
	if err != nil {
		return val, err
	}
	return host, nil
}
