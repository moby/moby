// Package tuf defines the core TUF logic around manipulating a repo.
package tuf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
	"github.com/docker/notary/tuf/utils"
)

// ErrSigVerifyFail - signature verification failed
type ErrSigVerifyFail struct{}

func (e ErrSigVerifyFail) Error() string {
	return "Error: Signature verification failed"
}

// ErrMetaExpired - metadata file has expired
type ErrMetaExpired struct{}

func (e ErrMetaExpired) Error() string {
	return "Error: Metadata has expired"
}

// ErrLocalRootExpired - the local root file is out of date
type ErrLocalRootExpired struct{}

func (e ErrLocalRootExpired) Error() string {
	return "Error: Local Root Has Expired"
}

// ErrNotLoaded - attempted to access data that has not been loaded into
// the repo. This means specifically that the relevant JSON file has not
// been loaded.
type ErrNotLoaded struct {
	Role string
}

func (err ErrNotLoaded) Error() string {
	return fmt.Sprintf("%s role has not been loaded", err.Role)
}

// StopWalk - used by visitor functions to signal WalkTargets to stop walking
type StopWalk struct{}

// Repo is an in memory representation of the TUF Repo.
// It operates at the data.Signed level, accepting and producing
// data.Signed objects. Users of a Repo are responsible for
// fetching raw JSON and using the Set* functions to populate
// the Repo instance.
type Repo struct {
	Root          *data.SignedRoot
	Targets       map[string]*data.SignedTargets
	Snapshot      *data.SignedSnapshot
	Timestamp     *data.SignedTimestamp
	cryptoService signed.CryptoService
}

// NewRepo initializes a Repo instance with a CryptoService.
// If the Repo will only be used for reading, the CryptoService
// can be nil.
func NewRepo(cryptoService signed.CryptoService) *Repo {
	repo := &Repo{
		Targets:       make(map[string]*data.SignedTargets),
		cryptoService: cryptoService,
	}
	return repo
}

// AddBaseKeys is used to add keys to the role in root.json
func (tr *Repo) AddBaseKeys(role string, keys ...data.PublicKey) error {
	if tr.Root == nil {
		return ErrNotLoaded{Role: data.CanonicalRootRole}
	}
	ids := []string{}
	for _, k := range keys {
		// Store only the public portion
		tr.Root.Signed.Keys[k.ID()] = k
		tr.Root.Signed.Roles[role].KeyIDs = append(tr.Root.Signed.Roles[role].KeyIDs, k.ID())
		ids = append(ids, k.ID())
	}
	tr.Root.Dirty = true

	// also, whichever role was switched out needs to be re-signed
	// root has already been marked dirty
	switch role {
	case data.CanonicalSnapshotRole:
		if tr.Snapshot != nil {
			tr.Snapshot.Dirty = true
		}
	case data.CanonicalTargetsRole:
		if target, ok := tr.Targets[data.CanonicalTargetsRole]; ok {
			target.Dirty = true
		}
	case data.CanonicalTimestampRole:
		if tr.Timestamp != nil {
			tr.Timestamp.Dirty = true
		}
	}
	return nil
}

// ReplaceBaseKeys is used to replace all keys for the given role with the new keys
func (tr *Repo) ReplaceBaseKeys(role string, keys ...data.PublicKey) error {
	r, err := tr.GetBaseRole(role)
	if err != nil {
		return err
	}
	err = tr.RemoveBaseKeys(role, r.ListKeyIDs()...)
	if err != nil {
		return err
	}
	return tr.AddBaseKeys(role, keys...)
}

// RemoveBaseKeys is used to remove keys from the roles in root.json
func (tr *Repo) RemoveBaseKeys(role string, keyIDs ...string) error {
	if tr.Root == nil {
		return ErrNotLoaded{Role: data.CanonicalRootRole}
	}
	var keep []string
	toDelete := make(map[string]struct{})
	// remove keys from specified role
	for _, k := range keyIDs {
		toDelete[k] = struct{}{}
		for _, rk := range tr.Root.Signed.Roles[role].KeyIDs {
			if k != rk {
				keep = append(keep, rk)
			}
		}
	}
	tr.Root.Signed.Roles[role].KeyIDs = keep

	// determine which keys are no longer in use by any roles
	for roleName, r := range tr.Root.Signed.Roles {
		if roleName == role {
			continue
		}
		for _, rk := range r.KeyIDs {
			if _, ok := toDelete[rk]; ok {
				delete(toDelete, rk)
			}
		}
	}

	// remove keys no longer in use by any roles
	for k := range toDelete {
		delete(tr.Root.Signed.Keys, k)
		// remove the signing key from the cryptoservice if it
		// isn't a root key. Root keys must be kept for rotation
		// signing
		if role != data.CanonicalRootRole {
			tr.cryptoService.RemoveKey(k)
		}
	}
	tr.Root.Dirty = true
	return nil
}

// GetBaseRole gets a base role from this repo's metadata
func (tr *Repo) GetBaseRole(name string) (data.BaseRole, error) {
	if !data.ValidRole(name) {
		return data.BaseRole{}, data.ErrInvalidRole{Role: name, Reason: "invalid base role name"}
	}
	if tr.Root == nil {
		return data.BaseRole{}, ErrNotLoaded{data.CanonicalRootRole}
	}
	// Find the role data public keys for the base role from TUF metadata
	baseRole, err := tr.Root.BuildBaseRole(name)
	if err != nil {
		return data.BaseRole{}, err
	}

	return baseRole, nil
}

// GetDelegationRole gets a delegation role from this repo's metadata, walking from the targets role down to the delegation itself
func (tr *Repo) GetDelegationRole(name string) (data.DelegationRole, error) {
	if !data.IsDelegation(name) {
		return data.DelegationRole{}, data.ErrInvalidRole{Role: name, Reason: "invalid delegation name"}
	}
	if tr.Root == nil {
		return data.DelegationRole{}, ErrNotLoaded{data.CanonicalRootRole}
	}
	_, ok := tr.Root.Signed.Roles[data.CanonicalTargetsRole]
	if !ok {
		return data.DelegationRole{}, ErrNotLoaded{data.CanonicalTargetsRole}
	}
	// Traverse target metadata, down to delegation itself
	// Get all public keys for the base role from TUF metadata
	_, ok = tr.Targets[data.CanonicalTargetsRole]
	if !ok {
		return data.DelegationRole{}, ErrNotLoaded{data.CanonicalTargetsRole}
	}

	// Start with top level roles in targets. Walk the chain of ancestors
	// until finding the desired role, or we run out of targets files to search.
	var foundRole *data.DelegationRole
	buildDelegationRoleVisitor := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
		// Try to find the delegation and build a DelegationRole structure
		for _, role := range tgt.Signed.Delegations.Roles {
			if role.Name == name {
				delgRole, err := tgt.BuildDelegationRole(name)
				if err != nil {
					return err
				}
				foundRole = &delgRole
				return StopWalk{}
			}
		}
		return nil
	}

	// Walk to the parent of this delegation, since that is where its role metadata exists
	err := tr.WalkTargets("", path.Dir(name), buildDelegationRoleVisitor)
	if err != nil {
		return data.DelegationRole{}, err
	}

	// We never found the delegation. In the context of this repo it is considered
	// invalid. N.B. it may be that it existed at one point but an ancestor has since
	// been modified/removed.
	if foundRole == nil {
		return data.DelegationRole{}, data.ErrInvalidRole{Role: name, Reason: "delegation does not exist"}
	}

	return *foundRole, nil
}

// GetAllLoadedRoles returns a list of all role entries loaded in this TUF repo, could be empty
func (tr *Repo) GetAllLoadedRoles() []*data.Role {
	var res []*data.Role
	if tr.Root == nil {
		// if root isn't loaded, we should consider we have no loaded roles because we can't
		// trust any other state that might be present
		return res
	}
	for name, rr := range tr.Root.Signed.Roles {
		res = append(res, &data.Role{
			RootRole: *rr,
			Name:     name,
		})
	}
	for _, delegate := range tr.Targets {
		for _, r := range delegate.Signed.Delegations.Roles {
			res = append(res, r)
		}
	}
	return res
}

// Walk to parent, and either create or update this delegation.  We can only create a new delegation if we're given keys
// Ensure all updates are valid, by checking against parent ancestor paths and ensuring the keys meet the role threshold.
func delegationUpdateVisitor(roleName string, addKeys data.KeyList, removeKeys, addPaths, removePaths []string, clearAllPaths bool, newThreshold int) walkVisitorFunc {
	return func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
		var err error
		// Validate the changes underneath this restricted validRole for adding paths, reject invalid path additions
		if len(addPaths) != len(data.RestrictDelegationPathPrefixes(validRole.Paths, addPaths)) {
			return data.ErrInvalidRole{Role: roleName, Reason: "invalid paths to add to role"}
		}
		// Try to find the delegation and amend it using our changelist
		var delgRole *data.Role
		for _, role := range tgt.Signed.Delegations.Roles {
			if role.Name == roleName {
				// Make a copy and operate on this role until we validate the changes
				keyIDCopy := make([]string, len(role.KeyIDs))
				copy(keyIDCopy, role.KeyIDs)
				pathsCopy := make([]string, len(role.Paths))
				copy(pathsCopy, role.Paths)
				delgRole = &data.Role{
					RootRole: data.RootRole{
						KeyIDs:    keyIDCopy,
						Threshold: role.Threshold,
					},
					Name:  role.Name,
					Paths: pathsCopy,
				}
				delgRole.RemovePaths(removePaths)
				if clearAllPaths {
					delgRole.Paths = []string{}
				}
				delgRole.AddPaths(addPaths)
				delgRole.RemoveKeys(removeKeys)
				break
			}
		}
		// We didn't find the role earlier, so create it only if we have keys to add
		if delgRole == nil {
			if len(addKeys) > 0 {
				delgRole, err = data.NewRole(roleName, newThreshold, addKeys.IDs(), addPaths)
				if err != nil {
					return err
				}
			} else {
				// If we can't find the role and didn't specify keys to add, this is an error
				return data.ErrInvalidRole{Role: roleName, Reason: "cannot create new delegation without keys"}
			}
		}
		// Add the key IDs to the role and the keys themselves to the parent
		for _, k := range addKeys {
			if !utils.StrSliceContains(delgRole.KeyIDs, k.ID()) {
				delgRole.KeyIDs = append(delgRole.KeyIDs, k.ID())
			}
		}
		// Make sure we have a valid role still
		if len(delgRole.KeyIDs) < delgRole.Threshold {
			return data.ErrInvalidRole{Role: roleName, Reason: "insufficient keys to meet threshold"}
		}
		// NOTE: this closure CANNOT error after this point, as we've committed to editing the SignedTargets metadata in the repo object.
		// Any errors related to updating this delegation must occur before this point.
		// If all of our changes were valid, we should edit the actual SignedTargets to match our copy
		for _, k := range addKeys {
			tgt.Signed.Delegations.Keys[k.ID()] = k
		}
		foundAt := utils.FindRoleIndex(tgt.Signed.Delegations.Roles, delgRole.Name)
		if foundAt < 0 {
			tgt.Signed.Delegations.Roles = append(tgt.Signed.Delegations.Roles, delgRole)
		} else {
			tgt.Signed.Delegations.Roles[foundAt] = delgRole
		}
		tgt.Dirty = true
		utils.RemoveUnusedKeys(tgt)
		return StopWalk{}
	}
}

// UpdateDelegationKeys updates the appropriate delegations, either adding
// a new delegation or updating an existing one. If keys are
// provided, the IDs will be added to the role (if they do not exist
// there already), and the keys will be added to the targets file.
func (tr *Repo) UpdateDelegationKeys(roleName string, addKeys data.KeyList, removeKeys []string, newThreshold int) error {
	if !data.IsDelegation(roleName) {
		return data.ErrInvalidRole{Role: roleName, Reason: "not a valid delegated role"}
	}
	parent := path.Dir(roleName)

	if err := tr.VerifyCanSign(parent); err != nil {
		return err
	}

	// check the parent role's metadata
	_, ok := tr.Targets[parent]
	if !ok { // the parent targetfile may not exist yet - if not, then create it
		var err error
		_, err = tr.InitTargets(parent)
		if err != nil {
			return err
		}
	}

	// Walk to the parent of this delegation, since that is where its role metadata exists
	// We do not have to verify that the walker reached its desired role in this scenario
	// since we've already done another walk to the parent role in VerifyCanSign, and potentially made a targets file
	err := tr.WalkTargets("", parent, delegationUpdateVisitor(roleName, addKeys, removeKeys, []string{}, []string{}, false, newThreshold))
	if err != nil {
		return err
	}
	return nil
}

// UpdateDelegationPaths updates the appropriate delegation's paths.
// It is not allowed to create a new delegation.
func (tr *Repo) UpdateDelegationPaths(roleName string, addPaths, removePaths []string, clearPaths bool) error {
	if !data.IsDelegation(roleName) {
		return data.ErrInvalidRole{Role: roleName, Reason: "not a valid delegated role"}
	}
	parent := path.Dir(roleName)

	if err := tr.VerifyCanSign(parent); err != nil {
		return err
	}

	// check the parent role's metadata
	_, ok := tr.Targets[parent]
	if !ok { // the parent targetfile may not exist yet
		// if not, this is an error because a delegation must exist to edit only paths
		return data.ErrInvalidRole{Role: roleName, Reason: "no valid delegated role exists"}
	}

	// Walk to the parent of this delegation, since that is where its role metadata exists
	// We do not have to verify that the walker reached its desired role in this scenario
	// since we've already done another walk to the parent role in VerifyCanSign
	err := tr.WalkTargets("", parent, delegationUpdateVisitor(roleName, data.KeyList{}, []string{}, addPaths, removePaths, clearPaths, notary.MinThreshold))
	if err != nil {
		return err
	}
	return nil
}

// DeleteDelegation removes a delegated targets role from its parent
// targets object. It also deletes the delegation from the snapshot.
// DeleteDelegation will only make use of the role Name field.
func (tr *Repo) DeleteDelegation(roleName string) error {
	if !data.IsDelegation(roleName) {
		return data.ErrInvalidRole{Role: roleName, Reason: "not a valid delegated role"}
	}

	parent := path.Dir(roleName)
	if err := tr.VerifyCanSign(parent); err != nil {
		return err
	}

	// delete delegated data from Targets map and Snapshot - if they don't
	// exist, these are no-op
	delete(tr.Targets, roleName)
	tr.Snapshot.DeleteMeta(roleName)

	p, ok := tr.Targets[parent]
	if !ok {
		// if there is no parent metadata (the role exists though), then this
		// is as good as done.
		return nil
	}

	foundAt := utils.FindRoleIndex(p.Signed.Delegations.Roles, roleName)

	if foundAt >= 0 {
		var roles []*data.Role
		// slice out deleted role
		roles = append(roles, p.Signed.Delegations.Roles[:foundAt]...)
		if foundAt+1 < len(p.Signed.Delegations.Roles) {
			roles = append(roles, p.Signed.Delegations.Roles[foundAt+1:]...)
		}
		p.Signed.Delegations.Roles = roles

		utils.RemoveUnusedKeys(p)

		p.Dirty = true
	} // if the role wasn't found, it's a good as deleted

	return nil
}

// InitRoot initializes an empty root file with the 4 core roles passed to the
// method, and the consistent flag.
func (tr *Repo) InitRoot(root, timestamp, snapshot, targets data.BaseRole, consistent bool) error {
	rootRoles := make(map[string]*data.RootRole)
	rootKeys := make(map[string]data.PublicKey)

	for _, r := range []data.BaseRole{root, timestamp, snapshot, targets} {
		rootRoles[r.Name] = &data.RootRole{
			Threshold: r.Threshold,
			KeyIDs:    r.ListKeyIDs(),
		}
		for kid, k := range r.Keys {
			rootKeys[kid] = k
		}
	}
	r, err := data.NewRoot(rootKeys, rootRoles, consistent)
	if err != nil {
		return err
	}
	tr.Root = r
	return nil
}

// InitTargets initializes an empty targets, and returns the new empty target
func (tr *Repo) InitTargets(role string) (*data.SignedTargets, error) {
	if !data.IsDelegation(role) && role != data.CanonicalTargetsRole {
		return nil, data.ErrInvalidRole{
			Role:   role,
			Reason: fmt.Sprintf("role is not a valid targets role name: %s", role),
		}
	}
	targets := data.NewTargets()
	tr.Targets[role] = targets
	return targets, nil
}

// InitSnapshot initializes a snapshot based on the current root and targets
func (tr *Repo) InitSnapshot() error {
	if tr.Root == nil {
		return ErrNotLoaded{Role: data.CanonicalRootRole}
	}
	root, err := tr.Root.ToSigned()
	if err != nil {
		return err
	}

	if _, ok := tr.Targets[data.CanonicalTargetsRole]; !ok {
		return ErrNotLoaded{Role: data.CanonicalTargetsRole}
	}
	targets, err := tr.Targets[data.CanonicalTargetsRole].ToSigned()
	if err != nil {
		return err
	}
	snapshot, err := data.NewSnapshot(root, targets)
	if err != nil {
		return err
	}
	tr.Snapshot = snapshot
	return nil
}

// InitTimestamp initializes a timestamp based on the current snapshot
func (tr *Repo) InitTimestamp() error {
	snap, err := tr.Snapshot.ToSigned()
	if err != nil {
		return err
	}
	timestamp, err := data.NewTimestamp(snap)
	if err != nil {
		return err
	}

	tr.Timestamp = timestamp
	return nil
}

// SetRoot sets the Repo.Root field to the SignedRoot object.
func (tr *Repo) SetRoot(s *data.SignedRoot) error {
	tr.Root = s
	return nil
}

// SetTimestamp parses the Signed object into a SignedTimestamp object
// and sets the Repo.Timestamp field.
func (tr *Repo) SetTimestamp(s *data.SignedTimestamp) error {
	tr.Timestamp = s
	return nil
}

// SetSnapshot parses the Signed object into a SignedSnapshots object
// and sets the Repo.Snapshot field.
func (tr *Repo) SetSnapshot(s *data.SignedSnapshot) error {
	tr.Snapshot = s
	return nil
}

// SetTargets sets the SignedTargets object agaist the role in the
// Repo.Targets map.
func (tr *Repo) SetTargets(role string, s *data.SignedTargets) error {
	tr.Targets[role] = s
	return nil
}

// TargetMeta returns the FileMeta entry for the given path in the
// targets file associated with the given role. This may be nil if
// the target isn't found in the targets file.
func (tr Repo) TargetMeta(role, path string) *data.FileMeta {
	if t, ok := tr.Targets[role]; ok {
		if m, ok := t.Signed.Targets[path]; ok {
			return &m
		}
	}
	return nil
}

// TargetDelegations returns a slice of Roles that are valid publishers
// for the target path provided.
func (tr Repo) TargetDelegations(role, path string) []*data.Role {
	var roles []*data.Role
	if t, ok := tr.Targets[role]; ok {
		for _, r := range t.Signed.Delegations.Roles {
			if r.CheckPaths(path) {
				roles = append(roles, r)
			}
		}
	}
	return roles
}

// VerifyCanSign returns nil if the role exists and we have at least one
// signing key for the role, false otherwise.  This does not check that we have
// enough signing keys to meet the threshold, since we want to support the use
// case of multiple signers for a role.  It returns an error if the role doesn't
// exist or if there are no signing keys.
func (tr *Repo) VerifyCanSign(roleName string) error {
	var (
		role data.BaseRole
		err  error
	)
	// we only need the BaseRole part of a delegation because we're just
	// checking KeyIDs
	if data.IsDelegation(roleName) {
		r, err := tr.GetDelegationRole(roleName)
		if err != nil {
			return err
		}
		role = r.BaseRole
	} else {
		role, err = tr.GetBaseRole(roleName)
	}
	if err != nil {
		return data.ErrInvalidRole{Role: roleName, Reason: "does not exist"}
	}

	for keyID, k := range role.Keys {
		check := []string{keyID}
		if canonicalID, err := utils.CanonicalKeyID(k); err == nil {
			check = append(check, canonicalID)
		}
		for _, id := range check {
			p, _, err := tr.cryptoService.GetPrivateKey(id)
			if err == nil && p != nil {
				return nil
			}
		}
	}
	return signed.ErrNoKeys{KeyIDs: role.ListKeyIDs()}
}

// used for walking the targets/delegations tree, potentially modifying the underlying SignedTargets for the repo
type walkVisitorFunc func(*data.SignedTargets, data.DelegationRole) interface{}

// WalkTargets will apply the specified visitor function to iteratively walk the targets/delegation metadata tree,
// until receiving a StopWalk.  The walk starts from the base "targets" role, and searches for the correct targetPath and/or rolePath
// to call the visitor function on.  Any roles passed into skipRoles will be excluded from the walk, as well as roles in those subtrees
func (tr *Repo) WalkTargets(targetPath, rolePath string, visitTargets walkVisitorFunc, skipRoles ...string) error {
	// Start with the base targets role, which implicitly has the "" targets path
	targetsRole, err := tr.GetBaseRole(data.CanonicalTargetsRole)
	if err != nil {
		return err
	}
	// Make the targets role have the empty path, when we treat it as a delegation role
	roles := []data.DelegationRole{
		{
			BaseRole: targetsRole,
			Paths:    []string{""},
		},
	}

	for len(roles) > 0 {
		role := roles[0]
		roles = roles[1:]

		// Check the role metadata
		signedTgt, ok := tr.Targets[role.Name]
		if !ok {
			// The role meta doesn't exist in the repo so continue onward
			continue
		}

		// We're at a prefix of the desired role subtree, so add its delegation role children and continue walking
		if strings.HasPrefix(rolePath, role.Name+"/") {
			roles = append(roles, signedTgt.GetValidDelegations(role)...)
			continue
		}

		// Determine whether to visit this role or not:
		// If the paths validate against the specified targetPath and the rolePath is empty or is in the subtree
		// Also check if we are choosing to skip visiting this role on this walk (see ListTargets and GetTargetByName priority)
		if isValidPath(targetPath, role) && isAncestorRole(role.Name, rolePath) && !utils.StrSliceContains(skipRoles, role.Name) {
			// If we had matching path or role name, visit this target and determine whether or not to keep walking
			res := visitTargets(signedTgt, role)
			switch typedRes := res.(type) {
			case StopWalk:
				// If the visitor function signalled a stop, return nil to finish the walk
				return nil
			case nil:
				// If the visitor function signalled to continue, add this role's delegation to the walk
				roles = append(roles, signedTgt.GetValidDelegations(role)...)
			case error:
				// Propagate any errors from the visitor
				return typedRes
			default:
				// Return out with an error if we got a different result
				return fmt.Errorf("unexpected return while walking: %v", res)
			}

		}
	}
	return nil
}

// helper function that returns whether the candidateChild role name is an ancestor or equal to the candidateAncestor role name
// Will return true if given an empty candidateAncestor role name
// The HasPrefix check is for determining whether the role name for candidateChild is a child (direct or further down the chain)
// of candidateAncestor, for ex: candidateAncestor targets/a and candidateChild targets/a/b/c
func isAncestorRole(candidateChild, candidateAncestor string) bool {
	return candidateAncestor == "" || candidateAncestor == candidateChild || strings.HasPrefix(candidateChild, candidateAncestor+"/")
}

// helper function that returns whether the delegation Role is valid against the given path
// Will return true if given an empty candidatePath
func isValidPath(candidatePath string, delgRole data.DelegationRole) bool {
	return candidatePath == "" || delgRole.CheckPaths(candidatePath)
}

// AddTargets will attempt to add the given targets specifically to
// the directed role. If the metadata for the role doesn't exist yet,
// AddTargets will create one.
func (tr *Repo) AddTargets(role string, targets data.Files) (data.Files, error) {
	err := tr.VerifyCanSign(role)
	if err != nil {
		return nil, err
	}

	// check existence of the role's metadata
	_, ok := tr.Targets[role]
	if !ok { // the targetfile may not exist yet - if not, then create it
		var err error
		_, err = tr.InitTargets(role)
		if err != nil {
			return nil, err
		}
	}

	addedTargets := make(data.Files)
	addTargetVisitor := func(targetPath string, targetMeta data.FileMeta) func(*data.SignedTargets, data.DelegationRole) interface{} {
		return func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
			// We've already validated the role's target path in our walk, so just modify the metadata
			tgt.Signed.Targets[targetPath] = targetMeta
			tgt.Dirty = true
			// Also add to our new addedTargets map to keep track of every target we've added successfully
			addedTargets[targetPath] = targetMeta
			return StopWalk{}
		}
	}

	// Walk the role tree while validating the target paths, and add all of our targets
	for path, target := range targets {
		tr.WalkTargets(path, role, addTargetVisitor(path, target))
	}
	if len(addedTargets) != len(targets) {
		return nil, fmt.Errorf("Could not add all targets")
	}
	return nil, nil
}

// RemoveTargets removes the given target (paths) from the given target role (delegation)
func (tr *Repo) RemoveTargets(role string, targets ...string) error {
	if err := tr.VerifyCanSign(role); err != nil {
		return err
	}

	removeTargetVisitor := func(targetPath string) func(*data.SignedTargets, data.DelegationRole) interface{} {
		return func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
			// We've already validated the role path in our walk, so just modify the metadata
			// We don't check against the target path against the valid role paths because it's
			// possible we got into an invalid state and are trying to fix it
			delete(tgt.Signed.Targets, targetPath)
			tgt.Dirty = true
			return StopWalk{}
		}
	}

	// if the role exists but metadata does not yet, then our work is done
	_, ok := tr.Targets[role]
	if ok {
		for _, path := range targets {
			tr.WalkTargets("", role, removeTargetVisitor(path))
		}
	}

	return nil
}

// UpdateSnapshot updates the FileMeta for the given role based on the Signed object
func (tr *Repo) UpdateSnapshot(role string, s *data.Signed) error {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return err
	}
	meta, err := data.NewFileMeta(bytes.NewReader(jsonData), "sha256")
	if err != nil {
		return err
	}
	tr.Snapshot.Signed.Meta[role] = meta
	tr.Snapshot.Dirty = true
	return nil
}

// UpdateTimestamp updates the snapshot meta in the timestamp based on the Signed object
func (tr *Repo) UpdateTimestamp(s *data.Signed) error {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return err
	}
	meta, err := data.NewFileMeta(bytes.NewReader(jsonData), "sha256")
	if err != nil {
		return err
	}
	tr.Timestamp.Signed.Meta["snapshot"] = meta
	tr.Timestamp.Dirty = true
	return nil
}

// SignRoot signs the root
func (tr *Repo) SignRoot(expires time.Time) (*data.Signed, error) {
	logrus.Debug("signing root...")
	tr.Root.Signed.Expires = expires
	tr.Root.Signed.Version++
	root, err := tr.GetBaseRole(data.CanonicalRootRole)
	if err != nil {
		return nil, err
	}
	signed, err := tr.Root.ToSigned()
	if err != nil {
		return nil, err
	}
	signed, err = tr.sign(signed, root)
	if err != nil {
		return nil, err
	}
	tr.Root.Signatures = signed.Signatures
	return signed, nil
}

// SignTargets signs the targets file for the given top level or delegated targets role
func (tr *Repo) SignTargets(role string, expires time.Time) (*data.Signed, error) {
	logrus.Debugf("sign targets called for role %s", role)
	if _, ok := tr.Targets[role]; !ok {
		return nil, data.ErrInvalidRole{
			Role:   role,
			Reason: "SignTargets called with non-existant targets role",
		}
	}
	tr.Targets[role].Signed.Expires = expires
	tr.Targets[role].Signed.Version++
	signed, err := tr.Targets[role].ToSigned()
	if err != nil {
		logrus.Debug("errored getting targets data.Signed object")
		return nil, err
	}

	var targets data.BaseRole
	if role == data.CanonicalTargetsRole {
		targets, err = tr.GetBaseRole(role)
	} else {
		tr, err := tr.GetDelegationRole(role)
		if err != nil {
			return nil, err
		}
		targets = tr.BaseRole
	}
	if err != nil {
		return nil, err
	}

	signed, err = tr.sign(signed, targets)
	if err != nil {
		logrus.Debug("errored signing ", role)
		return nil, err
	}
	tr.Targets[role].Signatures = signed.Signatures
	return signed, nil
}

// SignSnapshot updates the snapshot based on the current targets and root then signs it
func (tr *Repo) SignSnapshot(expires time.Time) (*data.Signed, error) {
	logrus.Debug("signing snapshot...")
	signedRoot, err := tr.Root.ToSigned()
	if err != nil {
		return nil, err
	}
	err = tr.UpdateSnapshot("root", signedRoot)
	if err != nil {
		return nil, err
	}
	tr.Root.Dirty = false // root dirty until changes captures in snapshot
	for role, targets := range tr.Targets {
		signedTargets, err := targets.ToSigned()
		if err != nil {
			return nil, err
		}
		err = tr.UpdateSnapshot(role, signedTargets)
		if err != nil {
			return nil, err
		}
		targets.Dirty = false
	}
	tr.Snapshot.Signed.Expires = expires
	tr.Snapshot.Signed.Version++
	signed, err := tr.Snapshot.ToSigned()
	if err != nil {
		return nil, err
	}
	snapshot, err := tr.GetBaseRole(data.CanonicalSnapshotRole)
	if err != nil {
		return nil, err
	}
	signed, err = tr.sign(signed, snapshot)
	if err != nil {
		return nil, err
	}
	tr.Snapshot.Signatures = signed.Signatures
	return signed, nil
}

// SignTimestamp updates the timestamp based on the current snapshot then signs it
func (tr *Repo) SignTimestamp(expires time.Time) (*data.Signed, error) {
	logrus.Debug("SignTimestamp")
	signedSnapshot, err := tr.Snapshot.ToSigned()
	if err != nil {
		return nil, err
	}
	err = tr.UpdateTimestamp(signedSnapshot)
	if err != nil {
		return nil, err
	}
	tr.Timestamp.Signed.Expires = expires
	tr.Timestamp.Signed.Version++
	signed, err := tr.Timestamp.ToSigned()
	if err != nil {
		return nil, err
	}
	timestamp, err := tr.GetBaseRole(data.CanonicalTimestampRole)
	if err != nil {
		return nil, err
	}
	signed, err = tr.sign(signed, timestamp)
	if err != nil {
		return nil, err
	}
	tr.Timestamp.Signatures = signed.Signatures
	tr.Snapshot.Dirty = false // snapshot is dirty until changes have been captured in timestamp
	return signed, nil
}

func (tr Repo) sign(signedData *data.Signed, role data.BaseRole) (*data.Signed, error) {
	ks := role.ListKeys()
	if len(ks) < 1 {
		return nil, signed.ErrNoKeys{}
	}
	err := signed.Sign(tr.cryptoService, signedData, ks...)
	if err != nil {
		return nil, err
	}
	return signedData, nil
}
