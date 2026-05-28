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
	"encoding/json"
	"time"
)

// Generic type constraint
type Roles interface {
	RootType | SnapshotType | TimestampType | TargetsType
}

// Define version of the TUF specification
const (
	SPECIFICATION_VERSION = "1.0.31"
)

// Define top level role names
const (
	ROOT      = "root"
	SNAPSHOT  = "snapshot"
	TARGETS   = "targets"
	TIMESTAMP = "timestamp"
)

var TOP_LEVEL_ROLE_NAMES = [...]string{ROOT, TIMESTAMP, SNAPSHOT, TARGETS}

// Metadata[T Roles] represents a TUF metadata.
// Provides methods to read and write to and
// from file and bytes, also to create, verify and clear metadata signatures.
type Metadata[T Roles] struct {
	Signed             T              `json:"signed"`
	Signatures         []Signature    `json:"signatures"`
	UnrecognizedFields map[string]any `json:"-"`
}

// Signature represents the Signature part of a TUF metadata
type Signature struct {
	KeyID              string         `json:"keyid"`
	Signature          HexBytes       `json:"sig"`
	UnrecognizedFields map[string]any `json:"-"`
}

// RootType represents the Signed portion of a root metadata
type RootType struct {
	Type               string           `json:"_type"`
	SpecVersion        string           `json:"spec_version"`
	ConsistentSnapshot bool             `json:"consistent_snapshot"`
	Version            int64            `json:"version"`
	Expires            time.Time        `json:"expires"`
	Keys               map[string]*Key  `json:"keys"`
	Roles              map[string]*Role `json:"roles"`
	UnrecognizedFields map[string]any   `json:"-"`
}

// SnapshotType represents the Signed portion of a snapshot metadata
type SnapshotType struct {
	Type               string                `json:"_type"`
	SpecVersion        string                `json:"spec_version"`
	Version            int64                 `json:"version"`
	Expires            time.Time             `json:"expires"`
	Meta               map[string]*MetaFiles `json:"meta"`
	UnrecognizedFields map[string]any        `json:"-"`
}

// TargetsType represents the Signed portion of a targets metadata
type TargetsType struct {
	Type               string                  `json:"_type"`
	SpecVersion        string                  `json:"spec_version"`
	Version            int64                   `json:"version"`
	Expires            time.Time               `json:"expires"`
	Targets            map[string]*TargetFiles `json:"targets"`
	Delegations        *Delegations            `json:"delegations,omitempty"`
	UnrecognizedFields map[string]any          `json:"-"`
}

// TimestampType represents the Signed portion of a timestamp metadata
type TimestampType struct {
	Type               string                `json:"_type"`
	SpecVersion        string                `json:"spec_version"`
	Version            int64                 `json:"version"`
	Expires            time.Time             `json:"expires"`
	Meta               map[string]*MetaFiles `json:"meta"`
	UnrecognizedFields map[string]any        `json:"-"`
}

// Key represents a key in TUF
type Key struct {
	Type               string         `json:"keytype"`
	Scheme             string         `json:"scheme"`
	Value              KeyVal         `json:"keyval"`
	id                 string         `json:"-"`
	UnrecognizedFields map[string]any `json:"-"`
}

type KeyVal struct {
	PublicKey          string         `json:"public"`
	UnrecognizedFields map[string]any `json:"-"`
}

// Role represents one of the top-level roles in TUF
type Role struct {
	KeyIDs             []string       `json:"keyids"`
	Threshold          int            `json:"threshold"`
	UnrecognizedFields map[string]any `json:"-"`
}

type HexBytes []byte

type Hashes map[string]HexBytes

// MetaFiles represents the value portion of METAFILES in TUF (used in Snapshot and Timestamp metadata). Used to store information about a particular meta file.
type MetaFiles struct {
	Length             int64          `json:"length,omitempty"`
	Hashes             Hashes         `json:"hashes,omitempty"`
	Version            int64          `json:"version"`
	UnrecognizedFields map[string]any `json:"-"`
}

// TargetFiles represents the value portion of TARGETS in TUF (used Targets metadata). Used to store information about a particular target file.
type TargetFiles struct {
	Length             int64            `json:"length"`
	Hashes             Hashes           `json:"hashes"`
	Custom             *json.RawMessage `json:"custom,omitempty"`
	Path               string           `json:"-"`
	UnrecognizedFields map[string]any   `json:"-"`
}

// Delegations is an optional object which represents delegation roles and their corresponding keys
type Delegations struct {
	Keys               map[string]*Key `json:"keys"`
	Roles              []DelegatedRole `json:"roles,omitempty"`
	SuccinctRoles      *SuccinctRoles  `json:"succinct_roles,omitempty"`
	UnrecognizedFields map[string]any  `json:"-"`
}

// DelegatedRole represents a delegated role in TUF
type DelegatedRole struct {
	Name               string         `json:"name"`
	KeyIDs             []string       `json:"keyids"`
	Threshold          int            `json:"threshold"`
	Terminating        bool           `json:"terminating"`
	PathHashPrefixes   []string       `json:"path_hash_prefixes,omitempty"`
	Paths              []string       `json:"paths,omitempty"`
	UnrecognizedFields map[string]any `json:"-"`
}

// SuccinctRoles represents a delegation graph that covers all targets,
// distributing them uniformly over the delegated roles (i.e. bins) in the graph.
type SuccinctRoles struct {
	KeyIDs             []string       `json:"keyids"`
	Threshold          int            `json:"threshold"`
	BitLength          int            `json:"bit_length"`
	NamePrefix         string         `json:"name_prefix"`
	UnrecognizedFields map[string]any `json:"-"`
}

// RoleResult represents the name and terminating status of a delegated role that is responsible for targetFilepath
type RoleResult struct {
	Name        string
	Terminating bool
}
