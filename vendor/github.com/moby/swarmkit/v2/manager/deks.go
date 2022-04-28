package manager

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"github.com/moby/swarmkit/v2/ca"
	"github.com/moby/swarmkit/v2/manager/encryption"
	"github.com/moby/swarmkit/v2/manager/state/raft"
)

// This module contains the data structures and control flow to manage rotating the raft
// DEK and also for interacting with KeyReadWriter to maintain the raft DEK information in
// the PEM headers fo the TLS key for the node.

const (
	// the raft DEK (data encryption key) is stored in the TLS key as a header
	// these are the header values
	pemHeaderRaftDEK              = "raft-dek"
	pemHeaderRaftPendingDEK       = "raft-dek-pending"
	pemHeaderRaftDEKNeedsRotation = "raft-dek-needs-rotation"
)

// RaftDEKData contains all the data stored in TLS pem headers.
type RaftDEKData struct {

	// EncryptionKeys contain the current and pending raft DEKs
	raft.EncryptionKeys

	// NeedsRotation indicates whether another rotation needs to be happen after
	// the current one.
	NeedsRotation bool

	// The FIPS boolean is not serialized, but is internal state which indicates how
	// the raft DEK headers should be encrypted (e.g. using FIPS compliant algorithms)
	FIPS bool
}

// RaftDEKData should implement the PEMKeyHeaders interface
var _ ca.PEMKeyHeaders = RaftDEKData{}

// UnmarshalHeaders loads the current state of the DEKs into a new RaftDEKData object (which is returned) given the
// current TLS headers and the current KEK.
func (r RaftDEKData) UnmarshalHeaders(headers map[string]string, kekData ca.KEKData) (ca.PEMKeyHeaders, error) {
	var (
		currentDEK, pendingDEK []byte
		err                    error
	)

	if currentDEKStr, ok := headers[pemHeaderRaftDEK]; ok {
		currentDEK, err = decodePEMHeaderValue(currentDEKStr, kekData.KEK, r.FIPS)
		if err != nil {
			return nil, err
		}
	}
	if pendingDEKStr, ok := headers[pemHeaderRaftPendingDEK]; ok {
		pendingDEK, err = decodePEMHeaderValue(pendingDEKStr, kekData.KEK, r.FIPS)
		if err != nil {
			return nil, err
		}
	}

	if pendingDEK != nil && currentDEK == nil {
		return nil, fmt.Errorf("there is a pending DEK, but no current DEK")
	}

	_, ok := headers[pemHeaderRaftDEKNeedsRotation]
	return RaftDEKData{
		NeedsRotation: ok,
		EncryptionKeys: raft.EncryptionKeys{
			CurrentDEK: currentDEK,
			PendingDEK: pendingDEK,
		},
		FIPS: r.FIPS,
	}, nil
}

// MarshalHeaders returns new PEM headers given the current KEK - it uses the current KEK to
// serialize/encrypt the current DEK state that is maintained in the current RaftDEKData object.
func (r RaftDEKData) MarshalHeaders(kekData ca.KEKData) (map[string]string, error) {
	headers := make(map[string]string)
	for headerKey, contents := range map[string][]byte{
		pemHeaderRaftDEK:        r.CurrentDEK,
		pemHeaderRaftPendingDEK: r.PendingDEK,
	} {
		if contents != nil {
			dekStr, err := encodePEMHeaderValue(contents, kekData.KEK, r.FIPS)
			if err != nil {
				return nil, err
			}
			headers[headerKey] = dekStr
		}
	}

	if r.NeedsRotation {
		headers[pemHeaderRaftDEKNeedsRotation] = "true"
	}

	// return a function that updates the dek data on write success
	return headers, nil
}

// UpdateKEK sets NeedRotation to true if we go from unlocked to locked.
func (r RaftDEKData) UpdateKEK(oldKEK, candidateKEK ca.KEKData) ca.PEMKeyHeaders {
	if _, unlockedToLocked, err := compareKEKs(oldKEK, candidateKEK); err == nil && unlockedToLocked {
		return RaftDEKData{
			EncryptionKeys: r.EncryptionKeys,
			NeedsRotation:  true,
			FIPS:           r.FIPS,
		}
	}
	return r
}

// Returns whether the old KEK should be replaced with the new KEK, whether we went from
// unlocked to locked, and whether there was an error (the versions are the same, but the
// keks are different)
func compareKEKs(oldKEK, candidateKEK ca.KEKData) (bool, bool, error) {
	keksEqual := subtle.ConstantTimeCompare(oldKEK.KEK, candidateKEK.KEK) == 1
	switch {
	case oldKEK.Version == candidateKEK.Version && !keksEqual:
		return false, false, fmt.Errorf("candidate KEK has the same version as the current KEK, but a different KEK value")
	case oldKEK.Version >= candidateKEK.Version || keksEqual:
		return false, false, nil
	default:
		return true, oldKEK.KEK == nil, nil
	}
}

// RaftDEKManager manages the raft DEK keys by interacting with KeyReadWriter, calling the necessary functions
// to update the TLS headers when the raft DEK needs to change, or to re-encrypt everything when the KEK changes.
type RaftDEKManager struct {
	kw         ca.KeyWriter
	rotationCh chan struct{}
	FIPS       bool
}

var errNoUpdateNeeded = fmt.Errorf("don't need to rotate or update")

// this error is returned if the KeyReadWriter's PEMKeyHeaders object is no longer a RaftDEKData object -
// this can happen if the node is no longer a manager, for example
var errNotUsingRaftDEKData = fmt.Errorf("RaftDEKManager can no longer store and manage TLS key headers")

// NewRaftDEKManager returns a RaftDEKManager that uses the current key writer
// and header manager
func NewRaftDEKManager(kw ca.KeyWriter, fips bool) (*RaftDEKManager, error) {
	// If there is no current DEK, generate one and write it to disk
	err := kw.ViewAndUpdateHeaders(func(h ca.PEMKeyHeaders) (ca.PEMKeyHeaders, error) {
		dekData, ok := h.(RaftDEKData)
		// it wasn't a raft DEK manager before - just replace it
		if !ok || dekData.CurrentDEK == nil {
			return RaftDEKData{
				EncryptionKeys: raft.EncryptionKeys{
					CurrentDEK: encryption.GenerateSecretKey(),
				},
				FIPS: fips,
			}, nil
		}
		return nil, errNoUpdateNeeded
	})
	if err != nil && err != errNoUpdateNeeded {
		return nil, err
	}
	return &RaftDEKManager{
		kw:         kw,
		FIPS:       fips,
		rotationCh: make(chan struct{}, 1),
	}, nil
}

// NeedsRotation returns a boolean about whether we should do a rotation
func (r *RaftDEKManager) NeedsRotation() bool {
	h, _ := r.kw.GetCurrentState()
	data, ok := h.(RaftDEKData)
	if !ok {
		return false
	}
	return data.NeedsRotation || data.EncryptionKeys.PendingDEK != nil
}

// GetKeys returns the current set of DEKs.  If NeedsRotation is true, and there
// is no existing PendingDEK, it will try to create one.  If it successfully creates
// and writes a PendingDEK, it sets NeedRotation to false.  If there are any errors
// doing so, just return the original set of keys.
func (r *RaftDEKManager) GetKeys() raft.EncryptionKeys {
	var newKeys, originalKeys raft.EncryptionKeys
	err := r.kw.ViewAndUpdateHeaders(func(h ca.PEMKeyHeaders) (ca.PEMKeyHeaders, error) {
		data, ok := h.(RaftDEKData)
		if !ok {
			return nil, errNotUsingRaftDEKData
		}
		originalKeys = data.EncryptionKeys
		if !data.NeedsRotation || data.PendingDEK != nil {
			return nil, errNoUpdateNeeded
		}
		newKeys = raft.EncryptionKeys{
			CurrentDEK: data.CurrentDEK,
			PendingDEK: encryption.GenerateSecretKey(),
		}
		return RaftDEKData{
			EncryptionKeys: newKeys,
			FIPS:           data.FIPS,
		}, nil
	})
	if err != nil {
		return originalKeys
	}
	return newKeys
}

// RotationNotify the channel used to notify subscribers as to whether there
// should be a rotation done
func (r *RaftDEKManager) RotationNotify() chan struct{} {
	return r.rotationCh
}

// UpdateKeys will set the updated encryption keys in the headers.  This finishes
// a rotation, and is expected to set the CurrentDEK to the previous PendingDEK.
func (r *RaftDEKManager) UpdateKeys(newKeys raft.EncryptionKeys) error {
	return r.kw.ViewAndUpdateHeaders(func(h ca.PEMKeyHeaders) (ca.PEMKeyHeaders, error) {
		data, ok := h.(RaftDEKData)
		if !ok {
			return nil, errNotUsingRaftDEKData
		}
		// If there is no current DEK, we are basically wiping out all DEKs (no header object)
		if newKeys.CurrentDEK == nil {
			return nil, nil
		}
		return RaftDEKData{
			EncryptionKeys: newKeys,
			NeedsRotation:  data.NeedsRotation,
			FIPS:           data.FIPS,
		}, nil
	})
}

// MaybeUpdateKEK does a KEK rotation if one is required.  Returns whether
// the kek was updated, whether it went from unlocked to locked, and any errors.
func (r *RaftDEKManager) MaybeUpdateKEK(candidateKEK ca.KEKData) (bool, bool, error) {
	var updated, unlockedToLocked bool
	err := r.kw.ViewAndRotateKEK(func(currentKEK ca.KEKData, h ca.PEMKeyHeaders) (ca.KEKData, ca.PEMKeyHeaders, error) {
		var err error
		updated, unlockedToLocked, err = compareKEKs(currentKEK, candidateKEK)
		if err == nil && !updated { // if we don't need to rotate the KEK, don't bother updating
			err = errNoUpdateNeeded
		}
		if err != nil {
			return ca.KEKData{}, nil, err
		}

		data, ok := h.(RaftDEKData)
		if !ok {
			return ca.KEKData{}, nil, errNotUsingRaftDEKData
		}

		if unlockedToLocked {
			data.NeedsRotation = true
		}
		return candidateKEK, data, nil
	})
	if err == errNoUpdateNeeded {
		err = nil
	}

	if err == nil && unlockedToLocked {
		r.rotationCh <- struct{}{}
	}
	return updated, unlockedToLocked, err
}

func decodePEMHeaderValue(headerValue string, kek []byte, fips bool) ([]byte, error) {
	var decrypter encryption.Decrypter = encryption.NoopCrypter
	if kek != nil {
		_, decrypter = encryption.Defaults(kek, fips)
	}
	valueBytes, err := base64.StdEncoding.DecodeString(headerValue)
	if err != nil {
		return nil, err
	}
	result, err := encryption.Decrypt(valueBytes, decrypter)
	if err != nil {
		return nil, ca.ErrInvalidKEK{Wrapped: err}
	}
	return result, nil
}

func encodePEMHeaderValue(headerValue []byte, kek []byte, fips bool) (string, error) {
	var encrypter encryption.Encrypter = encryption.NoopCrypter
	if kek != nil {
		encrypter, _ = encryption.Defaults(kek, fips)
	}
	encrypted, err := encryption.Encrypt(headerValue, encrypter)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}
