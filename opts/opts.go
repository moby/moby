package opts // import "github.com/docker/docker/opts"

import (
	"fmt"
	"net"
	"path"
	"regexp"
	"strings"

	units "github.com/docker/go-units"
)

var (
	alphaRegexp  = regexp.MustCompile(`[a-zA-Z]`)
	domainRegexp = regexp.MustCompile(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
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
	if len(*opts.values) == 0 {
		return ""
	}
	return fmt.Sprintf("%v", *opts.values)
}

// Set validates if needed the input value and adds it to the
// internal slice.
func (opts *ListOpts) Set(value string) error {
	if opts.validator != nil {
		v, err := opts.validator(value)
		if err != nil {
			return err
		}
		value = v
	}
	*opts.values = append(*opts.values, value)
	return nil
}

// Delete removes the specified element from the slice.
func (opts *ListOpts) Delete(key string) {
	for i, k := range *opts.values {
		if k == key {
			*opts.values = append((*opts.values)[:i], (*opts.values)[i+1:]...)
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
	return *opts.values
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
	return len(*opts.values)
}

// Type returns a string name for this Option type
func (opts *ListOpts) Type() string {
	return "list"
}

// WithValidator returns the ListOpts with validator set.
func (opts *ListOpts) WithValidator(validator ValidatorFctType) *ListOpts {
	opts.validator = validator
	return opts
}

// NamedOption is an interface that list and map options
// with names implement.
type NamedOption interface {
	Name() string
}

// NamedListOpts is a ListOpts with a configuration name.
// This struct is useful to keep reference to the assigned
// field name in the internal configuration struct.
type NamedListOpts struct {
	name string
	ListOpts
}

var _ NamedOption = &NamedListOpts{}

// NewNamedListOptsRef creates a reference to a new NamedListOpts struct.
func NewNamedListOptsRef(name string, values *[]string, validator ValidatorFctType) *NamedListOpts {
	return &NamedListOpts{
		name:     name,
		ListOpts: *NewListOptsRef(values, validator),
	}
}

// Name returns the name of the NamedListOpts in the configuration.
func (o *NamedListOpts) Name() string {
	return o.name
}

// NamedMapMapOpts is a MapMapOpts with a configuration name.
// This struct is useful to keep reference to the assigned
// field name in the internal configuration struct.
type NamedMapMapOpts struct {
	name string
	MapMapOpts
}

// NewNamedMapMapOpts creates a reference to a new NamedMapOpts struct.
func NewNamedMapMapOpts(name string, values map[string]map[string]string, validator ValidatorFctType) *NamedMapMapOpts {
	return &NamedMapMapOpts{
		name:       name,
		MapMapOpts: *NewMapMapOpts(values, validator),
	}
}

// Name returns the name of the NamedListOpts in the configuration.
func (o *NamedMapMapOpts) Name() string {
	return o.name
}

// MapMapOpts holds a map of maps of values and a validation function.
type MapMapOpts struct {
	values    map[string]map[string]string
	validator ValidatorFctType
}

// Set validates if needed the input value and add it to the
// internal map, by splitting on '='.
func (opts *MapMapOpts) Set(value string) error {
	if opts.validator != nil {
		v, err := opts.validator(value)
		if err != nil {
			return err
		}
		value = v
	}
	rk, rv, found := strings.Cut(value, "=")
	if !found {
		return fmt.Errorf("invalid value %q for map option, should be root-key=key=value", value)
	}
	k, v, found := strings.Cut(rv, "=")
	if !found {
		return fmt.Errorf("invalid value %q for map option, should be root-key=key=value", value)
	}
	if _, ok := opts.values[rk]; !ok {
		opts.values[rk] = make(map[string]string)
	}
	opts.values[rk][k] = v
	return nil
}

// GetAll returns the values of MapOpts as a map.
func (opts *MapMapOpts) GetAll() map[string]map[string]string {
	return opts.values
}

func (opts *MapMapOpts) String() string {
	return fmt.Sprintf("%v", opts.values)
}

// Type returns a string name for this Option type
func (opts *MapMapOpts) Type() string {
	return "mapmap"
}

// NewMapMapOpts creates a new MapMapOpts with the specified map of values and a validator.
func NewMapMapOpts(values map[string]map[string]string, validator ValidatorFctType) *MapMapOpts {
	if values == nil {
		values = make(map[string]map[string]string)
	}
	return &MapMapOpts{
		values:    values,
		validator: validator,
	}
}

// MapOpts holds a map of values and a validation function.
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
	k, v, _ := strings.Cut(value, "=")
	(opts.values)[k] = v
	return nil
}

// GetAll returns the values of MapOpts as a map.
func (opts *MapOpts) GetAll() map[string]string {
	return opts.values
}

func (opts *MapOpts) String() string {
	return fmt.Sprintf("%v", opts.values)
}

// Type returns a string name for this Option type
func (opts *MapOpts) Type() string {
	return "map"
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

// NamedMapOpts is a MapOpts struct with a configuration name.
// This struct is useful to keep reference to the assigned
// field name in the internal configuration struct.
type NamedMapOpts struct {
	name string
	MapOpts
}

var _ NamedOption = &NamedMapOpts{}

// NewNamedMapOpts creates a reference to a new NamedMapOpts struct.
func NewNamedMapOpts(name string, values map[string]string, validator ValidatorFctType) *NamedMapOpts {
	return &NamedMapOpts{
		name:    name,
		MapOpts: *NewMapOpts(values, validator),
	}
}

// Name returns the name of the NamedMapOpts in the configuration.
func (o *NamedMapOpts) Name() string {
	return o.name
}

// ValidatorFctType defines a validator function that returns a validated string and/or an error.
type ValidatorFctType func(val string) (string, error)

// ValidatorFctListType defines a validator function that returns a validated list of string and/or an error
type ValidatorFctListType func(val string) ([]string, error)

// ValidateIPAddress validates if the given value is a correctly formatted
// IP address, and returns the value in normalized form. Leading and trailing
// whitespace is allowed, but it does not allow IPv6 addresses surrounded by
// square brackets ("[::1]").
//
// Refer to [net.ParseIP] for accepted formats.
func ValidateIPAddress(val string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(val))
	if ip != nil {
		return ip.String(), nil
	}
	return "", fmt.Errorf("IP address is not correctly formatted: %s", val)
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

// ValidateLabel validates that the specified string is a valid label,
// it does not use the reserved namespaces com.docker.*, io.docker.*, org.dockerproject.*
// and returns it.
// Labels are in the form on key=value.
func ValidateLabel(val string) (string, error) {
	if strings.Count(val, "=") < 1 {
		return "", fmt.Errorf("bad attribute format: %s", val)
	}

	lowered := strings.ToLower(val)
	if strings.HasPrefix(lowered, "com.docker.") || strings.HasPrefix(lowered, "io.docker.") ||
		strings.HasPrefix(lowered, "org.dockerproject.") {
		return "", fmt.Errorf(
			"label %s is not allowed: the namespaces com.docker.*, io.docker.*, and org.dockerproject.* are reserved for internal use",
			val)
	}

	return val, nil
}

// ValidateSingleGenericResource validates that a single entry in the
// generic resource list is valid.
// i.e 'GPU=UID1' is valid however 'GPU:UID1' or 'UID1' isn't
func ValidateSingleGenericResource(val string) (string, error) {
	if strings.Count(val, "=") < 1 {
		return "", fmt.Errorf("invalid node-generic-resource format `%s` expected `name=value`", val)
	}
	return val, nil
}

// ParseLink parses and validates the specified string as a link format (name:alias)
func ParseLink(val string) (string, string, error) {
	if val == "" {
		return "", "", fmt.Errorf("empty string specified for links")
	}
	arr := strings.Split(val, ":")
	if len(arr) > 2 {
		return "", "", fmt.Errorf("bad format for links: %s", val)
	}
	if len(arr) == 1 {
		return val, val, nil
	}
	// This is kept because we can actually get a HostConfig with links
	// from an already created container and the format is not `foo:bar`
	// but `/foo:/c1/bar`
	if strings.HasPrefix(arr[0], "/") {
		_, alias := path.Split(arr[1])
		return arr[0][1:], alias, nil
	}
	return arr[0], arr[1], nil
}

// MemBytes is a type for human readable memory bytes (like 128M, 2g, etc)
type MemBytes int64

// String returns the string format of the human readable memory bytes
func (m *MemBytes) String() string {
	// NOTE: In spf13/pflag/flag.go, "0" is considered as "zero value" while "0 B" is not.
	// We return "0" in case value is 0 here so that the default value is hidden.
	// (Sometimes "default 0 B" is actually misleading)
	if m.Value() != 0 {
		return units.BytesSize(float64(m.Value()))
	}
	return "0"
}

// Set sets the value of the MemBytes by passing a string
func (m *MemBytes) Set(value string) error {
	val, err := units.RAMInBytes(value)
	*m = MemBytes(val)
	return err
}

// Type returns the type
func (m *MemBytes) Type() string {
	return "bytes"
}

// Value returns the value in int64
func (m *MemBytes) Value() int64 {
	return int64(*m)
}

// UnmarshalJSON is the customized unmarshaler for MemBytes
func (m *MemBytes) UnmarshalJSON(s []byte) error {
	if len(s) <= 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return fmt.Errorf("invalid size: %q", s)
	}
	val, err := units.RAMInBytes(string(s[1 : len(s)-1]))
	*m = MemBytes(val)
	return err
}
