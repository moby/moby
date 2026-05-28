package opts

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type NRIOpts struct {
	Enable           bool   `json:"enable,omitempty"`
	PluginPath       string `json:"plugin-path,omitempty"`
	PluginConfigPath string `json:"plugin-config-path,omitempty"`
	SocketPath       string `json:"socket-path,omitempty"`
}

func (c *NRIOpts) UnmarshalJSON(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	type nc *NRIOpts // prevent recursion
	if err := dec.Decode(nc(c)); err != nil {
		return err
	}
	return nil
}

// NamedNRIOpts is a NamedOption and flags.Value for NRI configuration parsing.
type NamedNRIOpts struct {
	Val *NRIOpts
}

func NewNamedNRIOptsRef(val *NRIOpts) *NamedNRIOpts {
	return &NamedNRIOpts{Val: val}
}

func (p *NamedNRIOpts) Set(value string) error {
	csvReader := csv.NewReader(strings.NewReader(value))
	fields, err := csvReader.Read()
	if err != nil {
		return err
	}

	for _, field := range fields {
		key, val, _ := strings.Cut(field, "=")
		switch key {
		case "enable":
			// Assume "enable=true" if no value given, so that "--nri enable" works.
			if val == "" {
				val = "true"
			}
			en, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("invalid value for NRI enable %q: %w", val, err)
			}
			p.Val.Enable = en
		case "plugin-path":
			p.Val.PluginPath = val
		case "plugin-config-path":
			p.Val.PluginConfigPath = val
		case "socket-path":
			p.Val.SocketPath = val
		default:
			return fmt.Errorf("unexpected key '%s' in '%s'", key, field)
		}
	}
	return nil
}

// Type returns the type of this option
func (p *NamedNRIOpts) Type() string {
	return "nri-opts"
}

// String returns a string repr of this option
func (p *NamedNRIOpts) String() string {
	vals := []string{fmt.Sprintf("enable=%v", p.Val.Enable)}
	if p.Val.PluginPath != "" {
		vals = append(vals, "plugin-path="+p.Val.PluginPath)
	}
	if p.Val.PluginConfigPath != "" {
		vals = append(vals, "plugin-config-path="+p.Val.PluginConfigPath)
	}
	if p.Val.SocketPath != "" {
		vals = append(vals, "socket-path="+p.Val.SocketPath)
	}
	return strings.Join(vals, ",")
}

// Name returns the flag name of this option
func (p *NamedNRIOpts) Name() string {
	return "nri-opts"
}
