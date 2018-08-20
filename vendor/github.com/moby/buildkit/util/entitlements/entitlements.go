package entitlements

import "github.com/pkg/errors"

type Entitlement string

const (
	EntitlementSecurityConfined   Entitlement = "security.confined"
	EntitlementSecurityUnconfined Entitlement = "security.unconfined" // unimplemented
	EntitlementNetworkHost        Entitlement = "network.host"
	EntitlementNetworkNone        Entitlement = "network.none"
)

var all = map[Entitlement]struct{}{
	EntitlementSecurityConfined:   {},
	EntitlementSecurityUnconfined: {},
	EntitlementNetworkHost:        {},
	EntitlementNetworkNone:        {},
}

var defaults = map[Entitlement]struct{}{
	EntitlementSecurityConfined: {},
	EntitlementNetworkNone:      {},
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
				return nil, errors.Errorf("entitlement %s is not allowed", e)
			}
		}
		m[e] = struct{}{}
	}

	for e := range defaults {
		m[e] = struct{}{}
	}
	return Set(m), nil
}

type Set map[Entitlement]struct{}

func (s Set) Allowed(e Entitlement) bool {
	_, ok := s[e]
	return ok
}
