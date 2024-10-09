package moby_buildkit_v1_sourcepolicy //nolint:revive

import (
	"github.com/moby/buildkit/util/gogo/proto"
	"github.com/pkg/errors"
)

// MarshalJSON implements json.Marshaler with custom marshaling for PolicyAction.
// It gives the string form of the enum value.
func (a PolicyAction) MarshalJSON() ([]byte, error) {
	return proto.MarshalJSONEnum(PolicyAction_name, int32(a))
}

func (a *PolicyAction) UnmarshalJSON(data []byte) error {
	val, err := proto.UnmarshalJSONEnum(PolicyAction_value, data, a.String())
	if err != nil {
		return err
	}

	_, ok := PolicyAction_name[val]
	if !ok {
		return errors.Errorf("invalid PolicyAction value: %d", val)
	}
	*a = PolicyAction(val)
	return nil
}

func (a AttrMatch) MarshalJSON() ([]byte, error) {
	return proto.MarshalJSONEnum(AttrMatch_name, int32(a))
}

func (a *AttrMatch) UnmarshalJSON(data []byte) error {
	val, err := proto.UnmarshalJSONEnum(AttrMatch_value, data, a.String())
	if err != nil {
		return err
	}

	_, ok := AttrMatch_name[val]
	if !ok {
		return errors.Errorf("invalid AttrMatch value: %d", val)
	}
	*a = AttrMatch(val)
	return nil
}

func (a MatchType) MarshalJSON() ([]byte, error) {
	return proto.MarshalJSONEnum(MatchType_name, int32(a))
}

func (a *MatchType) UnmarshalJSON(data []byte) error {
	val, err := proto.UnmarshalJSONEnum(MatchType_value, data, a.String())
	if err != nil {
		return err
	}

	_, ok := AttrMatch_name[val]
	if !ok {
		return errors.Errorf("invalid MatchType value: %d", val)
	}
	*a = MatchType(val)
	return nil
}
