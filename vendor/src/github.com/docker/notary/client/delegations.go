package client

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	"github.com/docker/notary/client/changelist"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/store"
	"github.com/docker/notary/tuf/utils"
)

// AddDelegation creates changelist entries to add provided delegation public keys and paths.
// This method composes AddDelegationRoleAndKeys and AddDelegationPaths (each creates one changelist if called).
func (r *NotaryRepository) AddDelegation(name string, delegationKeys []data.PublicKey, paths []string) error {
	if len(delegationKeys) > 0 {
		err := r.AddDelegationRoleAndKeys(name, delegationKeys)
		if err != nil {
			return err
		}
	}
	if len(paths) > 0 {
		err := r.AddDelegationPaths(name, paths)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddDelegationRoleAndKeys creates a changelist entry to add provided delegation public keys.
// This method is the simplest way to create a new delegation, because the delegation must have at least
// one key upon creation to be valid since we will reject the changelist while validating the threshold.
func (r *NotaryRepository) AddDelegationRoleAndKeys(name string, delegationKeys []data.PublicKey) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Adding delegation "%s" with threshold %d, and %d keys\n`,
		name, notary.MinThreshold, len(delegationKeys))

	// Defaulting to threshold of 1, since we don't allow for larger thresholds at the moment.
	tdJSON, err := json.Marshal(&changelist.TufDelegation{
		NewThreshold: notary.MinThreshold,
		AddKeys:      data.KeyList(delegationKeys),
	})
	if err != nil {
		return err
	}

	template := newCreateDelegationChange(name, tdJSON)
	return addChange(cl, template, name)
}

// AddDelegationPaths creates a changelist entry to add provided paths to an existing delegation.
// This method cannot create a new delegation itself because the role must meet the key threshold upon creation.
func (r *NotaryRepository) AddDelegationPaths(name string, paths []string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Adding %s paths to delegation %s\n`, paths, name)

	tdJSON, err := json.Marshal(&changelist.TufDelegation{
		AddPaths: paths,
	})
	if err != nil {
		return err
	}

	template := newCreateDelegationChange(name, tdJSON)
	return addChange(cl, template, name)
}

// RemoveDelegationKeysAndPaths creates changelist entries to remove provided delegation key IDs and paths.
// This method composes RemoveDelegationPaths and RemoveDelegationKeys (each creates one changelist if called).
func (r *NotaryRepository) RemoveDelegationKeysAndPaths(name string, keyIDs, paths []string) error {
	if len(paths) > 0 {
		err := r.RemoveDelegationPaths(name, paths)
		if err != nil {
			return err
		}
	}
	if len(keyIDs) > 0 {
		err := r.RemoveDelegationKeys(name, keyIDs)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveDelegationRole creates a changelist to remove all paths and keys from a role, and delete the role in its entirety.
func (r *NotaryRepository) RemoveDelegationRole(name string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Removing delegation "%s"\n`, name)

	template := newDeleteDelegationChange(name, nil)
	return addChange(cl, template, name)
}

// RemoveDelegationPaths creates a changelist entry to remove provided paths from an existing delegation.
func (r *NotaryRepository) RemoveDelegationPaths(name string, paths []string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Removing %s paths from delegation "%s"\n`, paths, name)

	tdJSON, err := json.Marshal(&changelist.TufDelegation{
		RemovePaths: paths,
	})
	if err != nil {
		return err
	}

	template := newUpdateDelegationChange(name, tdJSON)
	return addChange(cl, template, name)
}

// RemoveDelegationKeys creates a changelist entry to remove provided keys from an existing delegation.
// When this changelist is applied, if the specified keys are the only keys left in the role,
// the role itself will be deleted in its entirety.
func (r *NotaryRepository) RemoveDelegationKeys(name string, keyIDs []string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Removing %s keys from delegation "%s"\n`, keyIDs, name)

	tdJSON, err := json.Marshal(&changelist.TufDelegation{
		RemoveKeys: keyIDs,
	})
	if err != nil {
		return err
	}

	template := newUpdateDelegationChange(name, tdJSON)
	return addChange(cl, template, name)
}

// ClearDelegationPaths creates a changelist entry to remove all paths from an existing delegation.
func (r *NotaryRepository) ClearDelegationPaths(name string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Removing all paths from delegation "%s"\n`, name)

	tdJSON, err := json.Marshal(&changelist.TufDelegation{
		ClearAllPaths: true,
	})
	if err != nil {
		return err
	}

	template := newUpdateDelegationChange(name, tdJSON)
	return addChange(cl, template, name)
}

func newUpdateDelegationChange(name string, content []byte) *changelist.TufChange {
	return changelist.NewTufChange(
		changelist.ActionUpdate,
		name,
		changelist.TypeTargetsDelegation,
		"", // no path for delegations
		content,
	)
}

func newCreateDelegationChange(name string, content []byte) *changelist.TufChange {
	return changelist.NewTufChange(
		changelist.ActionCreate,
		name,
		changelist.TypeTargetsDelegation,
		"", // no path for delegations
		content,
	)
}

func newDeleteDelegationChange(name string, content []byte) *changelist.TufChange {
	return changelist.NewTufChange(
		changelist.ActionDelete,
		name,
		changelist.TypeTargetsDelegation,
		"", // no path for delegations
		content,
	)
}

// GetDelegationRoles returns the keys and roles of the repository's delegations
// Also converts key IDs to canonical key IDs to keep consistent with signing prompts
func (r *NotaryRepository) GetDelegationRoles() ([]*data.Role, error) {
	// Update state of the repo to latest
	if _, err := r.Update(false); err != nil {
		return nil, err
	}

	// All top level delegations (ex: targets/level1) are stored exclusively in targets.json
	_, ok := r.tufRepo.Targets[data.CanonicalTargetsRole]
	if !ok {
		return nil, store.ErrMetaNotFound{Resource: data.CanonicalTargetsRole}
	}

	// make a copy for traversing nested delegations
	allDelegations := []*data.Role{}

	// Define a visitor function to populate the delegations list and translate their key IDs to canonical IDs
	delegationCanonicalListVisitor := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
		// For the return list, update with a copy that includes canonicalKeyIDs
		// These aren't validated by the validRole
		canonicalDelegations, err := translateDelegationsToCanonicalIDs(tgt.Signed.Delegations)
		if err != nil {
			return err
		}
		allDelegations = append(allDelegations, canonicalDelegations...)
		return nil
	}
	err := r.tufRepo.WalkTargets("", "", delegationCanonicalListVisitor)
	if err != nil {
		return nil, err
	}
	return allDelegations, nil
}

func translateDelegationsToCanonicalIDs(delegationInfo data.Delegations) ([]*data.Role, error) {
	canonicalDelegations := make([]*data.Role, len(delegationInfo.Roles))
	copy(canonicalDelegations, delegationInfo.Roles)
	delegationKeys := delegationInfo.Keys
	for i, delegation := range canonicalDelegations {
		canonicalKeyIDs := []string{}
		for _, keyID := range delegation.KeyIDs {
			pubKey, ok := delegationKeys[keyID]
			if !ok {
				return nil, fmt.Errorf("Could not translate canonical key IDs for %s", delegation.Name)
			}
			canonicalKeyID, err := utils.CanonicalKeyID(pubKey)
			if err != nil {
				return nil, fmt.Errorf("Could not translate canonical key IDs for %s: %v", delegation.Name, err)
			}
			canonicalKeyIDs = append(canonicalKeyIDs, canonicalKeyID)
		}
		canonicalDelegations[i].KeyIDs = canonicalKeyIDs
	}
	return canonicalDelegations, nil
}
