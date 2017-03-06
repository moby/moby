package opts

import (
	"fmt"
	"math/big"
	"net"
	"path"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/filters"
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
	return fmt.Sprintf("%v", []string((*opts.values)))
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

// ValidateLabel validates that the specified string is a valid label, and returns it.
// Labels are in the form on key=value.
func ValidateLabel(val string) (string, error) {
	if strings.Count(val, "=") < 1 {
		return "", fmt.Errorf("bad attribute format: %s", val)
	}
	return val, nil
}

// ValidateSysctl validates a sysctl and returns it.
func ValidateSysctl(val string) (string, error) {
	validSysctlMap := map[string]bool{
		"kernel.msgmax":          true,
		"kernel.msgmnb":          true,
		"kernel.msgmni":          true,
		"kernel.sem":             true,
		"kernel.shmall":          true,
		"kernel.shmmax":          true,
		"kernel.shmmni":          true,
		"kernel.shm_rmid_forced": true,
	}
	validSysctlPrefixes := []string{
		"net.",
		"fs.mqueue.",
	}
	arr := strings.Split(val, "=")
	if len(arr) < 2 {
		return "", fmt.Errorf("sysctl '%s' is not whitelisted", val)
	}
	if validSysctlMap[arr[0]] {
		return val, nil
	}

	for _, vp := range validSysctlPrefixes {
		if strings.HasPrefix(arr[0], vp) {
			return val, nil
		}
	}
	return "", fmt.Errorf("sysctl '%s' is not whitelisted", val)
}

// FilterOpt is a flag type for validating filters
type FilterOpt struct {
	filter filters.Args
}

// NewFilterOpt returns a new FilterOpt
func NewFilterOpt() FilterOpt {
	return FilterOpt{filter: filters.NewArgs()}
}

func (o *FilterOpt) String() string {
	repr, err := filters.ToParam(o.filter)
	if err != nil {
		return "invalid filters"
	}
	return repr
}

// Set sets the value of the opt by parsing the command line value
func (o *FilterOpt) Set(value string) error {
	var err error
	o.filter, err = filters.ParseFlag(value, o.filter)
	return err
}

// Type returns the option type
func (o *FilterOpt) Type() string {
	return "filter"
}

// Value returns the value of this option
func (o *FilterOpt) Value() filters.Args {
	return o.filter
}

// NanoCPUs is a type for fixed point fractional number.
type NanoCPUs int64

// String returns the string format of the number
func (c *NanoCPUs) String() string {
	return big.NewRat(c.Value(), 1e9).FloatString(3)
}

// Set sets the value of the NanoCPU by passing a string
func (c *NanoCPUs) Set(value string) error {
	cpus, err := ParseCPUs(value)
	*c = NanoCPUs(cpus)
	return err
}

// Type returns the type
func (c *NanoCPUs) Type() string {
	return "decimal"
}

// Value returns the value in int64
func (c *NanoCPUs) Value() int64 {
	return int64(*c)
}

// ParseCPUs takes a string ratio and returns an integer value of nano cpus
func ParseCPUs(value string) (int64, error) {
	cpu, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, fmt.Errorf("failed to parse %v as a rational number", value)
	}
	nano := cpu.Mul(cpu, big.NewRat(1e9, 1))
	if !nano.IsInt() {
		return 0, fmt.Errorf("value is too precise")
	}
	return nano.Num().Int64(), nil
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

// ValidateLink validates that the specified string has a valid link format (containerName:alias).
func ValidateLink(val string) (string, error) {
	_, _, err := ParseLink(val)
	return val, err
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

// MemSwapBytes is a type for human readable memory bytes (like 128M, 2g, etc).
// It differs from MemBytes in that -1 is valid and the default.
type MemSwapBytes int64

// Set sets the value of the MemSwapBytes by passing a string
func (m *MemSwapBytes) Set(value string) error {
	if value == "-1" {
		*m = MemSwapBytes(-1)
		return nil
	}
	val, err := units.RAMInBytes(value)
	*m = MemSwapBytes(val)
	return err
}

// Type returns the type
func (m *MemSwapBytes) Type() string {
	return "bytes"
}

// Value returns the value in int64
func (m *MemSwapBytes) Value() int64 {
	return int64(*m)
}

func (m *MemSwapBytes) String() string {
	b := MemBytes(*m)
	return b.String()
}

// UnmarshalJSON is the customized unmarshaler for MemSwapBytes
func (m *MemSwapBytes) UnmarshalJSON(s []byte) error {
	b := MemBytes(*m)
	return b.UnmarshalJSON(s)
}
