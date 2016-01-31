package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	"github.com/docker/notary/certs"
	"github.com/docker/notary/client/changelist"
	"github.com/docker/notary/cryptoservice"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf"
	tufclient "github.com/docker/notary/tuf/client"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/keys"
	"github.com/docker/notary/tuf/signed"
	"github.com/docker/notary/tuf/store"
)

const (
	maxSize = 5 << 20
)

func init() {
	data.SetDefaultExpiryTimes(
		map[string]int{
			"root":     3650,
			"targets":  1095,
			"snapshot": 1095,
		},
	)
}

// ErrRepoNotInitialized is returned when trying to publish an uninitialized
// notary repository
type ErrRepoNotInitialized struct{}

func (err ErrRepoNotInitialized) Error() string {
	return "repository has not been initialized"
}

// ErrInvalidRemoteRole is returned when the server is requested to manage
// an unsupported key type
type ErrInvalidRemoteRole struct {
	Role string
}

func (err ErrInvalidRemoteRole) Error() string {
	return fmt.Sprintf(
		"notary does not support the server managing the %s key", err.Role)
}

// ErrRepositoryNotExist is returned when an action is taken on a remote
// repository that doesn't exist
type ErrRepositoryNotExist struct {
	remote string
	gun    string
}

func (err ErrRepositoryNotExist) Error() string {
	return fmt.Sprintf("%s does not have trust data for %s", err.remote, err.gun)
}

const (
	tufDir = "tuf"
)

// NotaryRepository stores all the information needed to operate on a notary
// repository.
type NotaryRepository struct {
	baseDir       string
	gun           string
	baseURL       string
	tufRepoPath   string
	fileStore     store.MetadataStore
	CryptoService signed.CryptoService
	tufRepo       *tuf.Repo
	roundTrip     http.RoundTripper
	CertStore     trustmanager.X509Store
}

// repositoryFromKeystores is a helper function for NewNotaryRepository that
// takes some basic NotaryRepository parameters as well as keystores (in order
// of usage preference), and returns a NotaryRepository.
func repositoryFromKeystores(baseDir, gun, baseURL string, rt http.RoundTripper,
	keyStores []trustmanager.KeyStore) (*NotaryRepository, error) {

	certPath := filepath.Join(baseDir, notary.TrustedCertsDir)
	certStore, err := trustmanager.NewX509FilteredFileStore(
		certPath,
		trustmanager.FilterCertsExpiredSha1,
	)
	if err != nil {
		return nil, err
	}

	cryptoService := cryptoservice.NewCryptoService(gun, keyStores...)

	nRepo := &NotaryRepository{
		gun:           gun,
		baseDir:       baseDir,
		baseURL:       baseURL,
		tufRepoPath:   filepath.Join(baseDir, tufDir, filepath.FromSlash(gun)),
		CryptoService: cryptoService,
		roundTrip:     rt,
		CertStore:     certStore,
	}

	fileStore, err := store.NewFilesystemStore(
		nRepo.tufRepoPath,
		"metadata",
		"json",
		"",
	)
	if err != nil {
		return nil, err
	}
	nRepo.fileStore = fileStore

	return nRepo, nil
}

// Target represents a simplified version of the data TUF operates on, so external
// applications don't have to depend on tuf data types.
type Target struct {
	Name   string      // the name of the target
	Hashes data.Hashes // the hash of the target
	Length int64       // the size in bytes of the target
}

// TargetWithRole represents a Target that exists in a particular role - this is
// produced by ListTargets and GetTargetByName
type TargetWithRole struct {
	Target
	Role string
}

// NewTarget is a helper method that returns a Target
func NewTarget(targetName string, targetPath string) (*Target, error) {
	b, err := ioutil.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}

	meta, err := data.NewFileMeta(bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	return &Target{Name: targetName, Hashes: meta.Hashes, Length: meta.Length}, nil
}

// Initialize creates a new repository by using rootKey as the root Key for the
// TUF repository.
func (r *NotaryRepository) Initialize(rootKeyID string, serverManagedRoles ...string) error {
	privKey, _, err := r.CryptoService.GetPrivateKey(rootKeyID)
	if err != nil {
		return err
	}

	// currently we only support server managing timestamps and snapshots, and
	// nothing else - timestamps are always managed by the server, and implicit
	// (do not have to be passed in as part of `serverManagedRoles`, so that
	// the API of Initialize doesn't change).
	var serverManagesSnapshot bool
	locallyManagedKeys := []string{
		data.CanonicalTargetsRole,
		data.CanonicalSnapshotRole,
		// root is also locally managed, but that should have been created
		// already
	}
	remotelyManagedKeys := []string{data.CanonicalTimestampRole}
	for _, role := range serverManagedRoles {
		switch role {
		case data.CanonicalTimestampRole:
			continue // timestamp is already in the right place
		case data.CanonicalSnapshotRole:
			// because we put Snapshot last
			locallyManagedKeys = []string{data.CanonicalTargetsRole}
			remotelyManagedKeys = append(
				remotelyManagedKeys, data.CanonicalSnapshotRole)
			serverManagesSnapshot = true
		default:
			return ErrInvalidRemoteRole{Role: role}
		}
	}

	// Hard-coded policy: the generated certificate expires in 10 years.
	startTime := time.Now()
	rootCert, err := cryptoservice.GenerateCertificate(
		privKey, r.gun, startTime, startTime.AddDate(10, 0, 0))

	if err != nil {
		return err
	}
	r.CertStore.AddCert(rootCert)

	// The root key gets stored in the TUF metadata X509 encoded, linking
	// the tuf root.json to our X509 PKI.
	// If the key is RSA, we store it as type RSAx509, if it is ECDSA we store it
	// as ECDSAx509 to allow the gotuf verifiers to correctly decode the
	// key on verification of signatures.
	var rootKey data.PublicKey
	switch privKey.Algorithm() {
	case data.RSAKey:
		rootKey = data.NewRSAx509PublicKey(trustmanager.CertToPEM(rootCert))
	case data.ECDSAKey:
		rootKey = data.NewECDSAx509PublicKey(trustmanager.CertToPEM(rootCert))
	default:
		return fmt.Errorf("invalid format for root key: %s", privKey.Algorithm())
	}

	kdb := keys.NewDB()
	err = addKeyForRole(kdb, data.CanonicalRootRole, rootKey)
	if err != nil {
		return err
	}

	// we want to create all the local keys first so we don't have to
	// make unnecessary network calls
	for _, role := range locallyManagedKeys {
		// This is currently hardcoding the keys to ECDSA.
		key, err := r.CryptoService.Create(role, data.ECDSAKey)
		if err != nil {
			return err
		}
		if err := addKeyForRole(kdb, role, key); err != nil {
			return err
		}
	}
	for _, role := range remotelyManagedKeys {
		// This key is generated by the remote server.
		key, err := getRemoteKey(r.baseURL, r.gun, role, r.roundTrip)
		if err != nil {
			return err
		}
		logrus.Debugf("got remote %s %s key with keyID: %s",
			role, key.Algorithm(), key.ID())
		if err := addKeyForRole(kdb, role, key); err != nil {
			return err
		}
	}

	r.tufRepo = tuf.NewRepo(kdb, r.CryptoService)

	err = r.tufRepo.InitRoot(false)
	if err != nil {
		logrus.Debug("Error on InitRoot: ", err.Error())
		return err
	}
	_, err = r.tufRepo.InitTargets(data.CanonicalTargetsRole)
	if err != nil {
		logrus.Debug("Error on InitTargets: ", err.Error())
		return err
	}
	err = r.tufRepo.InitSnapshot()
	if err != nil {
		logrus.Debug("Error on InitSnapshot: ", err.Error())
		return err
	}

	return r.saveMetadata(serverManagesSnapshot)
}

// adds a TUF Change template to the given roles
func addChange(cl *changelist.FileChangelist, c changelist.Change, roles ...string) error {

	if len(roles) == 0 {
		roles = []string{data.CanonicalTargetsRole}
	}

	var changes []changelist.Change
	for _, role := range roles {
		// Ensure we can only add targets to the CanonicalTargetsRole,
		// or a Delegation role (which is <CanonicalTargetsRole>/something else)
		if role != data.CanonicalTargetsRole && !data.IsDelegation(role) {
			return data.ErrInvalidRole{
				Role:   role,
				Reason: "cannot add targets to this role",
			}
		}

		changes = append(changes, changelist.NewTufChange(
			c.Action(),
			role,
			c.Type(),
			c.Path(),
			c.Content(),
		))
	}

	for _, c := range changes {
		if err := cl.Add(c); err != nil {
			return err
		}
	}
	return nil
}

// AddDelegation creates a new changelist entry to add a delegation to the repository
// when the changelist gets applied at publish time.  This does not do any validation
// other than checking the name of the delegation to add - all that will happen
// at publish time.
func (r *NotaryRepository) AddDelegation(name string, threshold int,
	delegationKeys []data.PublicKey, paths []string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Adding delegation "%s" with threshold %d, and %d keys\n`,
		name, threshold, len(delegationKeys))

	tdJSON, err := json.Marshal(&changelist.TufDelegation{
		NewThreshold: threshold,
		AddKeys:      data.KeyList(delegationKeys),
		AddPaths:     paths,
	})
	if err != nil {
		return err
	}

	template := changelist.NewTufChange(
		changelist.ActionCreate,
		name,
		changelist.TypeTargetsDelegation,
		"", // no path
		tdJSON,
	)

	return addChange(cl, template, name)
}

// RemoveDelegation creates a new changelist entry to remove a delegation from
// the repository when the changelist gets applied at publish time.
// This does not validate that the delegation exists, since one might exist
// after applying all changes.
func (r *NotaryRepository) RemoveDelegation(name string, keyIDs, paths []string, removeAll bool) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	logrus.Debugf(`Removing delegation "%s"\n`, name)
	var template *changelist.TufChange

	// We use the Delete action only for force removal, Update is used for removing individual keys and paths
	if removeAll {
		template = changelist.NewTufChange(
			changelist.ActionDelete,
			name,
			changelist.TypeTargetsDelegation,
			"",  // no path
			nil, // deleting role, no data needed
		)

	} else {
		tdJSON, err := json.Marshal(&changelist.TufDelegation{
			RemoveKeys:  keyIDs,
			RemovePaths: paths,
		})
		if err != nil {
			return err
		}

		template = changelist.NewTufChange(
			changelist.ActionUpdate,
			name,
			changelist.TypeTargetsDelegation,
			"", // no path
			tdJSON,
		)
	}

	return addChange(cl, template, name)
}

// AddTarget creates new changelist entries to add a target to the given roles
// in the repository when the changelist gets applied at publish time.
// If roles are unspecified, the default role is "targets".
func (r *NotaryRepository) AddTarget(target *Target, roles ...string) error {

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()
	logrus.Debugf("Adding target \"%s\" with sha256 \"%x\" and size %d bytes.\n", target.Name, target.Hashes["sha256"], target.Length)

	meta := data.FileMeta{Length: target.Length, Hashes: target.Hashes}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	template := changelist.NewTufChange(
		changelist.ActionCreate, "", changelist.TypeTargetsTarget,
		target.Name, metaJSON)
	return addChange(cl, template, roles...)
}

// RemoveTarget creates new changelist entries to remove a target from the given
// roles in the repository when the changelist gets applied at publish time.
// If roles are unspecified, the default role is "target".
func (r *NotaryRepository) RemoveTarget(targetName string, roles ...string) error {

	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	logrus.Debugf("Removing target \"%s\"", targetName)
	template := changelist.NewTufChange(changelist.ActionDelete, "",
		changelist.TypeTargetsTarget, targetName, nil)
	return addChange(cl, template, roles...)
}

// ListTargets lists all targets for the current repository. The list of
// roles should be passed in order from highest to lowest priority.
// IMPORTANT: if you pass a set of roles such as [ "targets/a", "targets/x"
// "targets/a/b" ], even though "targets/a/b" is part of the "targets/a" subtree
// its entries will be strictly shadowed by those in other parts of the "targets/a"
// subtree and also the "targets/x" subtree, as we will defer parsing it until
// we explicitly reach it in our iteration of the provided list of roles.
func (r *NotaryRepository) ListTargets(roles ...string) ([]*TargetWithRole, error) {
	_, err := r.Update(false)
	if err != nil {
		return nil, err
	}

	if len(roles) == 0 {
		roles = []string{data.CanonicalTargetsRole}
	}
	targets := make(map[string]*TargetWithRole)
	for _, role := range roles {
		// we don't need to do anything special with removing role from
		// roles because listSubtree always processes role and only excludes
		// descendant delegations that appear in roles.
		r.listSubtree(targets, role, roles...)
	}

	var targetList []*TargetWithRole
	for _, v := range targets {
		targetList = append(targetList, v)
	}

	return targetList, nil
}

func (r *NotaryRepository) listSubtree(targets map[string]*TargetWithRole, role string, exclude ...string) {
	excl := make(map[string]bool)
	for _, r := range exclude {
		excl[r] = true
	}
	roles := []string{role}
	for len(roles) > 0 {
		role = roles[0]
		roles = roles[1:]
		tgts, ok := r.tufRepo.Targets[role]
		if !ok {
			// not every role has to exist
			continue
		}
		for name, meta := range tgts.Signed.Targets {
			if _, ok := targets[name]; !ok {
				targets[name] = &TargetWithRole{
					Target: Target{Name: name, Hashes: meta.Hashes, Length: meta.Length}, Role: role}
			}
		}
		for _, d := range tgts.Signed.Delegations.Roles {
			if !excl[d.Name] {
				roles = append(roles, d.Name)
			}
		}
	}
}

// GetTargetByName returns a target given a name. If no roles are passed
// it uses the targets role and does a search of the entire delegation
// graph, finding the first entry in a breadth first search of the delegations.
// If roles are passed, they should be passed in descending priority and
// the target entry found in the subtree of the highest priority role
// will be returned
// See the IMPORTANT section on ListTargets above. Those roles also apply here.
func (r *NotaryRepository) GetTargetByName(name string, roles ...string) (*TargetWithRole, error) {
	c, err := r.Update(false)
	if err != nil {
		return nil, err
	}

	if len(roles) == 0 {
		roles = append(roles, data.CanonicalTargetsRole)
	}
	for _, role := range roles {
		meta, foundRole := c.TargetMeta(role, name, roles...)
		if meta != nil {
			return &TargetWithRole{
				Target: Target{Name: name, Hashes: meta.Hashes, Length: meta.Length}, Role: foundRole}, nil
		}
	}
	return nil, fmt.Errorf("No trust data for %s", name)

}

// GetChangelist returns the list of the repository's unpublished changes
func (r *NotaryRepository) GetChangelist() (changelist.Changelist, error) {
	changelistDir := filepath.Join(r.tufRepoPath, "changelist")
	cl, err := changelist.NewFileChangelist(changelistDir)
	if err != nil {
		logrus.Debug("Error initializing changelist")
		return nil, err
	}
	return cl, nil
}

// GetDelegationRoles returns the keys and roles of the repository's delegations
func (r *NotaryRepository) GetDelegationRoles() ([]*data.Role, error) {
	// Update state of the repo to latest
	if _, err := r.Update(false); err != nil {
		return nil, err
	}

	// All top level delegations (ex: targets/level1) are stored exclusively in targets.json
	targets, ok := r.tufRepo.Targets[data.CanonicalTargetsRole]
	if !ok {
		return nil, store.ErrMetaNotFound{Resource: data.CanonicalTargetsRole}
	}

	allDelegations := targets.Signed.Delegations.Roles

	// make a copy for traversing nested delegations
	delegationsList := make([]*data.Role, len(allDelegations))
	copy(delegationsList, allDelegations)

	// Now traverse to lower level delegations (ex: targets/level1/level2)
	for len(delegationsList) > 0 {
		// Pop off first delegation to traverse
		delegation := delegationsList[0]
		delegationsList = delegationsList[1:]

		// Get metadata
		delegationMeta, ok := r.tufRepo.Targets[delegation.Name]
		// If we get an error, don't try to traverse further into this subtree because it doesn't exist or is malformed
		if !ok {
			continue
		}

		// Add nested delegations to return list and exploration list
		allDelegations = append(allDelegations, delegationMeta.Signed.Delegations.Roles...)
		delegationsList = append(delegationsList, delegationMeta.Signed.Delegations.Roles...)
	}
	return allDelegations, nil
}

// RoleWithSignatures is a Role with its associated signatures
type RoleWithSignatures struct {
	Signatures []data.Signature
	data.Role
}

// ListRoles returns a list of RoleWithSignatures objects for this repo
// This represents the latest metadata for each role in this repo
func (r *NotaryRepository) ListRoles() ([]RoleWithSignatures, error) {
	// Update to latest repo state
	_, err := r.Update(false)
	if err != nil {
		return nil, err
	}

	// Get all role info from our updated keysDB, can be empty
	roles := r.tufRepo.GetAllLoadedRoles()

	var roleWithSigs []RoleWithSignatures

	// Populate RoleWithSignatures with Role from keysDB and signatures from TUF metadata
	for _, role := range roles {
		roleWithSig := RoleWithSignatures{Role: *role, Signatures: nil}
		switch role.Name {
		case data.CanonicalRootRole:
			roleWithSig.Signatures = r.tufRepo.Root.Signatures
		case data.CanonicalTargetsRole:
			roleWithSig.Signatures = r.tufRepo.Targets[data.CanonicalTargetsRole].Signatures
		case data.CanonicalSnapshotRole:
			roleWithSig.Signatures = r.tufRepo.Snapshot.Signatures
		case data.CanonicalTimestampRole:
			roleWithSig.Signatures = r.tufRepo.Timestamp.Signatures
		default:
			// If the role isn't a delegation, we should error -- this is only possible if we have invalid keyDB state
			if !data.IsDelegation(role.Name) {
				return nil, data.ErrInvalidRole{Role: role.Name, Reason: "invalid role name"}
			}
			if _, ok := r.tufRepo.Targets[role.Name]; ok {
				// We'll only find a signature if we've published any targets with this delegation
				roleWithSig.Signatures = r.tufRepo.Targets[role.Name].Signatures
			}
		}
		roleWithSigs = append(roleWithSigs, roleWithSig)
	}
	return roleWithSigs, nil
}

// Publish pushes the local changes in signed material to the remote notary-server
// Conceptually it performs an operation similar to a `git rebase`
func (r *NotaryRepository) Publish() error {
	var initialPublish bool
	// update first before publishing
	_, err := r.Update(true)
	if err != nil {
		// If the remote is not aware of the repo, then this is being published
		// for the first time.  Try to load from disk instead for publishing.
		if _, ok := err.(ErrRepositoryNotExist); ok {
			err := r.bootstrapRepo()
			if err != nil {
				logrus.Debugf("Unable to load repository from local files: %s",
					err.Error())
				if _, ok := err.(store.ErrMetaNotFound); ok {
					return ErrRepoNotInitialized{}
				}
				return err
			}
			// Ensure we will push the initial root and targets file.  Either or
			// both of the root and targets may not be marked as Dirty, since
			// there may not be any changes that update them, so use a
			// different boolean.
			initialPublish = true
		} else {
			// We could not update, so we cannot publish.
			logrus.Error("Could not publish Repository: ", err.Error())
			return err
		}
	}

	cl, err := r.GetChangelist()
	if err != nil {
		return err
	}
	// apply the changelist to the repo
	err = applyChangelist(r.tufRepo, cl)
	if err != nil {
		logrus.Debug("Error applying changelist")
		return err
	}

	// these are the tuf files we will need to update, serialized as JSON before
	// we send anything to remote
	updatedFiles := make(map[string][]byte)

	// check if our root file is nearing expiry or dirty. Resign if it is.  If
	// root is not dirty but we are publishing for the first time, then just
	// publish the existing root we have.
	if nearExpiry(r.tufRepo.Root) || r.tufRepo.Root.Dirty {
		rootJSON, err := serializeCanonicalRole(r.tufRepo, data.CanonicalRootRole)
		if err != nil {
			return err
		}
		updatedFiles[data.CanonicalRootRole] = rootJSON
	} else if initialPublish {
		rootJSON, err := r.tufRepo.Root.MarshalJSON()
		if err != nil {
			return err
		}
		updatedFiles[data.CanonicalRootRole] = rootJSON
	}

	// iterate through all the targets files - if they are dirty, sign and update
	for roleName, roleObj := range r.tufRepo.Targets {
		if roleObj.Dirty || (roleName == data.CanonicalTargetsRole && initialPublish) {
			targetsJSON, err := serializeCanonicalRole(r.tufRepo, roleName)
			if err != nil {
				return err
			}
			updatedFiles[roleName] = targetsJSON
		}
	}

	// if we initialized the repo while designating the server as the snapshot
	// signer, then there won't be a snapshots file.  However, we might now
	// have a local key (if there was a rotation), so initialize one.
	if r.tufRepo.Snapshot == nil {
		if err := r.tufRepo.InitSnapshot(); err != nil {
			return err
		}
	}

	snapshotJSON, err := serializeCanonicalRole(
		r.tufRepo, data.CanonicalSnapshotRole)

	if err == nil {
		// Only update the snapshot if we've sucessfully signed it.
		updatedFiles[data.CanonicalSnapshotRole] = snapshotJSON
	} else if _, ok := err.(signed.ErrNoKeys); ok {
		// If signing fails due to us not having the snapshot key, then
		// assume the server is going to sign, and do not include any snapshot
		// data.
		logrus.Debugf("Client does not have the key to sign snapshot. " +
			"Assuming that server should sign the snapshot.")
	} else {
		logrus.Debugf("Client was unable to sign the snapshot: %s", err.Error())
		return err
	}

	remote, err := getRemoteStore(r.baseURL, r.gun, r.roundTrip)
	if err != nil {
		return err
	}

	err = remote.SetMultiMeta(updatedFiles)
	if err != nil {
		return err
	}
	err = cl.Clear("")
	if err != nil {
		// This is not a critical problem when only a single host is pushing
		// but will cause weird behaviour if changelist cleanup is failing
		// and there are multiple hosts writing to the repo.
		logrus.Warn("Unable to clear changelist. You may want to manually delete the folder ", filepath.Join(r.tufRepoPath, "changelist"))
	}
	return nil
}

// bootstrapRepo loads the repository from the local file system.  This attempts
// to load metadata for all roles.  Since server snapshots are supported,
// if the snapshot metadata fails to load, that's ok.
// This can also be unified with some cache reading tools from tuf/client.
// This assumes that bootstrapRepo is only used by Publish()
func (r *NotaryRepository) bootstrapRepo() error {
	kdb := keys.NewDB()
	tufRepo := tuf.NewRepo(kdb, r.CryptoService)

	logrus.Debugf("Loading trusted collection.")
	rootJSON, err := r.fileStore.GetMeta("root", 0)
	if err != nil {
		return err
	}
	root := &data.SignedRoot{}
	err = json.Unmarshal(rootJSON, root)
	if err != nil {
		return err
	}
	err = tufRepo.SetRoot(root)
	if err != nil {
		return err
	}
	targetsJSON, err := r.fileStore.GetMeta("targets", 0)
	if err != nil {
		return err
	}
	targets := &data.SignedTargets{}
	err = json.Unmarshal(targetsJSON, targets)
	if err != nil {
		return err
	}
	tufRepo.SetTargets("targets", targets)

	snapshotJSON, err := r.fileStore.GetMeta("snapshot", 0)
	if err == nil {
		snapshot := &data.SignedSnapshot{}
		err = json.Unmarshal(snapshotJSON, snapshot)
		if err != nil {
			return err
		}
		tufRepo.SetSnapshot(snapshot)
	} else if _, ok := err.(store.ErrMetaNotFound); !ok {
		return err
	}

	r.tufRepo = tufRepo

	return nil
}

func (r *NotaryRepository) saveMetadata(ignoreSnapshot bool) error {
	logrus.Debugf("Saving changes to Trusted Collection.")

	rootJSON, err := serializeCanonicalRole(r.tufRepo, data.CanonicalRootRole)
	if err != nil {
		return err
	}
	err = r.fileStore.SetMeta(data.CanonicalRootRole, rootJSON)
	if err != nil {
		return err
	}

	targetsToSave := make(map[string][]byte)
	for t := range r.tufRepo.Targets {
		signedTargets, err := r.tufRepo.SignTargets(t, data.DefaultExpires("targets"))
		if err != nil {
			return err
		}
		targetsJSON, err := json.Marshal(signedTargets)
		if err != nil {
			return err
		}
		targetsToSave[t] = targetsJSON
	}

	for role, blob := range targetsToSave {
		parentDir := filepath.Dir(role)
		os.MkdirAll(parentDir, 0755)
		r.fileStore.SetMeta(role, blob)
	}

	if ignoreSnapshot {
		return nil
	}

	snapshotJSON, err := serializeCanonicalRole(r.tufRepo, data.CanonicalSnapshotRole)
	if err != nil {
		return err
	}

	return r.fileStore.SetMeta(data.CanonicalSnapshotRole, snapshotJSON)
}

// returns a properly constructed ErrRepositoryNotExist error based on this
// repo's information
func (r *NotaryRepository) errRepositoryNotExist() error {
	host := r.baseURL
	parsed, err := url.Parse(r.baseURL)
	if err == nil {
		host = parsed.Host // try to exclude the scheme and any paths
	}
	return ErrRepositoryNotExist{remote: host, gun: r.gun}
}

// Update bootstraps a trust anchor (root.json) before updating all the
// metadata from the repo.
func (r *NotaryRepository) Update(forWrite bool) (*tufclient.Client, error) {
	c, err := r.bootstrapClient(forWrite)
	if err != nil {
		if _, ok := err.(store.ErrMetaNotFound); ok {
			return nil, r.errRepositoryNotExist()
		}
		return nil, err
	}
	err = c.Update()
	if err != nil {
		if notFound, ok := err.(store.ErrMetaNotFound); ok && notFound.Resource == data.CanonicalRootRole {
			return nil, r.errRepositoryNotExist()
		}
		return nil, err
	}
	return c, nil
}

// bootstrapClient attempts to bootstrap a root.json to be used as the trust
// anchor for a repository. The checkInitialized argument indicates whether
// we should always attempt to contact the server to determine if the repository
// is initialized or not. If set to true, we will always attempt to download
// and return an error if the remote repository errors.
func (r *NotaryRepository) bootstrapClient(checkInitialized bool) (*tufclient.Client, error) {
	var (
		rootJSON   []byte
		err        error
		signedRoot *data.SignedRoot
	)
	// try to read root from cache first. We will trust this root
	// until we detect a problem during update which will cause
	// us to download a new root and perform a rotation.
	rootJSON, cachedRootErr := r.fileStore.GetMeta("root", maxSize)

	if cachedRootErr == nil {
		signedRoot, cachedRootErr = r.validateRoot(rootJSON)
	}

	remote, remoteErr := getRemoteStore(r.baseURL, r.gun, r.roundTrip)
	if remoteErr != nil {
		logrus.Error(remoteErr)
	} else if cachedRootErr != nil || checkInitialized {
		// remoteErr was nil and we had a cachedRootErr (or are specifically
		// checking for initialization of the repo).

		// if remote store successfully set up, try and get root from remote
		tmpJSON, err := remote.GetMeta("root", maxSize)
		if err != nil {
			// we didn't have a root in cache and were unable to load one from
			// the server. Nothing we can do but error.
			return nil, err
		}
		if cachedRootErr != nil {
			// we always want to use the downloaded root if there was a cache
			// error.
			signedRoot, err = r.validateRoot(tmpJSON)
			if err != nil {
				return nil, err
			}

			err = r.fileStore.SetMeta("root", tmpJSON)
			if err != nil {
				// if we can't write cache we should still continue, just log error
				logrus.Errorf("could not save root to cache: %s", err.Error())
			}
		}
	}

	kdb := keys.NewDB()
	r.tufRepo = tuf.NewRepo(kdb, r.CryptoService)

	if signedRoot == nil {
		return nil, ErrRepoNotInitialized{}
	}

	err = r.tufRepo.SetRoot(signedRoot)
	if err != nil {
		return nil, err
	}

	return tufclient.NewClient(
		r.tufRepo,
		remote,
		kdb,
		r.fileStore,
	), nil
}

// validateRoot MUST only be used during bootstrapping. It will only validate
// signatures of the root based on known keys, not expiry or other metadata.
// This is so that an out of date root can be loaded to be used in a rotation
// should the TUF update process detect a problem.
func (r *NotaryRepository) validateRoot(rootJSON []byte) (*data.SignedRoot, error) {
	// can't just unmarshal into SignedRoot because validate root
	// needs the root.Signed field to still be []byte for signature
	// validation
	root := &data.Signed{}
	err := json.Unmarshal(rootJSON, root)
	if err != nil {
		return nil, err
	}

	err = certs.ValidateRoot(r.CertStore, root, r.gun)
	if err != nil {
		return nil, err
	}

	return data.RootFromSigned(root)
}

// RotateKey removes all existing keys associated with the role, and either
// creates and adds one new key or delegates managing the key to the server.
// These changes are staged in a changelist until publish is called.
func (r *NotaryRepository) RotateKey(role string, serverManagesKey bool) error {
	if role == data.CanonicalRootRole || role == data.CanonicalTimestampRole {
		return fmt.Errorf(
			"notary does not currently support rotating the %s key", role)
	}
	if serverManagesKey && role == data.CanonicalTargetsRole {
		return ErrInvalidRemoteRole{Role: data.CanonicalTargetsRole}
	}

	var (
		pubKey data.PublicKey
		err    error
	)
	if serverManagesKey {
		pubKey, err = getRemoteKey(r.baseURL, r.gun, role, r.roundTrip)
	} else {
		pubKey, err = r.CryptoService.Create(role, data.ECDSAKey)
	}
	if err != nil {
		return err
	}

	return r.rootFileKeyChange(role, changelist.ActionCreate, pubKey)
}

func (r *NotaryRepository) rootFileKeyChange(role, action string, key data.PublicKey) error {
	cl, err := changelist.NewFileChangelist(filepath.Join(r.tufRepoPath, "changelist"))
	if err != nil {
		return err
	}
	defer cl.Close()

	kl := make(data.KeyList, 0, 1)
	kl = append(kl, key)
	meta := changelist.TufRootData{
		RoleName: role,
		Keys:     kl,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	c := changelist.NewTufChange(
		action,
		changelist.ScopeRoot,
		changelist.TypeRootRole,
		role,
		metaJSON,
	)
	err = cl.Add(c)
	if err != nil {
		return err
	}
	return nil
}

// DeleteTrustData removes the trust data stored for this repo in the TUF cache and certificate store on the client side
func (r *NotaryRepository) DeleteTrustData() error {
	// Clear TUF files and cache
	if err := r.fileStore.RemoveAll(); err != nil {
		return fmt.Errorf("error clearing TUF repo data: %v", err)
	}
	r.tufRepo = tuf.NewRepo(nil, nil)
	// Clear certificates
	certificates, err := r.CertStore.GetCertificatesByCN(r.gun)
	if err != nil {
		// If there were no certificates to delete, we're done
		if _, ok := err.(*trustmanager.ErrNoCertificatesFound); ok {
			return nil
		}
		return fmt.Errorf("error retrieving certificates for %s: %v", r.gun, err)
	}
	for _, cert := range certificates {
		if err := r.CertStore.RemoveCert(cert); err != nil {
			return fmt.Errorf("error removing certificate: %v: %v", cert, err)
		}
	}
	return nil
}
