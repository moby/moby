// Copyright 2024 The Update Framework Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License
//
// SPDX-License-Identifier: Apache-2.0
//

package metadata

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// The following marshal/unmarshal methods override the default behavior for for each TUF type
// in order to support unrecognized fields

func (signed RootType) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(signed.UnrecognizedFields) != 0 {
		copyMapValues(signed.UnrecognizedFields, dict)
	}
	dict["_type"] = signed.Type
	dict["spec_version"] = signed.SpecVersion
	dict["consistent_snapshot"] = signed.ConsistentSnapshot
	dict["version"] = signed.Version
	dict["expires"] = signed.Expires
	dict["keys"] = signed.Keys
	dict["roles"] = signed.Roles
	return json.Marshal(dict)
}

func (signed *RootType) UnmarshalJSON(data []byte) error {
	type Alias RootType
	var s Alias
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*signed = RootType(s)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "_type")
	delete(dict, "spec_version")
	delete(dict, "consistent_snapshot")
	delete(dict, "version")
	delete(dict, "expires")
	delete(dict, "keys")
	delete(dict, "roles")
	signed.UnrecognizedFields = dict
	return nil
}

func (signed SnapshotType) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(signed.UnrecognizedFields) != 0 {
		copyMapValues(signed.UnrecognizedFields, dict)
	}
	dict["_type"] = signed.Type
	dict["spec_version"] = signed.SpecVersion
	dict["version"] = signed.Version
	dict["expires"] = signed.Expires
	dict["meta"] = signed.Meta
	return json.Marshal(dict)
}

func (signed *SnapshotType) UnmarshalJSON(data []byte) error {
	type Alias SnapshotType
	var s Alias
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*signed = SnapshotType(s)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "_type")
	delete(dict, "spec_version")
	delete(dict, "version")
	delete(dict, "expires")
	delete(dict, "meta")
	signed.UnrecognizedFields = dict
	return nil
}

func (signed TimestampType) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(signed.UnrecognizedFields) != 0 {
		copyMapValues(signed.UnrecognizedFields, dict)
	}
	dict["_type"] = signed.Type
	dict["spec_version"] = signed.SpecVersion
	dict["version"] = signed.Version
	dict["expires"] = signed.Expires
	dict["meta"] = signed.Meta
	return json.Marshal(dict)
}

func (signed *TimestampType) UnmarshalJSON(data []byte) error {
	type Alias TimestampType
	var s Alias
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*signed = TimestampType(s)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "_type")
	delete(dict, "spec_version")
	delete(dict, "version")
	delete(dict, "expires")
	delete(dict, "meta")
	signed.UnrecognizedFields = dict
	return nil
}

func (signed TargetsType) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(signed.UnrecognizedFields) != 0 {
		copyMapValues(signed.UnrecognizedFields, dict)
	}
	dict["_type"] = signed.Type
	dict["spec_version"] = signed.SpecVersion
	dict["version"] = signed.Version
	dict["expires"] = signed.Expires
	dict["targets"] = signed.Targets
	if signed.Delegations != nil {
		dict["delegations"] = signed.Delegations
	}
	return json.Marshal(dict)
}

func (signed *TargetsType) UnmarshalJSON(data []byte) error {
	type Alias TargetsType
	var s Alias
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*signed = TargetsType(s)

	// populate the path field for each target
	for name, targetFile := range signed.Targets {
		targetFile.Path = name
	}

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "_type")
	delete(dict, "spec_version")
	delete(dict, "version")
	delete(dict, "expires")
	delete(dict, "targets")
	delete(dict, "delegations")
	signed.UnrecognizedFields = dict
	return nil
}

func (signed MetaFiles) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(signed.UnrecognizedFields) != 0 {
		copyMapValues(signed.UnrecognizedFields, dict)
	}
	// length and hashes are optional
	if signed.Length != 0 {
		dict["length"] = signed.Length
	}
	if len(signed.Hashes) != 0 {
		dict["hashes"] = signed.Hashes
	}
	dict["version"] = signed.Version
	return json.Marshal(dict)
}

func (signed *MetaFiles) UnmarshalJSON(data []byte) error {
	type Alias MetaFiles
	var s Alias
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*signed = MetaFiles(s)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "length")
	delete(dict, "hashes")
	delete(dict, "version")
	signed.UnrecognizedFields = dict
	return nil
}

func (signed TargetFiles) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(signed.UnrecognizedFields) != 0 {
		copyMapValues(signed.UnrecognizedFields, dict)
	}
	dict["length"] = signed.Length
	dict["hashes"] = signed.Hashes
	if signed.Custom != nil {
		dict["custom"] = signed.Custom
	}
	return json.Marshal(dict)
}

func (signed *TargetFiles) UnmarshalJSON(data []byte) error {
	type Alias TargetFiles
	var s Alias
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*signed = TargetFiles(s)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "length")
	delete(dict, "hashes")
	delete(dict, "custom")
	signed.UnrecognizedFields = dict
	return nil
}

func (key Key) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(key.UnrecognizedFields) != 0 {
		copyMapValues(key.UnrecognizedFields, dict)
	}
	dict["keytype"] = key.Type
	dict["scheme"] = key.Scheme
	dict["keyval"] = key.Value
	return json.Marshal(dict)
}

func (key *Key) UnmarshalJSON(data []byte) error {
	type Alias Key
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	// nolint
	*key = Key(a)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "keytype")
	delete(dict, "scheme")
	delete(dict, "keyval")
	key.UnrecognizedFields = dict
	return nil
}

func (meta Metadata[T]) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(meta.UnrecognizedFields) != 0 {
		copyMapValues(meta.UnrecognizedFields, dict)
	}
	dict["signed"] = meta.Signed
	dict["signatures"] = meta.Signatures
	return json.Marshal(dict)
}

func (meta *Metadata[T]) UnmarshalJSON(data []byte) error {
	tmp := any(new(T))
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	switch tmp.(type) {
	case *RootType:
		dict := struct {
			Signed     RootType    `json:"signed"`
			Signatures []Signature `json:"signatures"`
		}{}
		if err := json.Unmarshal(data, &dict); err != nil {
			return err
		}
		var i interface{} = dict.Signed
		meta.Signed = i.(T)
		meta.Signatures = dict.Signatures
	case *SnapshotType:
		dict := struct {
			Signed     SnapshotType `json:"signed"`
			Signatures []Signature  `json:"signatures"`
		}{}
		if err := json.Unmarshal(data, &dict); err != nil {
			return err
		}
		var i interface{} = dict.Signed
		meta.Signed = i.(T)
		meta.Signatures = dict.Signatures
	case *TimestampType:
		dict := struct {
			Signed     TimestampType `json:"signed"`
			Signatures []Signature   `json:"signatures"`
		}{}
		if err := json.Unmarshal(data, &dict); err != nil {
			return err
		}
		var i interface{} = dict.Signed
		meta.Signed = i.(T)
		meta.Signatures = dict.Signatures
	case *TargetsType:
		dict := struct {
			Signed     TargetsType `json:"signed"`
			Signatures []Signature `json:"signatures"`
		}{}
		if err := json.Unmarshal(data, &dict); err != nil {
			return err
		}
		var i interface{} = dict.Signed
		meta.Signed = i.(T)
		meta.Signatures = dict.Signatures
	default:
		return &ErrValue{Msg: "unrecognized metadata type"}
	}
	delete(m, "signed")
	delete(m, "signatures")
	meta.UnrecognizedFields = m
	return nil
}

func (s Signature) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(s.UnrecognizedFields) != 0 {
		copyMapValues(s.UnrecognizedFields, dict)
	}
	dict["keyid"] = s.KeyID
	dict["sig"] = s.Signature
	return json.Marshal(dict)
}

func (s *Signature) UnmarshalJSON(data []byte) error {
	type Alias Signature
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = Signature(a)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "keyid")
	delete(dict, "sig")
	s.UnrecognizedFields = dict
	return nil
}

func (kv KeyVal) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(kv.UnrecognizedFields) != 0 {
		copyMapValues(kv.UnrecognizedFields, dict)
	}
	dict["public"] = kv.PublicKey
	return json.Marshal(dict)
}

func (kv *KeyVal) UnmarshalJSON(data []byte) error {
	type Alias KeyVal
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*kv = KeyVal(a)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "public")
	kv.UnrecognizedFields = dict
	return nil
}

func (role Role) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(role.UnrecognizedFields) != 0 {
		copyMapValues(role.UnrecognizedFields, dict)
	}
	dict["keyids"] = role.KeyIDs
	dict["threshold"] = role.Threshold
	return json.Marshal(dict)
}

func (role *Role) UnmarshalJSON(data []byte) error {
	type Alias Role
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*role = Role(a)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "keyids")
	delete(dict, "threshold")
	role.UnrecognizedFields = dict
	return nil
}

func (d Delegations) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(d.UnrecognizedFields) != 0 {
		copyMapValues(d.UnrecognizedFields, dict)
	}
	// only one is allowed
	dict["keys"] = d.Keys
	if d.Roles != nil {
		dict["roles"] = d.Roles
	} else if d.SuccinctRoles != nil {
		dict["succinct_roles"] = d.SuccinctRoles
	}
	return json.Marshal(dict)
}

func (d *Delegations) UnmarshalJSON(data []byte) error {
	type Alias Delegations
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*d = Delegations(a)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "keys")
	delete(dict, "roles")
	delete(dict, "succinct_roles")
	d.UnrecognizedFields = dict
	return nil
}

func (role DelegatedRole) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(role.UnrecognizedFields) != 0 {
		copyMapValues(role.UnrecognizedFields, dict)
	}
	dict["name"] = role.Name
	dict["keyids"] = role.KeyIDs
	dict["threshold"] = role.Threshold
	dict["terminating"] = role.Terminating
	// make sure we have only one of the two (per spec)
	if role.Paths != nil && role.PathHashPrefixes != nil {
		return nil, &ErrValue{Msg: "failed to marshal: not allowed to have both \"paths\" and \"path_hash_prefixes\" present"}
	}
	if role.Paths != nil {
		dict["paths"] = role.Paths
	} else if role.PathHashPrefixes != nil {
		dict["path_hash_prefixes"] = role.PathHashPrefixes
	}
	return json.Marshal(dict)
}

func (role *DelegatedRole) UnmarshalJSON(data []byte) error {
	type Alias DelegatedRole
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*role = DelegatedRole(a)

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "name")
	delete(dict, "keyids")
	delete(dict, "threshold")
	delete(dict, "terminating")
	delete(dict, "paths")
	delete(dict, "path_hash_prefixes")
	role.UnrecognizedFields = dict
	return nil
}

func (role SuccinctRoles) MarshalJSON() ([]byte, error) {
	dict := map[string]any{}
	if len(role.UnrecognizedFields) != 0 {
		copyMapValues(role.UnrecognizedFields, dict)
	}
	dict["keyids"] = role.KeyIDs
	dict["threshold"] = role.Threshold
	dict["bit_length"] = role.BitLength
	dict["name_prefix"] = role.NamePrefix
	return json.Marshal(dict)
}

func (role *SuccinctRoles) UnmarshalJSON(data []byte) error {
	type Alias SuccinctRoles
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*role = SuccinctRoles(a)

	// Validate BitLength: must be between 1 and 32 inclusive.
	// - BitLength determines the number of bins as 2^BitLength
	// - We use the leftmost BitLength bits of a SHA-256 hash (32 bits max from 4 bytes)
	// - BitLength < 1 would result in 0 or fractional bins
	// - BitLength > 32 would cause a negative shift value in GetRolesForTarget
	if role.BitLength < 1 || role.BitLength > 32 {
		return fmt.Errorf("invalid bit_length: %d, must be between 1 and 32", role.BitLength)
	}

	var dict map[string]any
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}
	delete(dict, "keyids")
	delete(dict, "threshold")
	delete(dict, "bit_length")
	delete(dict, "name_prefix")
	role.UnrecognizedFields = dict
	return nil
}

func (b *HexBytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || len(data)%2 != 0 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("tuf: invalid JSON hex bytes")
	}
	res := make([]byte, hex.DecodedLen(len(data)-2))
	_, err := hex.Decode(res, data[1:len(data)-1])
	if err != nil {
		return err
	}
	*b = res
	return nil
}

func (b HexBytes) MarshalJSON() ([]byte, error) {
	res := make([]byte, hex.EncodedLen(len(b))+2)
	res[0] = '"'
	res[len(res)-1] = '"'
	hex.Encode(res[1:], b)
	return res, nil
}

func (b HexBytes) String() string {
	return hex.EncodeToString(b)
}

// copyMapValues copies the values of the src map to dst
func copyMapValues(src, dst map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
