//
// Copyright 2021 The Sigstore Authors.
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

package util

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// heavily borrowed from https://github.com/transparency-dev/formats/blob/main/log/checkpoint.go

type Checkpoint struct {
	// Origin is the unique identifier/version string
	Origin string
	// Size is the number of entries in the log at this checkpoint.
	Size uint64
	// Hash is the hash which commits to the contents of the entire log.
	Hash []byte
	// OtherContent is any additional data to be included in the signed payload; each element is assumed to be one line
	OtherContent []string
}

// String returns the String representation of the Checkpoint
func (c Checkpoint) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%d\n%s\n", c.Origin, c.Size, base64.StdEncoding.EncodeToString(c.Hash))
	for _, line := range c.OtherContent {
		fmt.Fprintf(&b, "%s\n", line)
	}
	return b.String()
}

// MarshalCheckpoint returns the common format representation of this Checkpoint.
func (c Checkpoint) MarshalCheckpoint() ([]byte, error) {
	return []byte(c.String()), nil
}

// UnmarshalCheckpoint parses the common formatted checkpoint data and stores the result
// in the Checkpoint.
//
// The supplied data is expected to begin with the following 3 lines of text,
// each followed by a newline:
// <ecosystem/version string>
// <decimal representation of log size>
// <base64 representation of root hash>
// <optional non-empty line of other content>...
// <optional non-empty line of other content>...
//
// This will discard any content found after the checkpoint (including signatures)
func (c *Checkpoint) UnmarshalCheckpoint(data []byte) error {
	l := bytes.Split(data, []byte("\n"))
	if len(l) < 4 {
		return errors.New("invalid checkpoint - too few newlines")
	}
	origin := string(l[0])
	if len(origin) == 0 {
		return errors.New("invalid checkpoint - empty ecosystem")
	}
	size, err := strconv.ParseUint(string(l[1]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid checkpoint - size invalid: %w", err)
	}
	h, err := base64.StdEncoding.DecodeString(string(l[2]))
	if err != nil {
		return fmt.Errorf("invalid checkpoint - invalid hash: %w", err)
	}
	*c = Checkpoint{
		Origin: origin,
		Size:   size,
		Hash:   h,
	}
	if len(l) >= 3 {
		for _, line := range l[3:] {
			if len(line) == 0 {
				break
			}
			c.OtherContent = append(c.OtherContent, string(line))
		}
	}
	return nil
}

type SignedCheckpoint struct {
	Checkpoint
	SignedNote
}

func CreateSignedCheckpoint(c Checkpoint) (*SignedCheckpoint, error) {
	text, err := c.MarshalCheckpoint()
	if err != nil {
		return nil, err
	}
	return &SignedCheckpoint{
		Checkpoint: c,
		SignedNote: SignedNote{Note: string(text)},
	}, nil
}

func SignedCheckpointValidator(strToValidate string) bool {
	s := SignedNote{}
	if err := s.UnmarshalText([]byte(strToValidate)); err != nil {
		return false
	}
	c := &Checkpoint{}
	return c.UnmarshalCheckpoint([]byte(s.Note)) == nil
}

func CheckpointValidator(strToValidate string) bool {
	c := &Checkpoint{}
	return c.UnmarshalCheckpoint([]byte(strToValidate)) == nil
}

func (r *SignedCheckpoint) UnmarshalText(data []byte) error {
	s := SignedNote{}
	if err := s.UnmarshalText([]byte(data)); err != nil {
		return fmt.Errorf("unmarshalling signed note: %w", err)
	}
	c := Checkpoint{}
	if err := c.UnmarshalCheckpoint([]byte(s.Note)); err != nil {
		return fmt.Errorf("unmarshalling checkpoint: %w", err)
	}
	*r = SignedCheckpoint{Checkpoint: c, SignedNote: s}
	return nil
}

// CreateAndSignCheckpoint creates a signed checkpoint as a commitment to the current root hash
func CreateAndSignCheckpoint(ctx context.Context, hostname string, treeID int64, treeSize uint64, rootHash []byte, signer signature.Signer) ([]byte, error) {
	sth, err := CreateSignedCheckpoint(Checkpoint{
		Origin: fmt.Sprintf("%s - %d", hostname, treeID),
		Size:   treeSize,
		Hash:   rootHash,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating checkpoint: %w", err)
	}
	if _, err := sth.Sign(hostname, signer, options.WithContext(ctx)); err != nil {
		return nil, fmt.Errorf("error signing checkpoint: %w", err)
	}
	scBytes, err := sth.MarshalText()
	if err != nil {
		return nil, fmt.Errorf("error marshalling checkpoint: %w", err)
	}
	return scBytes, nil
}
