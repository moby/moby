// tuf defines the core TUF logic around manipulating a repo.
package tuf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/endophage/gotuf/data"
	"github.com/endophage/gotuf/errors"
	"github.com/endophage/gotuf/keys"
	"github.com/endophage/gotuf/signed"
	"github.com/endophage/gotuf/utils"
)

type ErrSigVerifyFail struct{}

func (e ErrSigVerifyFail) Error() string {
	return "Error: Signature verification failed"
}

type ErrMetaExpired struct{}

func (e ErrMetaExpired) Error() string {
	return "Error: Metadata has expired"
}

type ErrLocalRootExpired struct{}

func (e ErrLocalRootExpired) Error() string {
	return "Error: Local Root Has Expired"
}

type ErrNotLoaded struct {
	role string
}

func (err ErrNotLoaded) Error() string {
	return fmt.Sprintf("%s role has not been loaded", err.role)
}

// TufRepo is an in memory representation of the TUF Repo.
// It operates at the data.Signed level, accepting and producing
// data.Signed objects. Users of a TufRepo are responsible for
// fetching raw JSON and using the Set* functions to populate
// the TufRepo instance.
type TufRepo struct {
	Root          *data.SignedRoot
	Targets       map[string]*data.SignedTargets
	Snapshot      *data.SignedSnapshot
	Timestamp     *data.SignedTimestamp
	keysDB        *keys.KeyDB
	cryptoService signed.CryptoService
}

// NewTufRepo initializes a TufRepo instance with a keysDB and a signer.
// If the TufRepo will only be used for reading, the signer should be nil.
func NewTufRepo(keysDB *keys.KeyDB, cryptoService signed.CryptoService) *TufRepo {
	repo := &TufRepo{
		Targets:       make(map[string]*data.SignedTargets),
		keysDB:        keysDB,
		cryptoService: cryptoService,
	}
	return repo
}

// AddBaseKeys is used to add keys to the role in root.json
func (tr *TufRepo) AddBaseKeys(role string, keys ...*data.TUFKey) error {
	if tr.Root == nil {
		return ErrNotLoaded{role: "root"}
	}
	for _, k := range keys {
		// Store only the public portion
		pubKey := *k
		pubKey.Value.Private = nil
		tr.Root.Signed.Keys[pubKey.ID()] = &pubKey
		tr.keysDB.AddKey(&pubKey)
		tr.Root.Signed.Roles[role].KeyIDs = append(tr.Root.Signed.Roles[role].KeyIDs, pubKey.ID())
	}
	tr.Root.Dirty = true
	return nil

}

// RemoveKeys is used to remove keys from the roles in root.json
func (tr *TufRepo) RemoveBaseKeys(role string, keyIDs ...string) error {
	if tr.Root == nil {
		return ErrNotLoaded{role: "root"}
	}
	keep := make([]string, 0)
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
	for k, _ := range toDelete {
		delete(tr.Root.Signed.Keys, k)
	}
	tr.Root.Dirty = true
	return nil
}

// UpdateDelegations updates the appropriate delegations, either adding
// a new delegation or updating an existing one. If keys are
// provided, the IDs will be added to the role (if they do not exist
// there already), and the keys will be added to the targets file.
// The "before" argument specifies another role which this new role
// will be added in front of (i.e. higher priority) in the delegation list.
// An empty before string indicates to add the role to the end of the
// delegation list.
// A new, empty, targets file will be created for the new role.
func (tr *TufRepo) UpdateDelegations(role *data.Role, keys []data.Key, before string) error {
	if !role.IsDelegation() || !role.IsValid() {
		return errors.ErrInvalidRole{}
	}
	parent := filepath.Dir(role.Name)
	p, ok := tr.Targets[parent]
	if !ok {
		return errors.ErrInvalidRole{}
	}
	for _, k := range keys {
		key := data.NewPublicKey(k.Algorithm(), k.Public())
		if !utils.StrSliceContains(role.KeyIDs, key.ID()) {
			role.KeyIDs = append(role.KeyIDs, key.ID())
		}
		p.Signed.Delegations.Keys[key.ID()] = key
		tr.keysDB.AddKey(key)
	}

	i := -1
	var r *data.Role
	for i, r = range p.Signed.Delegations.Roles {
		if r.Name == role.Name {
			break
		}
	}
	if i >= 0 {
		p.Signed.Delegations.Roles[i] = role
	} else {
		p.Signed.Delegations.Roles = append(p.Signed.Delegations.Roles, role)
	}
	p.Dirty = true

	roleTargets := data.NewTargets() // NewTargets always marked Dirty
	tr.Targets[role.Name] = roleTargets

	tr.keysDB.AddRole(role)

	return nil
}

// InitRepo creates the base files for a repo. It inspects data.ValidRoles and
// data.ValidTypes to determine what the role names and filename should be. It
// also relies on the keysDB having already been populated with the keys and
// roles.
func (tr *TufRepo) InitRepo(consistent bool) error {
	if err := tr.InitRoot(consistent); err != nil {
		return err
	}
	if err := tr.InitTargets(); err != nil {
		return err
	}
	if err := tr.InitSnapshot(); err != nil {
		return err
	}
	return tr.InitTimestamp()
}

func (tr *TufRepo) InitRoot(consistent bool) error {
	rootRoles := make(map[string]*data.RootRole)
	rootKeys := make(map[string]data.PublicKey)
	for _, r := range data.ValidRoles {
		role := tr.keysDB.GetRole(r)
		if role == nil {
			return errors.ErrInvalidRole{}
		}
		rootRoles[r] = &role.RootRole
		for _, kid := range role.KeyIDs {
			// don't need to check if GetKey returns nil, Key presence was
			// checked by KeyDB when role was added.
			key := tr.keysDB.GetKey(kid)
			// Create new key object to doubly ensure private key is excluded
			k := data.NewPublicKey(key.Algorithm(), key.Public())
			rootKeys[kid] = k
		}
	}
	root, err := data.NewRoot(rootKeys, rootRoles, consistent)
	if err != nil {
		return err
	}
	tr.Root = root
	return nil
}

func (tr *TufRepo) InitTargets() error {
	targets := data.NewTargets()
	tr.Targets[data.ValidRoles["targets"]] = targets
	return nil
}

func (tr *TufRepo) InitSnapshot() error {
	root, err := tr.Root.ToSigned()
	if err != nil {
		return err
	}
	targets, err := tr.Targets[data.ValidRoles["targets"]].ToSigned()
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

func (tr *TufRepo) InitTimestamp() error {
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
// the keys and roles in the KeyDB, and sets the TufRepo.Root field
// to the SignedRoot object.
func (tr *TufRepo) SetRoot(s *data.SignedRoot) error {
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
// and sets the TufRepo.Timestamp field.
func (tr *TufRepo) SetTimestamp(s *data.SignedTimestamp) error {
	tr.Timestamp = s
	return nil
}

// SetSnapshot parses the Signed object into a SignedSnapshots object
// and sets the TufRepo.Snapshot field.
func (tr *TufRepo) SetSnapshot(s *data.SignedSnapshot) error {
	tr.Snapshot = s
	return nil
}

// SetTargets parses the Signed object into a SignedTargets object,
// reads the delegated roles and keys into the KeyDB, and sets the
// SignedTargets object agaist the role in the TufRepo.Targets map.
func (tr *TufRepo) SetTargets(role string, s *data.SignedTargets) error {
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
func (tr TufRepo) TargetMeta(role, path string) *data.FileMeta {
	if t, ok := tr.Targets[role]; ok {
		if m, ok := t.Signed.Targets[path]; ok {
			return &m
		}
	}
	return nil
}

// TargetDelegations returns a slice of Roles that are valid publishers
// for the target path provided.
func (tr TufRepo) TargetDelegations(role, path, pathHex string) []*data.Role {
	if pathHex == "" {
		pathDigest := sha256.Sum256([]byte(path))
		pathHex = hex.EncodeToString(pathDigest[:])
	}
	roles := make([]*data.Role, 0)
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
func (tr TufRepo) FindTarget(path string) *data.FileMeta {
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

// AddTargets will attempt to add the given targets specifically to
// the directed role. If the user does not have the signing keys for the role
// the function will return an error and the full slice of targets.
func (tr *TufRepo) AddTargets(role string, targets data.Files) (data.Files, error) {
	t, ok := tr.Targets[role]
	if !ok {
		return targets, errors.ErrInvalidRole{role}
	}
	invalid := make(data.Files)
	for path, target := range targets {
		pathDigest := sha256.Sum256([]byte(path))
		pathHex := hex.EncodeToString(pathDigest[:])
		r := tr.keysDB.GetRole(role)
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

func (tr *TufRepo) RemoveTargets(role string, targets ...string) error {
	t, ok := tr.Targets[role]
	if !ok {
		return errors.ErrInvalidRole{role}
	}

	for _, path := range targets {
		delete(t.Signed.Targets, path)
	}
	t.Dirty = true
	return nil
}

func (tr *TufRepo) UpdateSnapshot(role string, s *data.Signed) error {
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

func (tr *TufRepo) UpdateTimestamp(s *data.Signed) error {
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

func (tr *TufRepo) SignRoot(expires time.Time, cryptoService signed.CryptoService) (*data.Signed, error) {
	logrus.Debug("signing root...")
	tr.Root.Signed.Expires = expires
	tr.Root.Signed.Version++
	root := tr.keysDB.GetRole(data.ValidRoles["root"])
	signed, err := tr.Root.ToSigned()
	if err != nil {
		return nil, err
	}
	signed, err = tr.sign(signed, *root, cryptoService)
	if err != nil {
		return nil, err
	}
	tr.Root.Signatures = signed.Signatures
	return signed, nil
}

func (tr *TufRepo) SignTargets(role string, expires time.Time, cryptoService signed.CryptoService) (*data.Signed, error) {
	logrus.Debugf("sign targets called for role %s", role)
	tr.Targets[role].Signed.Expires = expires
	tr.Targets[role].Signed.Version++
	signed, err := tr.Targets[role].ToSigned()
	if err != nil {
		logrus.Debug("errored getting targets data.Signed object")
		return nil, err
	}
	targets := tr.keysDB.GetRole(role)
	signed, err = tr.sign(signed, *targets, cryptoService)
	if err != nil {
		logrus.Debug("errored signing ", role)
		return nil, err
	}
	tr.Targets[role].Signatures = signed.Signatures
	return signed, nil
}

func (tr *TufRepo) SignSnapshot(expires time.Time, cryptoService signed.CryptoService) (*data.Signed, error) {
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
	}
	tr.Snapshot.Signed.Expires = expires
	tr.Snapshot.Signed.Version++
	signed, err := tr.Snapshot.ToSigned()
	if err != nil {
		return nil, err
	}
	snapshot := tr.keysDB.GetRole(data.ValidRoles["snapshot"])
	signed, err = tr.sign(signed, *snapshot, cryptoService)
	if err != nil {
		return nil, err
	}
	tr.Snapshot.Signatures = signed.Signatures
	return signed, nil
}

func (tr *TufRepo) SignTimestamp(expires time.Time, cryptoService signed.CryptoService) (*data.Signed, error) {
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
	signed, err = tr.sign(signed, *timestamp, cryptoService)
	if err != nil {
		return nil, err
	}
	tr.Timestamp.Signatures = signed.Signatures
	tr.Snapshot.Dirty = false // snapshot is dirty until changes have been captured in timestamp
	return signed, nil
}

func (tr TufRepo) sign(signedData *data.Signed, role data.Role, cryptoService signed.CryptoService) (*data.Signed, error) {
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
	if cryptoService == nil {
		cryptoService = tr.cryptoService
	}
	err := signed.Sign(cryptoService, signedData, ks...)
	if err != nil {
		return nil, err
	}
	return signedData, nil
}
