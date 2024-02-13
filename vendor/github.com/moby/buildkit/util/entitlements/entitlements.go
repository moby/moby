package entitlements

import (
	"github.com/pkg/errors"
)

type Entitlement string

const (
	EntitlementSecurityInsecure Entitlement = "security.insecure"
	EntitlementNetworkHost      Entitlement = "network.host"
)

var all = map[Entitlement]struct{}{
	EntitlementSecurityInsecure: {},
	EntitlementNetworkHost:      {},
}

func Parse(s string) (Entitlement, error) {
	_, ok := all[Entitlement(s)]
	if !ok {
		return "", errors.Errorf("unknown entitlement %s", s)
	}
	return Entitlement(s), nil
}

func WhiteList(allowed, supported []Entitlement) (Set, error) {
	m := map[Entitlement]struct{}{}

	var supm Set
	if supported != nil {
		var err error
		supm, err = WhiteList(supported, nil)
		if err != nil { // should not happen
			return nil, err
		}
	}

	for _, e := range allowed {
		e, err := Parse(string(e))
		if err != nil {
			return nil, err
		}
		if supported != nil {
			if !supm.Allowed(e) {
				return nil, errors.Errorf("granting entitlement %s is not allowed by build daemon configuration", e)
			}
		}
		m[e] = struct{}{}
	}

	return Set(m), nil
}

type Set map[Entitlement]struct{}

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
}
