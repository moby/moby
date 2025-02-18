package entitlements

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/tonistiigi/go-csvvalue"
)

type Entitlement string

func (e Entitlement) String() string {
	return string(e)
}

const (
	EntitlementSecurityInsecure Entitlement = "security.insecure"
	EntitlementNetworkHost      Entitlement = "network.host"
	EntitlementDevice           Entitlement = "device"
)

var all = map[Entitlement]struct{}{
	EntitlementSecurityInsecure: {},
	EntitlementNetworkHost:      {},
	EntitlementDevice:           {},
}

type EntitlementsConfig interface {
	Merge(EntitlementsConfig) error
}

type DevicesConfig struct {
	Devices map[string]string
	All     bool
}

var _ EntitlementsConfig = &DevicesConfig{}

func ParseDevicesConfig(s string) (*DevicesConfig, error) {
	if s == "" {
		return &DevicesConfig{All: true}, nil
	}

	fields, err := csvvalue.Fields(s, nil)
	if err != nil {
		return nil, err
	}
	deviceName := fields[0]
	var deviceAlias string

	for _, field := range fields[1:] {
		k, v, ok := strings.Cut(field, "=")
		if !ok {
			return nil, errors.Errorf("invalid device config %q", field)
		}
		switch k {
		case "alias":
			deviceAlias = v
		default:
			return nil, errors.Errorf("unknown device config key %q", k)
		}
	}

	cfg := &DevicesConfig{Devices: map[string]string{}}

	if deviceAlias != "" {
		cfg.Devices[deviceAlias] = deviceName
	} else {
		cfg.Devices[deviceName] = ""
	}
	return cfg, nil
}

func (c *DevicesConfig) Merge(in EntitlementsConfig) error {
	c2, ok := in.(*DevicesConfig)
	if !ok {
		return errors.Errorf("cannot merge %T into %T", in, c)
	}

	if c2.All {
		c.All = true
		return nil
	}

	for k, v := range c2.Devices {
		if c.Devices == nil {
			c.Devices = map[string]string{}
		}
		c.Devices[k] = v
	}
	return nil
}

func Parse(s string) (Entitlement, EntitlementsConfig, error) {
	var cfg EntitlementsConfig
	key, rest, _ := strings.Cut(s, "=")
	switch Entitlement(key) {
	case EntitlementDevice:
		s = key
		var err error
		cfg, err = ParseDevicesConfig(rest)
		if err != nil {
			return "", nil, err
		}
	default:
	}

	_, ok := all[Entitlement(s)]
	if !ok {
		return "", nil, errors.Errorf("unknown entitlement %s", s)
	}
	return Entitlement(s), cfg, nil
}

func WhiteList(allowed, supported []Entitlement) (Set, error) {
	m := map[Entitlement]EntitlementsConfig{}

	var supm Set
	if supported != nil {
		var err error
		supm, err = WhiteList(supported, nil)
		if err != nil { // should not happen
			return nil, err
		}
	}

	for _, e := range allowed {
		e, cfg, err := Parse(string(e))
		if err != nil {
			return nil, err
		}
		if supported != nil {
			if !supm.Allowed(e) {
				return nil, errors.Errorf("granting entitlement %s is not allowed by build daemon configuration", e)
			}
		}
		if prev, ok := m[e]; ok && prev != nil {
			if err := prev.Merge(cfg); err != nil {
				return nil, err
			}
		} else {
			m[e] = cfg
		}
	}

	return Set(m), nil
}

type Set map[Entitlement]EntitlementsConfig

func (s Set) Allowed(e Entitlement) bool {
	_, ok := s[e]
	return ok
}

func (s Set) Check(v Values) error {
	if v.NetworkHost {
		if !s.Allowed(EntitlementNetworkHost) {
			return errors.Errorf("%s is not allowed", EntitlementNetworkHost)
		}
	}

	if v.SecurityInsecure {
		if !s.Allowed(EntitlementSecurityInsecure) {
			return errors.Errorf("%s is not allowed", EntitlementSecurityInsecure)
		}
	}
	return nil
}

type Values struct {
	NetworkHost      bool
	SecurityInsecure bool
	Devices          map[string]struct{}
}
