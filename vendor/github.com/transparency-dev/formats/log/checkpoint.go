// Copyright 2021 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package log provides basic support for the common log checkpoint and proof
// format described by the README in this directory.
package log

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
)

// Checkpoint represents a minimal log checkpoint (STH).
type Checkpoint struct {
	// Origin is the string identifying the log which issued this checkpoint.
	Origin string
	// Size is the number of entries in the log at this checkpoint.
	Size uint64
	// Hash is the hash which commits to the contents of the entire log.
	Hash []byte
}

// Marshal returns the common format representation of this Checkpoint.
func (c Checkpoint) Marshal() []byte {
	return []byte(fmt.Sprintf("%s\n%d\n%s\n", c.Origin, c.Size, base64.StdEncoding.EncodeToString(c.Hash)))
}

// Unmarshal parses the common formatted checkpoint data and stores the result
// in the Checkpoint.
//
// The supplied data is expected to begin with the following 3 lines of text,
// each followed by a newline:
//   - <origin string>
//   - <decimal representation of log size>
//   - <base64 representation of root hash>
//
// Any trailing data after this will be returned.
func (c *Checkpoint) Unmarshal(data []byte) ([]byte, error) {
	l := bytes.SplitN(data, []byte("\n"), 4)
	if len(l) < 4 {
		return nil, errors.New("invalid checkpoint - too few newlines")
	}
	origin := string(l[0])
	if len(origin) == 0 {
		return nil, errors.New("invalid checkpoint - empty origin")
	}
	size, err := strconv.ParseUint(string(l[1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid checkpoint - size invalid: %w", err)
	}
	h, err := base64.StdEncoding.DecodeString(string(l[2]))
	if err != nil {
		return nil, fmt.Errorf("invalid checkpoint - invalid hash: %w", err)
	}
	var rest []byte
	if len(l[3]) > 0 {
		rest = l[3]
	}
	*c = Checkpoint{
		Origin: origin,
		Size:   size,
		Hash:   h,
	}
	return rest, nil
}
