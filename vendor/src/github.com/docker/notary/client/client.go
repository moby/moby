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
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	"github.com/docker/notary/client/changelist"
	"github.com/docker/notary/cryptoservice"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/trustpinning"
	"github.com/docker/notary/tuf"
	tufclient "github.com/docker/notary/tuf/client"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
	"github.com/docker/notary/tuf/store"
	"github.com/docker/notary/tuf/utils"
)

func init() {
	data.SetDefaultExpiryTimes(notary.NotaryDefaultExpiries)
}

// ErrRepoNotInitialized is returned when trying to publish an uninitialized
// notary repository
type ErrRepoNotInitialized struct{}

func (err ErrRepoNotInitialized) Error() string {
	return "repository has not been initialized"
}

// ErrInvalidRemoteRole is returned when the server is requested to manage
// a key type that is not permitted
type ErrInvalidRemoteRole struct {
	Role string
}

func (err ErrInvalidRemoteRole) Error() string {
	return fmt.Sprintf(
		"notary does not permit the server managing the %s key", err.Role)
}

// ErrInvalidLocalRole is returned when the client wants to manage
// a key type that is not permitted
type ErrInvalidLocalRole struct {
	Role string
}

func (err ErrInvalidLocalRole) Error() string {
	return fmt.Sprintf(
		"notary does not permit the client managing the %s key", err.Role)
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
	trustPinning  trustpinning.TrustPinConfig
}

// repositoryFromKeystores is a helper function for NewNotaryRepository that
// takes some basic NotaryRepository parameters as well as keystores (in order
// of usage preference), and returns a NotaryRepository.
func repositoryFromKeystores(baseDir, gun, baseURL string, rt http.RoundTripper,
	keyStores []trustmanager.KeyStore, trustPin trustpinning.TrustPinConfig) (*NotaryRepository, error) {

	cryptoService := cryptoservice.NewCryptoService(keyStores...)

	nRepo := &NotaryRepository{
		gun:           gun,
		baseDir:       baseDir,
		baseURL:       baseURL,
		tufRepoPath:   filepath.Join(baseDir, tufDir, filepath.FromSlash(gun)),
		CryptoService: cryptoService,
		roundTrip:     rt,
		trustPinning:  trustPin,
	}

	fileStore, err := store.NewFilesystemStore(
		nRepo.tufRepoPath,
		"metadata",
		"json",
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

	meta, err := data.NewFileMeta(bytes.NewBuffer(b), data.NotaryDefaultHashes...)
	if err != nil {
		return nil, err
	}

	return &Target{Name: targetName, Hashes: meta.Hashes, Length: meta.Length}, nil
}

func rootCertKey(gun string, privKey data.PrivateKey) (data.PublicKey, error) {
	// Hard-coded policy: the generated certificate expires in 10 years.
	startTime := time.Now()
	cert, err := cryptoservice.GenerateCertificate(
		privKey, gun, startTime, startTime.Add(notary.Year*10))
	if err != nil {
		return nil, err
	}

	x509PublicKey := trustmanager.CertToKey(cert)
	if x509PublicKey == nil {
		return nil, fmt.Errorf(
			"cannot use regenerated certificate: format %s", cert.PublicKeyAlgorithm)
	}

	return x509PublicKey, nil
}

// Initialize creates a new repository by using rootKey as the root Key for the
// TUF repository. The server must be reachable (and is asked to generate a
// timestamp key and possibly other serverManagedRoles), but the created repository
// result is only stored on local disk, not published to the server. To do that,
// use r.Publish() eventually.
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

	rootKey, err := rootCertKey(r.gun, privKey)
	if err != nil {
		return err
	}

	var (
		rootRole = data.NewBaseRole(
			data.CanonicalRootRole,
			notary.MinThreshold,
			rootKey,
		)
		timestampRole data.BaseRole
		snapshotRole  data.BaseRole
		targetsRole   data.BaseRole
	)

	// we want to create all the local keys first so we don't have to
	// make unnecessary network calls
	for _, role := range locallyManagedKeys {
		// This is currently hardcoding the keys to ECDSA.
		key, err := r.CryptoService.Create(role, r.gun, data.ECDSAKey)
		if err != nil {
			return err
		}
		switch role {
		case data.CanonicalSnapshotRole:
			snapshotRole = data.NewBaseRole(
				role,
				notary.MinThreshold,
				key,
			)
		case data.CanonicalTargetsRole:
			targetsRole = data.NewBaseRole(
				role,
				notary.MinThreshold,
				key,
			)
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
		switch role {
		case data.CanonicalSnapshotRole:
			snapshotRole = data.NewBaseRole(
				role,
				notary.MinThreshold,
				key,
			)
		case data.CanonicalTimestampRole:
			timestampRole = data.NewBaseRole(
				role,
				notary.MinThreshold,
				key,
			)
		}
	}

	r.tufRepo = tuf.NewRepo(r.CryptoService)

	err = r.tufRepo.InitRoot(
		rootRole,
		timestampRole,
		snapshotRole,
		targetsRole,
		false,
	)
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

// AddTarget creates new changelist entries to add a target to the given roles
// in the repository when the changelist gets applied at publish time.
// If roles are unspecified, the default role is "targets"
func (r *NotaryRepository) AddTarget(target *Target, roles ...string) error {

	if len(target.Hashes) == 0 {
		return fmt.Errorf("no hashes specified for target \"%s\"", target.Name)
	}
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
	if err := r.Update(false); err != nil {
		return nil, err
	}

	if len(roles) == 0 {
		roles = []string{data.CanonicalTargetsRole}
	}
	targets := make(map[string]*TargetWithRole)
	for _, role := range roles {
		// Define an array of roles to skip for this walk (see IMPORTANT comment above)
		skipRoles := utils.StrSliceRemove(roles, role)

		// Define a visitor function to populate the targets map in priority order
		listVisitorFunc := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
			// We found targets so we should try to add them to our targets map
			for targetName, targetMeta := range tgt.Signed.Targets {
				// Follow the priority by not overriding previously set targets
				// and check that this path is valid with this role
				if _, ok := targets[targetName]; ok || !validRole.CheckPaths(targetName) {
					continue
				}
				targets[targetName] =
					&TargetWithRole{Target: Target{Name: targetName, Hashes: targetMeta.Hashes, Length: targetMeta.Length}, Role: validRole.Name}
			}
			return nil
		}
		r.tufRepo.WalkTargets("", role, listVisitorFunc, skipRoles...)
	}

	var targetList []*TargetWithRole
	for _, v := range targets {
		targetList = append(targetList, v)
	}

	return targetList, nil
}

// GetTargetByName returns a target by the given name. If no roles are passed
// it uses the targets role and does a search of the entire delegation
// graph, finding the first entry in a breadth first search of the delegations.
// If roles are passed, they should be passed in descending priority and
// the target entry found in the subtree of the highest priority role
// will be returned.
// See the IMPORTANT section on ListTargets above. Those roles also apply here.
func (r *NotaryRepository) GetTargetByName(name string, roles ...string) (*TargetWithRole, error) {
	if err := r.Update(false); err != nil {
		return nil, err
	}

	if len(roles) == 0 {
		roles = append(roles, data.CanonicalTargetsRole)
	}
	var resultMeta data.FileMeta
	var resultRoleName string
	var foundTarget bool
	for _, role := range roles {
		// Define an array of roles to skip for this walk (see IMPORTANT comment above)
		skipRoles := utils.StrSliceRemove(roles, role)

		// Define a visitor function to find the specified target
		getTargetVisitorFunc := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
			if tgt == nil {
				return nil
			}
			// We found the target and validated path compatibility in our walk,
			// so we should stop our walk and set the resultMeta and resultRoleName variables
			if resultMeta, foundTarget = tgt.Signed.Targets[name]; foundTarget {
				resultRoleName = validRole.Name
				return tuf.StopWalk{}
			}
			return nil
		}
		// Check that we didn't error, and that we assigned to our target
		if err := r.tufRepo.WalkTargets(name, role, getTargetVisitorFunc, skipRoles...); err == nil && foundTarget {
			return &TargetWithRole{Target: Target{Name: name, Hashes: resultMeta.Hashes, Length: resultMeta.Length}, Role: resultRoleName}, nil
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

// RoleWithSignatures is a Role with its associated signatures
type RoleWithSignatures struct {
	Signatures []data.Signature
	data.Role
}

// ListRoles returns a list of RoleWithSignatures objects for this repo
// This represents the latest metadata for each role in this repo
func (r *NotaryRepository) ListRoles() ([]RoleWithSignatures, error) {
	// Update to latest repo state
	if err := r.Update(false); err != nil {
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
			if !data.IsDelegation(role.Name) {
				continue
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
	cl, err := r.GetChangelist()
	if err != nil {
		return err
	}
	if err = r.publish(cl); err != nil {
		return err
	}
	if err = cl.Clear(""); err != nil {
		// This is not a critical problem when only a single host is pushing
		// but will cause weird behaviour if changelist cleanup is failing
		// and there are multiple hosts writing to the repo.
		logrus.Warn("Unable to clear changelist. You may want to manually delete the folder ", filepath.Join(r.tufRepoPath, "changelist"))
	}
	return nil
}

// publish pushes the changes in the given changelist to the remote notary-server
// Conceptually it performs an operation similar to a `git rebase`
func (r *NotaryRepository) publish(cl changelist.Changelist) error {
	var initialPublish bool
	// update first before publishing
	if err := r.Update(true); err != nil {
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
			logrus.Error("Could not publish Repository since we could not update: ", err.Error())
			return err
		}
	}
	// apply the changelist to the repo
	if err := applyChangelist(r.tufRepo, cl); err != nil {
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
		// Only update the snapshot if we've successfully signed it.
		updatedFiles[data.CanonicalSnapshotRole] = snapshotJSON
	} else if signErr, ok := err.(signed.ErrInsufficientSignatures); ok && signErr.FoundKeys == 0 {
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

	return remote.SetMultiMeta(updatedFiles)
}

// bootstrapRepo loads the repository from the local file system (i.e.
// a not yet published repo or a possibly obsolete local copy) into
// r.tufRepo.  This attempts to load metadata for all roles.  Since server
// snapshots are supported, if the snapshot metadata fails to load, that's ok.
// This assumes that bootstrapRepo is only used by Publish() or RotateKey()
func (r *NotaryRepository) bootstrapRepo() error {
	b := tuf.NewRepoBuilder(r.gun, r.CryptoService, r.trustPinning)

	logrus.Debugf("Loading trusted collection.")

	for _, role := range data.BaseRoles {
		jsonBytes, err := r.fileStore.GetMeta(role, store.NoSizeLimit)
		if err != nil {
			if _, ok := err.(store.ErrMetaNotFound); ok &&
				// server snapshots are supported, and server timestamp management
				// is required, so if either of these fail to load that's ok - especially
				// if the repo is new
				role == data.CanonicalSnapshotRole || role == data.CanonicalTimestampRole {
				continue
			}
			return err
		}
		if err := b.Load(role, jsonBytes, 1, true); err != nil {
			return err
		}
	}

	tufRepo, err := b.Finish()
	if err == nil {
		r.tufRepo = tufRepo
	}
	return nil
}

// saveMetadata saves contents of r.tufRepo onto the local disk, creating
// signatures as necessary, possibly prompting for passphrases.
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
		signedTargets, err := r.tufRepo.SignTargets(t, data.DefaultExpires(data.CanonicalTargetsRole))
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
func (r *NotaryRepository) Update(forWrite bool) error {
	c, err := r.bootstrapClient(forWrite)
	if err != nil {
		if _, ok := err.(store.ErrMetaNotFound); ok {
			return r.errRepositoryNotExist()
		}
		return err
	}
	repo, err := c.Update()
	if err != nil {
		// notFound.Resource may include a checksum so when the role is root,
		// it will be root or root.<checksum>. Therefore best we can
		// do it match a "root." prefix
		if notFound, ok := err.(store.ErrMetaNotFound); ok && strings.HasPrefix(notFound.Resource, data.CanonicalRootRole+".") {
			return r.errRepositoryNotExist()
		}
		return err
	}
	r.tufRepo = repo
	return nil
}

// bootstrapClient attempts to bootstrap a root.json to be used as the trust
// anchor for a repository. The checkInitialized argument indicates whether
// we should always attempt to contact the server to determine if the repository
// is initialized or not. If set to true, we will always attempt to download
// and return an error if the remote repository errors.
//
// Populates a tuf.RepoBuilder with this root metadata (only use
// tufclient.Client.Update to load the rest).
//
// Fails if the remote server is reachable and does not know the repo
// (i.e. before the first r.Publish()), in which case the error is
// store.ErrMetaNotFound, or if the root metadata (from whichever source is used)
// is not trusted.
//
// Returns a tufclient.Client for the remote server, which may not be actually
// operational (if the URL is invalid but a root.json is cached).
func (r *NotaryRepository) bootstrapClient(checkInitialized bool) (*tufclient.Client, error) {
	minVersion := 1
	// the old root on disk should not be validated against any trust pinning configuration
	// because if we have an old root, it itself is the thing that pins trust
	oldBuilder := tuf.NewRepoBuilder(r.gun, r.CryptoService, trustpinning.TrustPinConfig{})

	// by default, we want to use the trust pinning configuration on any new root that we download
	newBuilder := tuf.NewRepoBuilder(r.gun, r.CryptoService, r.trustPinning)

	// Try to read root from cache first. We will trust this root until we detect a problem
	// during update which will cause us to download a new root and perform a rotation.
	// If we have an old root, and it's valid, then we overwrite the newBuilder to be one
	// preloaded with the old root or one which uses the old root for trust bootstrapping.
	if rootJSON, err := r.fileStore.GetMeta(data.CanonicalRootRole, store.NoSizeLimit); err == nil {
		// if we can't load the cached root, fail hard because that is how we pin trust
		if err := oldBuilder.Load(data.CanonicalRootRole, rootJSON, minVersion, true); err != nil {
			return nil, err
		}

		// again, the root on disk is the source of trust pinning, so use an empty trust
		// pinning configuration
		newBuilder = tuf.NewRepoBuilder(r.gun, r.CryptoService, trustpinning.TrustPinConfig{})

		if err := newBuilder.Load(data.CanonicalRootRole, rootJSON, minVersion, false); err != nil {
			// Ok, the old root is expired - we want to download a new one.  But we want to use the
			// old root to verify the new root, so bootstrap a new builder with the old builder
			minVersion = oldBuilder.GetLoadedVersion(data.CanonicalRootRole)
			newBuilder = oldBuilder.BootstrapNewBuilder()
		}
	}

	remote, remoteErr := getRemoteStore(r.baseURL, r.gun, r.roundTrip)
	if remoteErr != nil {
		logrus.Error(remoteErr)
	} else if !newBuilder.IsLoaded(data.CanonicalRootRole) || checkInitialized {
		// remoteErr was nil and we were not able to load a root from cache or
		// are specifically checking for initialization of the repo.

		// if remote store successfully set up, try and get root from remote
		// We don't have any local data to determine the size of root, so try the maximum (though it is restricted at 100MB)
		tmpJSON, err := remote.GetMeta(data.CanonicalRootRole, store.NoSizeLimit)
		if err != nil {
			// we didn't have a root in cache and were unable to load one from
			// the server. Nothing we can do but error.
			return nil, err
		}

		if !newBuilder.IsLoaded(data.CanonicalRootRole) {
			// we always want to use the downloaded root if we couldn't load from cache
			if err := newBuilder.Load(data.CanonicalRootRole, tmpJSON, minVersion, false); err != nil {
				return nil, err
			}

			err = r.fileStore.SetMeta(data.CanonicalRootRole, tmpJSON)
			if err != nil {
				// if we can't write cache we should still continue, just log error
				logrus.Errorf("could not save root to cache: %s", err.Error())
			}
		}
	}

	// We can only get here if remoteErr != nil (hence we don't download any new root),
	// and there was no root on disk
	if !newBuilder.IsLoaded(data.CanonicalRootRole) {
		return nil, ErrRepoNotInitialized{}
	}

	return tufclient.NewClient(oldBuilder, newBuilder, remote, r.fileStore), nil
}

// RotateKey removes all existing keys associated with the role, and either
// creates and adds one new key or delegates managing the key to the server.
// These changes are staged in a changelist until publish is called.
func (r *NotaryRepository) RotateKey(role string, serverManagesKey bool) error {
	// We currently support remotely managing timestamp and snapshot keys
	canBeRemoteKey := role == data.CanonicalTimestampRole || role == data.CanonicalSnapshotRole
	// And locally managing root, targets, and snapshot keys
	canBeLocalKey := (role == data.CanonicalSnapshotRole || role == data.CanonicalTargetsRole ||
		role == data.CanonicalRootRole)

	switch {
	case !data.ValidRole(role) || data.IsDelegation(role):
		return fmt.Errorf("notary does not currently permit rotating the %s key", role)
	case serverManagesKey && !canBeRemoteKey:
		return ErrInvalidRemoteRole{Role: role}
	case !serverManagesKey && !canBeLocalKey:
		return ErrInvalidLocalRole{Role: role}
	}

	var (
		pubKey    data.PublicKey
		err       error
		errFmtMsg string
	)
	switch serverManagesKey {
	case true:
		pubKey, err = getRemoteKey(r.baseURL, r.gun, role, r.roundTrip)
		errFmtMsg = "unable to rotate remote key: %s"
	default:
		pubKey, err = r.CryptoService.Create(role, r.gun, data.ECDSAKey)
		errFmtMsg = "unable to generate key: %s"
	}

	if err != nil {
		return fmt.Errorf(errFmtMsg, err)
	}

	// if this is a root role, generate a root cert for the public key
	if role == data.CanonicalRootRole {
		privKey, _, err := r.CryptoService.GetPrivateKey(pubKey.ID())
		if err != nil {
			return err
		}
		pubKey, err = rootCertKey(r.gun, privKey)
		if err != nil {
			return err
		}
	}

	cl := changelist.NewMemChangelist()
	if err := r.rootFileKeyChange(cl, role, changelist.ActionCreate, pubKey); err != nil {
		return err
	}
	return r.publish(cl)
}

func (r *NotaryRepository) rootFileKeyChange(cl changelist.Changelist, role, action string, key data.PublicKey) error {
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
	return cl.Add(c)
}

// DeleteTrustData removes the trust data stored for this repo in the TUF cache on the client side
func (r *NotaryRepository) DeleteTrustData() error {
	// Clear TUF files and cache
	if err := r.fileStore.RemoveAll(); err != nil {
		return fmt.Errorf("error clearing TUF repo data: %v", err)
	}
	r.tufRepo = tuf.NewRepo(nil)
	return nil
}
