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

package log

import (
	"fmt"

	"golang.org/x/mod/sumdb/note"
)

// ParseCheckpoint takes a raw checkpoint as bytes and returns a parsed checkpoint
// and any otherData in the body, providing that:
// * a valid log signature is found; and
// * the checkpoint unmarshals correctly; and
// * the log origin is that expected.
// In all other cases, an empty checkpoint is returned. The underlying note is always
// returned where possible.
// The signatures on the note will include the log signature if no error is returned,
// plus any signatures from otherVerifiers that were found.
func ParseCheckpoint(chkpt []byte, origin string, logVerifier note.Verifier, otherVerifiers ...note.Verifier) (*Checkpoint, []byte, *note.Note, error) {
	vs := append(append(make([]note.Verifier, 0, len(otherVerifiers)+1), logVerifier), otherVerifiers...)
	verifiers := note.VerifierList(vs...)

	n, err := note.Open(chkpt, verifiers)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to verify signatures on checkpoint: %v", err)
	}

	for _, s := range n.Sigs {
		if s.Hash == logVerifier.KeyHash() && s.Name == logVerifier.Name() {
			// The log has signed this checkpoint. It is now safe to parse.
			cp := &Checkpoint{}
			var otherData []byte
			if otherData, err = cp.Unmarshal([]byte(n.Text)); err != nil {
				return nil, nil, n, fmt.Errorf("failed to unmarshal checkpoint: %v", err)
			}
			if cp.Origin != origin {
				return nil, nil, n, fmt.Errorf("got Origin %q but expected %q", cp.Origin, origin)
			}
			return cp, otherData, n, nil
		}
	}
	return nil, nil, n, fmt.Errorf("no log signature found on note")
}
