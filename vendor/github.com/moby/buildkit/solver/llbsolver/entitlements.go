package llbsolver

import (
	"context"

	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/pkg/errors"
)

const (
	keyEntitlements = "llb.entitlements"
)

func supportedEntitlements(ents []string) []entitlements.Entitlement {
	out := []entitlements.Entitlement{} // nil means no filter
	for _, e := range ents {
		if e == string(entitlements.EntitlementNetworkHost) {
			out = append(out, entitlements.EntitlementNetworkHost)
		}
		if e == string(entitlements.EntitlementSecurityInsecure) {
			out = append(out, entitlements.EntitlementSecurityInsecure)
		}
		if e == string(entitlements.EntitlementDevice) {
			out = append(out, entitlements.EntitlementDevice)
		}
	}
	return out
}

func loadEntitlements(b solver.Builder) (entitlements.Set, error) {
	var ent entitlements.Set = map[entitlements.Entitlement]entitlements.EntitlementsConfig{}
	err := b.EachValue(context.TODO(), keyEntitlements, func(v any) error {
		set, ok := v.(entitlements.Set)
		if !ok {
			return errors.Errorf("invalid entitlements %T", v)
		}
		for k, v := range set {
			if prev, ok := ent[k]; ok && prev != nil {
				prev.Merge(v)
			} else {
				ent[k] = v
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ent, nil
}
