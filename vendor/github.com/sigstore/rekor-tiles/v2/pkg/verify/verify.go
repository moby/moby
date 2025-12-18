//
// Copyright 2025 The Sigstore Authors.
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

package verify

import (
	"fmt"

	pbs "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/rekor-tiles/v2/internal/safeint"
	f_log "github.com/transparency-dev/formats/log"
	"github.com/transparency-dev/merkle/proof"
	"github.com/transparency-dev/merkle/rfc6962"
	sumdb_note "golang.org/x/mod/sumdb/note"
)

// VerifyInclusionProof verifies an entry's inclusion proof
func VerifyInclusionProof(entry *pbs.TransparencyLogEntry, cp *f_log.Checkpoint) error { //nolint: revive
	leafHash := rfc6962.DefaultHasher.HashLeaf(entry.CanonicalizedBody)
	index, err := safeint.NewSafeInt64(entry.LogIndex)
	if err != nil {
		return fmt.Errorf("invalid index: %w", err)
	}
	if err := proof.VerifyInclusion(rfc6962.DefaultHasher, index.U(), cp.Size, leafHash, entry.InclusionProof.Hashes, cp.Hash); err != nil {
		return fmt.Errorf("verifying inclusion: %w", err)
	}
	return nil
}

// VerifyCheckpoint verifies the signature on the entry's inclusion proof checkpoint
func VerifyCheckpoint(unverifiedCp string, verifier sumdb_note.Verifier) (*f_log.Checkpoint, error) { //nolint: revive
	cp, _, _, err := f_log.ParseCheckpoint([]byte(unverifiedCp), verifier.Name(), verifier)
	if err != nil {
		return nil, fmt.Errorf("unverified checkpoint signature: %v", err)
	}
	return cp, nil
}

// VerifyWitnessedCheckpoint verifies the signature on the entry's inclusion proof checkpoint in addition to witness cosignatures.
// This returns the underlying note which contains all verified signatures.
func VerifyWitnessedCheckpoint(unverifiedCp string, verifier sumdb_note.Verifier, otherVerifiers ...sumdb_note.Verifier) (*f_log.Checkpoint, *sumdb_note.Note, error) { //nolint: revive
	cp, _, n, err := f_log.ParseCheckpoint([]byte(unverifiedCp), verifier.Name(), verifier, otherVerifiers...)
	if err != nil {
		return nil, nil, fmt.Errorf("unverified checkpoint signature: %v", err)
	}
	return cp, n, nil
}

// VerifyLogEntry verifies the log entry. This includes verifying the signature on the entry's
// inclusion proof checkpoint and verifying the entry inclusion proof
func VerifyLogEntry(entry *pbs.TransparencyLogEntry, verifier sumdb_note.Verifier) error { //nolint: revive
	cp, err := VerifyCheckpoint(entry.GetInclusionProof().GetCheckpoint().GetEnvelope(), verifier)
	if err != nil {
		return err
	}
	return VerifyInclusionProof(entry, cp)
}

// VerifyConsistencyProof verifies the latest checkpoint signature and the consistency proof between a previous log size
// and root hash and the latest checkpoint's size and root hash. This may be used by a C2SP witness.
func VerifyConsistencyProof(consistencyProof [][]byte, oldSize uint64, oldRootHash []byte, newUnverifiedCp string, verifier sumdb_note.Verifier) error { //nolint: revive
	newCp, err := VerifyCheckpoint(newUnverifiedCp, verifier)
	if err != nil {
		return err
	}
	return proof.VerifyConsistency(rfc6962.DefaultHasher, oldSize, newCp.Size, consistencyProof, oldRootHash, newCp.Hash)
}

// VerifyConsistencyProofWithCheckpoints verifies previous and latest checkpoint signatures and the consistency proof
// between these checkpoints. This may be used by a monitor that persists checkpoints.
func VerifyConsistencyProofWithCheckpoints(consistencyProof [][]byte, oldUnverifiedCp, newUnverifiedCp string, verifier sumdb_note.Verifier) error { //nolint: revive
	oldCp, err := VerifyCheckpoint(oldUnverifiedCp, verifier)
	if err != nil {
		return err
	}
	return VerifyConsistencyProof(consistencyProof, oldCp.Size, oldCp.Hash, newUnverifiedCp, verifier)
}
