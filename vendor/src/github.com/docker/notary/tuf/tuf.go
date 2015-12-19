// Package tuf defines the core TUF logic around manipulating a repo.
package tuf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/keys"
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
// the repo
type ErrNotLoaded struct {
	role string
}

func (err ErrNotLoaded) Error() string {
	return fmt.Sprintf("%s role has not been loaded", err.role)
}

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
	keysDB        *keys.KeyDB
	cryptoService signed.CryptoService
}

// NewRepo initializes a Repo instance with a keysDB and a signer.
// If the Repo will only be used for reading, the signer should be nil.
func NewRepo(keysDB *keys.KeyDB, cryptoService signed.CryptoService) *Repo {
	repo := &Repo{
		Targets:       make(map[string]*data.SignedTargets),
		keysDB:        keysDB,
		cryptoService: cryptoService,
	}
	return repo
}

// AddBaseKeys is used to add keys to the role in root.json
func (tr *Repo) AddBaseKeys(role string, keys ...data.PublicKey) error {
	if tr.Root == nil {
		return ErrNotLoaded{role: "root"}
	}
	ids := []string{}
	for _, k := range keys {
		// Store only the public portion
		tr.Root.Signed.Keys[k.ID()] = k
		tr.keysDB.AddKey(k)
		tr.Root.Signed.Roles[role].KeyIDs = append(tr.Root.Signed.Roles[role].KeyIDs, k.ID())
		ids = append(ids, k.ID())
	}
	r, err := data.NewRole(
		role,
		tr.Root.Signed.Roles[role].Threshold,
		ids,
		nil,
		nil,
	)
	if err != nil {
		return err
	}
	tr.keysDB.AddRole(r)
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
	r := tr.keysDB.GetRole(role)
	err := tr.RemoveBaseKeys(role, r.KeyIDs...)
	if err != nil {
		return err
	}
	return tr.AddBaseKeys(role, keys...)
}

// RemoveBaseKeys is used to remove keys from the roles in root.json
func (tr *Repo) RemoveBaseKeys(role string, keyIDs ...string) error {
	if tr.Root == nil {
		return ErrNotLoaded{role: "root"}
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

// GetDelegation finds the role entry representing the provided
// role name or ErrInvalidRole
func (tr *Repo) GetDelegation(role string) (*data.Role, error) {
	r := data.Role{Name: role}
	if !r.IsDelegation() {
		return nil, data.ErrInvalidRole{Role: role, Reason: "not a valid delegated role"}
	}

	parent := path.Dir(role)

	// check the parent role
	if parentRole := tr.keysDB.GetRole(parent); parentRole == nil {
		return nil, data.ErrInvalidRole{Role: role, Reason: "parent role not found"}
	}

	// check the parent role's metadata
	p, ok := tr.Targets[parent]
	if !ok { // the parent targetfile may not exist yet, so it can't be in the list
		return nil, data.ErrNoSuchRole{Role: role}
	}

	foundAt := utils.FindRoleIndex(p.Signed.Delegations.Roles, role)
	if foundAt < 0 {
		return nil, data.ErrNoSuchRole{Role: role}
	}
	return p.Signed.Delegations.Roles[foundAt], nil
}

// UpdateDelegations updates the appropriate delegations, either adding
// a new delegation or updating an existing one. If keys are
// provided, the IDs will be added to the role (if they do not exist
// there already), and the keys will be added to the targets file.
func (tr *Repo) UpdateDelegations(role *data.Role, keys []data.PublicKey) error {
	if !role.IsDelegation() || !role.IsValid() {
		return data.ErrInvalidRole{Role: role.Name, Reason: "not a valid delegated role"}
	}
	parent := path.Dir(role.Name)

	if err := tr.VerifyCanSign(parent); err != nil {
		return err
	}

	// check the parent role's metadata
	p, ok := tr.Targets[parent]
	if !ok { // the parent targetfile may not exist yet - if not, then create it
		var err error
		p, err = tr.InitTargets(parent)
		if err != nil {
			return err
		}
	}

	for _, k := range keys {
		if !utils.StrSliceContains(role.KeyIDs, k.ID()) {
			role.KeyIDs = append(role.KeyIDs, k.ID())
		}
		p.Signed.Delegations.Keys[k.ID()] = k
		tr.keysDB.AddKey(k)
	}

	// if the role has fewer keys than the threshold, it
	// will never be able to create a valid targets file
	// and should be considered invalid.
	if len(role.KeyIDs) < role.Threshold {
		return data.ErrInvalidRole{Role: role.Name, Reason: "insufficient keys to meet threshold"}
	}

	foundAt := utils.FindRoleIndex(p.Signed.Delegations.Roles, role.Name)

	if foundAt >= 0 {
		p.Signed.Delegations.Roles[foundAt] = role
	} else {
		p.Signed.Delegations.Roles = append(p.Signed.Delegations.Roles, role)
	}
	// We've made a change to parent. Set it to dirty
	p.Dirty = true

	// We don't actually want to create the new delegation metadata yet.
	// When we add a delegation, it may only be signable by a key we don't have
	// (hence we are delegating signing).

	tr.keysDB.AddRole(role)
	utils.RemoveUnusedKeys(p)

	return nil
}

// DeleteDelegation removes a delegated targets role from its parent
// targets object. It also deletes the delegation from the snapshot.
// DeleteDelegation will only make use of the role Name field.
func (tr *Repo) DeleteDelegation(role data.Role) error {
	if !role.IsDelegation() {
		return data.ErrInvalidRole{Role: role.Name, Reason: "not a valid delegated role"}
	}
	// the role variable must not be used past this assignment for safety
	name := role.Name

	parent := path.Dir(name)
	if err := tr.VerifyCanSign(parent); err != nil {
		return err
	}

	// delete delegated data from Targets map and Snapshot - if they don't
	// exist, these are no-op
	delete(tr.Targets, name)
	tr.Snapshot.DeleteMeta(name)

	p, ok := tr.Targets[parent]
	if !ok {
		// if there is no parent metadata (the role exists though), then this
		// is as good as done.
		return nil
	}

	foundAt := utils.FindRoleIndex(p.Signed.Delegations.Roles, name)

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

// InitRepo creates the base files for a repo. It inspects data.ValidRoles and
// data.ValidTypes to determine what the role names and filename should be. It
// also relies on the keysDB having already been populated with the keys and
// roles.
func (tr *Repo) InitRepo(consistent bool) error {
	if err := tr.InitRoot(consistent); err != nil {
		return err
	}
	if _, err := tr.InitTargets(data.CanonicalTargetsRole); err != nil {
		return err
	}
	if err := tr.InitSnapshot(); err != nil {
		return err
	}
	return tr.InitTimestamp()
}

// InitRoot initializes an empty root file with the 4 core roles based
// on the current content of th ekey db
func (tr *Repo) InitRoot(consistent bool) error {
	rootRoles := make(map[string]*data.RootRole)
	rootKeys := make(map[string]data.PublicKey)
	for _, r := range data.ValidRoles {
		role := tr.keysDB.GetRole(r)
		if role == nil {
			return data.ErrInvalidRole{Role: data.CanonicalRootRole, Reason: "root role not initialized in key database"}
		}
		rootRoles[r] = &role.RootRole
		for _, kid := range role.KeyIDs {
			// don't need to check if GetKey returns nil, Key presence was
			// checked by KeyDB when role was added.
			key := tr.keysDB.GetKey(kid)
			rootKeys[kid] = key
		}
	}
	root, err := data.NewRoot(rootKeys, rootRoles, consistent)
	if err != nil {
		return err
	}
	tr.Root = root
	return nil
}

// InitTargets initializes an empty targets, and returns the new empty target
func (tr *Repo) InitTargets(role string) (*data.SignedTargets, error) {
	r := data.Role{Name: role}
	if !r.IsDelegation() && data.CanonicalRole(role) != data.CanonicalTargetsRole {
		return nil, data.ErrInvalidRole{
			Role:   role,
			Reason: fmt.Sprintf("role is not a valid targets role name: %s", role),
		}
	}
	targets := data.NewTargets()
	tr.Targets[data.RoleName(role)] = targets
	return targets, nil
}

// InitSnapshot initializes a snapshot based on the current root and targets
func (tr *Repo) InitSnapshot() error {
	if tr.Root == nil {
		return ErrNotLoaded{role: "root"}
	}
	root, err := tr.Root.ToSigned()
	if err != nil {
		return err
	}

	if _, ok := tr.Targets[data.RoleName(data.CanonicalTargetsRole)]; !ok {
		return ErrNotLoaded{role: "targets"}
	}
	targets, err := tr.Targets[data.RoleName(data.CanonicalTargetsRole)].ToSigned()
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

// SetRoot parses the Signed object into a SignedRoot object, sets
// the keys and roles in the KeyDB, and sets the Repo.Root field
// to the SignedRoot object.
func (tr *Repo) SetRoot(s *data.SignedRoot) error {
	for _, key := range s.Signed.Keys {
		logrus.Debug("Adding key ", key.ID())
		tr.keysDB.AddKey(key)
	}
	for roleName, role := range s.Signed.Roles {
		logrus.Debugf("Adding role %s with keys %s", roleName, strings.Join(role.KeyIDs, ","))
		baseRole, err := data.NewRole(
			roleName,
			role.Threshold,
			role.KeyIDs,
			nil,
			nil,
		)
		if err != nil {
			return err
		}
		err = tr.keysDB.AddRole(baseRole)
		if err != nil {
			return err
		}
	}
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

// SetTargets parses the Signed object into a SignedTargets object,
// reads the delegated roles and keys into the KeyDB, and sets the
// SignedTargets object agaist the role in the Repo.Targets map.
func (tr *Repo) SetTargets(role string, s *data.SignedTargets) error {
	for _, k := range s.Signed.Delegations.Keys {
		tr.keysDB.AddKey(k)
	}
	for _, r := range s.Signed.Delegations.Roles {
		tr.keysDB.AddRole(r)
	}
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
func (tr Repo) TargetDelegations(role, path, pathHex string) []*data.Role {
	if pathHex == "" {
		pathDigest := sha256.Sum256([]byte(path))
		pathHex = hex.EncodeToString(pathDigest[:])
	}
	var roles []*data.Role
	if t, ok := tr.Targets[role]; ok {
		for _, r := range t.Signed.Delegations.Roles {
			if r.CheckPrefixes(pathHex) || r.CheckPaths(path) {
				roles = append(roles, r)
			}
		}
	}
	return roles
}

// FindTarget attempts to find the target represented by the given
// path by starting at the top targets file and traversing
// appropriate delegations until the first entry is found or it
// runs out of locations to search.
// N.B. Multiple entries may exist in different delegated roles
//      for the same target. Only the first one encountered is returned.
func (tr Repo) FindTarget(path string) *data.FileMeta {
	pathDigest := sha256.Sum256([]byte(path))
	pathHex := hex.EncodeToString(pathDigest[:])

	var walkTargets func(role string) *data.FileMeta
	walkTargets = func(role string) *data.FileMeta {
		if m := tr.TargetMeta(role, path); m != nil {
			return m
		}
		// Depth first search of delegations based on order
		// as presented in current targets file for role:
		for _, r := range tr.TargetDelegations(role, path, pathHex) {
			if m := walkTargets(r.Name); m != nil {
				return m
			}
		}
		return nil
	}

	return walkTargets("targets")
}

// VerifyCanSign returns nil if the role exists and we have at least one
// signing key for the role, false otherwise.  This does not check that we have
// enough signing keys to meet the threshold, since we want to support the use
// case of multiple signers for a role.  It returns an error if the role doesn't
// exist or if there are no signing keys.
func (tr *Repo) VerifyCanSign(roleName string) error {
	role := tr.keysDB.GetRole(roleName)
	if role == nil {
		return data.ErrInvalidRole{Role: roleName, Reason: "does not exist"}
	}

	for _, keyID := range role.KeyIDs {
		k := tr.keysDB.GetKey(keyID)
		canonicalID, err := utils.CanonicalKeyID(k)
		check := []string{keyID}
		if err == nil {
			check = append(check, canonicalID)
		}
		for _, id := range check {
			p, _, err := tr.cryptoService.GetPrivateKey(id)
			if err == nil && p != nil {
				return nil
			}
		}
	}
	return signed.ErrNoKeys{KeyIDs: role.KeyIDs}
}

// AddTargets will attempt to add the given targets specifically to
// the directed role. If the metadata for the role doesn't exist yet,
// AddTargets will create one.
func (tr *Repo) AddTargets(role string, targets data.Files) (data.Files, error) {

	err := tr.VerifyCanSign(role)
	if err != nil {
		return nil, err
	}

	// check the role's metadata
	t, ok := tr.Targets[role]
	if !ok { // the targetfile may not exist yet - if not, then create it
		var err error
		t, err = tr.InitTargets(role)
		if err != nil {
			return nil, err
		}
	}

	// VerifyCanSign already makes sure this is not nil
	r := tr.keysDB.GetRole(role)

	invalid := make(data.Files)
	for path, target := range targets {
		pathDigest := sha256.Sum256([]byte(path))
		pathHex := hex.EncodeToString(pathDigest[:])
		if role == data.ValidRoles["targets"] || (r.CheckPaths(path) || r.CheckPrefixes(pathHex)) {
			t.Signed.Targets[path] = target
		} else {
			invalid[path] = target
		}
	}
	t.Dirty = true
	if len(invalid) > 0 {
		return invalid, fmt.Errorf("Could not add all targets")
	}
	return nil, nil
}

// RemoveTargets removes the given target (paths) from the given target role (delegation)
func (tr *Repo) RemoveTargets(role string, targets ...string) error {
	if err := tr.VerifyCanSign(role); err != nil {
		return err
	}

	// if the role exists but metadata does not yet, then our work is done
	t, ok := tr.Targets[role]
	if ok {
		for _, path := range targets {
			delete(t.Signed.Targets, path)
		}
		t.Dirty = true
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
	root := tr.keysDB.GetRole(data.ValidRoles["root"])
	signed, err := tr.Root.ToSigned()
	if err != nil {
		return nil, err
	}
	signed, err = tr.sign(signed, *root)
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
	targets := tr.keysDB.GetRole(role)
	signed, err = tr.sign(signed, *targets)
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
	snapshot := tr.keysDB.GetRole(data.ValidRoles["snapshot"])
	signed, err = tr.sign(signed, *snapshot)
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
	timestamp := tr.keysDB.GetRole(data.ValidRoles["timestamp"])
	signed, err = tr.sign(signed, *timestamp)
	if err != nil {
		return nil, err
	}
	tr.Timestamp.Signatures = signed.Signatures
	tr.Snapshot.Dirty = false // snapshot is dirty until changes have been captured in timestamp
	return signed, nil
}

func (tr Repo) sign(signedData *data.Signed, role data.Role) (*data.Signed, error) {
	ks := make([]data.PublicKey, 0, len(role.KeyIDs))
	for _, kid := range role.KeyIDs {
		k := tr.keysDB.GetKey(kid)
		if k == nil {
			continue
		}
		ks = append(ks, k)
	}
	if len(ks) < 1 {
		return nil, keys.ErrInvalidKey
	}
	err := signed.Sign(tr.cryptoService, signedData, ks...)
	if err != nil {
		return nil, err
	}
	return signedData, nil
}
