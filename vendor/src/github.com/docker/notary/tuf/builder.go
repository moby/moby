package tuf

import (
	"fmt"

	"github.com/docker/go/canonical/json"
	"github.com/docker/notary"

	"github.com/docker/notary/trustpinning"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
	"github.com/docker/notary/tuf/utils"
)

// ErrBuildDone is returned when any functions are called on RepoBuilder, and it
// is already finished building
var ErrBuildDone = fmt.Errorf(
	"the builder has finished building and cannot accept any more input or produce any more output")

// ErrInvalidBuilderInput is returned when RepoBuilder.Load is called
// with the wrong type of metadata for the state that it's in
type ErrInvalidBuilderInput struct{ msg string }

func (e ErrInvalidBuilderInput) Error() string {
	return e.msg
}

// ConsistentInfo is the consistent name and size of a role, or just the name
// of the role and a -1 if no file metadata for the role is known
type ConsistentInfo struct {
	RoleName string
	fileMeta data.FileMeta
}

// ChecksumKnown determines whether or not we know enough to provide a size and
// consistent name
func (c ConsistentInfo) ChecksumKnown() bool {
	// empty hash, no size : this is the zero value
	return len(c.fileMeta.Hashes) > 0 || c.fileMeta.Length != 0
}

// ConsistentName returns the consistent name (rolename.sha256) for the role
// given this consistent information
func (c ConsistentInfo) ConsistentName() string {
	return utils.ConsistentName(c.RoleName, c.fileMeta.Hashes[notary.SHA256])
}

// Length returns the expected length of the role as per this consistent
// information - if no checksum information is known, the size is -1.
func (c ConsistentInfo) Length() int64 {
	if c.ChecksumKnown() {
		return c.fileMeta.Length
	}
	return -1
}

// RepoBuilder is an interface for an object which builds a tuf.Repo
type RepoBuilder interface {
	Load(roleName string, content []byte, minVersion int, allowExpired bool) error
	GenerateSnapshot(prev *data.SignedSnapshot) ([]byte, int, error)
	GenerateTimestamp(prev *data.SignedTimestamp) ([]byte, int, error)
	Finish() (*Repo, *Repo, error)
	BootstrapNewBuilder() RepoBuilder
	BootstrapNewBuilderWithNewTrustpin(trustpin trustpinning.TrustPinConfig) RepoBuilder

	// informative functions
	IsLoaded(roleName string) bool
	GetLoadedVersion(roleName string) int
	GetConsistentInfo(roleName string) ConsistentInfo
}

// finishedBuilder refuses any more input or output
type finishedBuilder struct{}

func (f finishedBuilder) Load(roleName string, content []byte, minVersion int, allowExpired bool) error {
	return ErrBuildDone
}
func (f finishedBuilder) GenerateSnapshot(prev *data.SignedSnapshot) ([]byte, int, error) {
	return nil, 0, ErrBuildDone
}
func (f finishedBuilder) GenerateTimestamp(prev *data.SignedTimestamp) ([]byte, int, error) {
	return nil, 0, ErrBuildDone
}
func (f finishedBuilder) Finish() (*Repo, *Repo, error)    { return nil, nil, ErrBuildDone }
func (f finishedBuilder) BootstrapNewBuilder() RepoBuilder { return f }
func (f finishedBuilder) BootstrapNewBuilderWithNewTrustpin(trustpin trustpinning.TrustPinConfig) RepoBuilder {
	return f
}
func (f finishedBuilder) IsLoaded(roleName string) bool        { return false }
func (f finishedBuilder) GetLoadedVersion(roleName string) int { return 0 }
func (f finishedBuilder) GetConsistentInfo(roleName string) ConsistentInfo {
	return ConsistentInfo{RoleName: roleName}
}

// NewRepoBuilder is the only way to get a pre-built RepoBuilder
func NewRepoBuilder(gun string, cs signed.CryptoService, trustpin trustpinning.TrustPinConfig) RepoBuilder {
	return NewBuilderFromRepo(gun, NewRepo(cs), trustpin)
}

// NewBuilderFromRepo allows us to bootstrap a builder given existing repo data.
// YOU PROBABLY SHOULDN'T BE USING THIS OUTSIDE OF TESTING CODE!!!
func NewBuilderFromRepo(gun string, repo *Repo, trustpin trustpinning.TrustPinConfig) RepoBuilder {
	return &repoBuilderWrapper{
		RepoBuilder: &repoBuilder{
			repo:                 repo,
			invalidRoles:         NewRepo(nil),
			gun:                  gun,
			trustpin:             trustpin,
			loadedNotChecksummed: make(map[string][]byte),
		},
	}
}

// repoBuilderWrapper embeds a repoBuilder, but once Finish is called, swaps
// the embed out with a finishedBuilder
type repoBuilderWrapper struct {
	RepoBuilder
}

func (rbw *repoBuilderWrapper) Finish() (*Repo, *Repo, error) {
	switch rbw.RepoBuilder.(type) {
	case finishedBuilder:
		return rbw.RepoBuilder.Finish()
	default:
		old := rbw.RepoBuilder
		rbw.RepoBuilder = finishedBuilder{}
		return old.Finish()
	}
}

// repoBuilder actually builds a tuf.Repo
type repoBuilder struct {
	repo         *Repo
	invalidRoles *Repo

	// needed for root trust pininng verification
	gun      string
	trustpin trustpinning.TrustPinConfig

	// in case we load root and/or targets before snapshot and timestamp (
	// or snapshot and not timestamp), so we know what to verify when the
	// data with checksums come in
	loadedNotChecksummed map[string][]byte

	// bootstrapped values to validate a new root
	prevRoot                 *data.SignedRoot
	bootstrappedRootChecksum *data.FileMeta

	// for bootstrapping the next builder
	nextRootChecksum *data.FileMeta
}

func (rb *repoBuilder) Finish() (*Repo, *Repo, error) {
	return rb.repo, rb.invalidRoles, nil
}

func (rb *repoBuilder) BootstrapNewBuilder() RepoBuilder {
	return &repoBuilderWrapper{RepoBuilder: &repoBuilder{
		repo:                 NewRepo(rb.repo.cryptoService),
		invalidRoles:         NewRepo(nil),
		gun:                  rb.gun,
		loadedNotChecksummed: make(map[string][]byte),
		trustpin:             rb.trustpin,

		prevRoot:                 rb.repo.Root,
		bootstrappedRootChecksum: rb.nextRootChecksum,
	}}
}

func (rb *repoBuilder) BootstrapNewBuilderWithNewTrustpin(trustpin trustpinning.TrustPinConfig) RepoBuilder {
	return &repoBuilderWrapper{RepoBuilder: &repoBuilder{
		repo:                 NewRepo(rb.repo.cryptoService),
		gun:                  rb.gun,
		loadedNotChecksummed: make(map[string][]byte),
		trustpin:             trustpin,

		prevRoot:                 rb.repo.Root,
		bootstrappedRootChecksum: rb.nextRootChecksum,
	}}
}

// IsLoaded returns whether a particular role has already been loaded
func (rb *repoBuilder) IsLoaded(roleName string) bool {
	switch roleName {
	case data.CanonicalRootRole:
		return rb.repo.Root != nil
	case data.CanonicalSnapshotRole:
		return rb.repo.Snapshot != nil
	case data.CanonicalTimestampRole:
		return rb.repo.Timestamp != nil
	default:
		return rb.repo.Targets[roleName] != nil
	}
}

// GetLoadedVersion returns the metadata version, if it is loaded, or 1 (the
// minimum valid version number) otherwise
func (rb *repoBuilder) GetLoadedVersion(roleName string) int {
	switch {
	case roleName == data.CanonicalRootRole && rb.repo.Root != nil:
		return rb.repo.Root.Signed.Version
	case roleName == data.CanonicalSnapshotRole && rb.repo.Snapshot != nil:
		return rb.repo.Snapshot.Signed.Version
	case roleName == data.CanonicalTimestampRole && rb.repo.Timestamp != nil:
		return rb.repo.Timestamp.Signed.Version
	default:
		if tgts, ok := rb.repo.Targets[roleName]; ok {
			return tgts.Signed.Version
		}
	}

	return 1
}

// GetConsistentInfo returns the consistent name and size of a role, if it is known,
// otherwise just the rolename and a -1 for size (both of which are inside a
// ConsistentInfo object)
func (rb *repoBuilder) GetConsistentInfo(roleName string) ConsistentInfo {
	info := ConsistentInfo{RoleName: roleName} // starts out with unknown filemeta
	switch roleName {
	case data.CanonicalTimestampRole:
		// we do not want to get a consistent timestamp, but we do want to
		// limit its size
		info.fileMeta.Length = notary.MaxTimestampSize
	case data.CanonicalSnapshotRole:
		if rb.repo.Timestamp != nil {
			info.fileMeta = rb.repo.Timestamp.Signed.Meta[roleName]
		}
	case data.CanonicalRootRole:
		switch {
		case rb.bootstrappedRootChecksum != nil:
			info.fileMeta = *rb.bootstrappedRootChecksum
		case rb.repo.Snapshot != nil:
			info.fileMeta = rb.repo.Snapshot.Signed.Meta[roleName]
		}
	default:
		if rb.repo.Snapshot != nil {
			info.fileMeta = rb.repo.Snapshot.Signed.Meta[roleName]
		}
	}
	return info
}

func (rb *repoBuilder) Load(roleName string, content []byte, minVersion int, allowExpired bool) error {
	if !data.ValidRole(roleName) {
		return ErrInvalidBuilderInput{msg: fmt.Sprintf("%s is an invalid role", roleName)}
	}

	if rb.IsLoaded(roleName) {
		return ErrInvalidBuilderInput{msg: fmt.Sprintf("%s has already been loaded", roleName)}
	}

	var err error
	switch roleName {
	case data.CanonicalRootRole:
		break
	case data.CanonicalTimestampRole, data.CanonicalSnapshotRole, data.CanonicalTargetsRole:
		err = rb.checkPrereqsLoaded([]string{data.CanonicalRootRole})
	default: // delegations
		err = rb.checkPrereqsLoaded([]string{data.CanonicalRootRole, data.CanonicalTargetsRole})
	}
	if err != nil {
		return err
	}

	switch roleName {
	case data.CanonicalRootRole:
		return rb.loadRoot(content, minVersion, allowExpired)
	case data.CanonicalSnapshotRole:
		return rb.loadSnapshot(content, minVersion, allowExpired)
	case data.CanonicalTimestampRole:
		return rb.loadTimestamp(content, minVersion, allowExpired)
	case data.CanonicalTargetsRole:
		return rb.loadTargets(content, minVersion, allowExpired)
	default:
		return rb.loadDelegation(roleName, content, minVersion, allowExpired)
	}
}

func (rb *repoBuilder) checkPrereqsLoaded(prereqRoles []string) error {
	for _, req := range prereqRoles {
		if !rb.IsLoaded(req) {
			return ErrInvalidBuilderInput{msg: fmt.Sprintf("%s must be loaded first", req)}
		}
	}
	return nil
}

// GenerateSnapshot generates a new snapshot given a previous (optional) snapshot
// We can't just load the previous snapshot, because it may have been signed by a different
// snapshot key (maybe from a previous root version).  Note that we need the root role and
// targets role to be loaded, because we need to generate metadata for both (and we need
// the root to be loaded so we can get the snapshot role to sign with)
func (rb *repoBuilder) GenerateSnapshot(prev *data.SignedSnapshot) ([]byte, int, error) {
	switch {
	case rb.repo.cryptoService == nil:
		return nil, 0, ErrInvalidBuilderInput{msg: "cannot generate snapshot without a cryptoservice"}
	case rb.IsLoaded(data.CanonicalSnapshotRole):
		return nil, 0, ErrInvalidBuilderInput{msg: "snapshot has already been loaded"}
	case rb.IsLoaded(data.CanonicalTimestampRole):
		return nil, 0, ErrInvalidBuilderInput{msg: "cannot generate snapshot if timestamp has already been loaded"}
	}

	if err := rb.checkPrereqsLoaded([]string{data.CanonicalRootRole}); err != nil {
		return nil, 0, err
	}

	// If there is no previous snapshot, we need to generate one, and so the targets must
	// have already been loaded.  Otherwise, so long as the previous snapshot structure is
	// valid (it has a targets meta), we're good.
	switch prev {
	case nil:
		if err := rb.checkPrereqsLoaded([]string{data.CanonicalTargetsRole}); err != nil {
			return nil, 0, err
		}

		if err := rb.repo.InitSnapshot(); err != nil {
			rb.repo.Snapshot = nil
			return nil, 0, err
		}
	default:
		if err := data.IsValidSnapshotStructure(prev.Signed); err != nil {
			return nil, 0, err
		}
		rb.repo.Snapshot = prev
	}

	sgnd, err := rb.repo.SignSnapshot(data.DefaultExpires(data.CanonicalSnapshotRole))
	if err != nil {
		rb.repo.Snapshot = nil
		return nil, 0, err
	}

	sgndJSON, err := json.Marshal(sgnd)
	if err != nil {
		rb.repo.Snapshot = nil
		return nil, 0, err
	}

	// loadedNotChecksummed should currently contain the root awaiting checksumming,
	// since it has to have been loaded.  Since the snapshot was generated using
	// the root and targets data (there may not be any) that that have been loaded,
	// remove all of them from rb.loadedNotChecksummed
	for tgtName := range rb.repo.Targets {
		delete(rb.loadedNotChecksummed, tgtName)
	}
	delete(rb.loadedNotChecksummed, data.CanonicalRootRole)

	// The timestamp can't have been loaded yet, so we want to cache the snapshot
	// bytes so we can validate the checksum when a timestamp gets generated or
	// loaded later.
	rb.loadedNotChecksummed[data.CanonicalSnapshotRole] = sgndJSON

	return sgndJSON, rb.repo.Snapshot.Signed.Version, nil
}

// GenerateTimestamp generates a new timestamp given a previous (optional) timestamp
// We can't just load the previous timestamp, because it may have been signed by a different
// timestamp key (maybe from a previous root version)
func (rb *repoBuilder) GenerateTimestamp(prev *data.SignedTimestamp) ([]byte, int, error) {
	switch {
	case rb.repo.cryptoService == nil:
		return nil, 0, ErrInvalidBuilderInput{msg: "cannot generate timestamp without a cryptoservice"}
	case rb.IsLoaded(data.CanonicalTimestampRole):
		return nil, 0, ErrInvalidBuilderInput{msg: "timestamp has already been loaded"}
	}

	// SignTimestamp always serializes the loaded snapshot and signs in the data, so we must always
	// have the snapshot loaded first
	if err := rb.checkPrereqsLoaded([]string{data.CanonicalRootRole, data.CanonicalSnapshotRole}); err != nil {
		return nil, 0, err
	}

	switch prev {
	case nil:
		if err := rb.repo.InitTimestamp(); err != nil {
			rb.repo.Timestamp = nil
			return nil, 0, err
		}
	default:
		if err := data.IsValidTimestampStructure(prev.Signed); err != nil {
			return nil, 0, err
		}
		rb.repo.Timestamp = prev
	}

	sgnd, err := rb.repo.SignTimestamp(data.DefaultExpires(data.CanonicalTimestampRole))
	if err != nil {
		rb.repo.Timestamp = nil
		return nil, 0, err
	}

	sgndJSON, err := json.Marshal(sgnd)
	if err != nil {
		rb.repo.Timestamp = nil
		return nil, 0, err
	}

	// The snapshot should have been loaded (and not checksummed, since a timestamp
	// cannot have been loaded), so it is awaiting checksumming. Since this
	// timestamp was generated using the snapshot awaiting checksumming, we can
	// remove it from rb.loadedNotChecksummed. There should be no other items
	// awaiting checksumming now since loading/generating a snapshot should have
	// cleared out everything else in `loadNotChecksummed`.
	delete(rb.loadedNotChecksummed, data.CanonicalSnapshotRole)

	return sgndJSON, rb.repo.Timestamp.Signed.Version, nil
}

// loadRoot loads a root if one has not been loaded
func (rb *repoBuilder) loadRoot(content []byte, minVersion int, allowExpired bool) error {
	roleName := data.CanonicalRootRole

	signedObj, err := rb.bytesToSigned(content, data.CanonicalRootRole)
	if err != nil {
		return err
	}
	// ValidateRoot validates against the previous root's role, as well as validates that the root
	// itself is self-consistent with its own signatures and thresholds.
	// This assumes that ValidateRoot calls data.RootFromSigned, which validates
	// the metadata, rather than just unmarshalling signedObject into a SignedRoot object itself.
	signedRoot, err := trustpinning.ValidateRoot(rb.prevRoot, signedObj, rb.gun, rb.trustpin)
	if err != nil {
		return err
	}

	if err := signed.VerifyVersion(&(signedRoot.Signed.SignedCommon), minVersion); err != nil {
		return err
	}

	if !allowExpired { // check must go at the end because all other validation should pass
		if err := signed.VerifyExpiry(&(signedRoot.Signed.SignedCommon), roleName); err != nil {
			return err
		}
	}

	rootRole, err := signedRoot.BuildBaseRole(data.CanonicalRootRole)
	if err != nil { // this should never happen since the root has been validated
		return err
	}
	rb.repo.Root = signedRoot
	rb.repo.originalRootRole = rootRole
	return nil
}

func (rb *repoBuilder) loadTimestamp(content []byte, minVersion int, allowExpired bool) error {
	roleName := data.CanonicalTimestampRole

	timestampRole, err := rb.repo.Root.BuildBaseRole(roleName)
	if err != nil { // this should never happen, since it's already been validated
		return err
	}

	signedObj, err := rb.bytesToSignedAndValidateSigs(timestampRole, content)
	if err != nil {
		return err
	}

	signedTimestamp, err := data.TimestampFromSigned(signedObj)
	if err != nil {
		return err
	}

	if err := signed.VerifyVersion(&(signedTimestamp.Signed.SignedCommon), minVersion); err != nil {
		return err
	}

	if !allowExpired { // check must go at the end because all other validation should pass
		if err := signed.VerifyExpiry(&(signedTimestamp.Signed.SignedCommon), roleName); err != nil {
			return err
		}
	}

	if err := rb.validateChecksumsFromTimestamp(signedTimestamp); err != nil {
		return err
	}

	rb.repo.Timestamp = signedTimestamp
	return nil
}

func (rb *repoBuilder) loadSnapshot(content []byte, minVersion int, allowExpired bool) error {
	roleName := data.CanonicalSnapshotRole

	snapshotRole, err := rb.repo.Root.BuildBaseRole(roleName)
	if err != nil { // this should never happen, since it's already been validated
		return err
	}

	signedObj, err := rb.bytesToSignedAndValidateSigs(snapshotRole, content)
	if err != nil {
		return err
	}

	signedSnapshot, err := data.SnapshotFromSigned(signedObj)
	if err != nil {
		return err
	}

	if err := signed.VerifyVersion(&(signedSnapshot.Signed.SignedCommon), minVersion); err != nil {
		return err
	}

	if !allowExpired { // check must go at the end because all other validation should pass
		if err := signed.VerifyExpiry(&(signedSnapshot.Signed.SignedCommon), roleName); err != nil {
			return err
		}
	}

	// at this point, the only thing left to validate is existing checksums - we can use
	// this snapshot to bootstrap the next builder if needed - and we don't need to do
	// the 2-value assignment since we've already validated the signedSnapshot, which MUST
	// have root metadata
	rootMeta := signedSnapshot.Signed.Meta[data.CanonicalRootRole]
	rb.nextRootChecksum = &rootMeta

	if err := rb.validateChecksumsFromSnapshot(signedSnapshot); err != nil {
		return err
	}

	rb.repo.Snapshot = signedSnapshot
	return nil
}

func (rb *repoBuilder) loadTargets(content []byte, minVersion int, allowExpired bool) error {
	roleName := data.CanonicalTargetsRole

	targetsRole, err := rb.repo.Root.BuildBaseRole(roleName)
	if err != nil { // this should never happen, since it's already been validated
		return err
	}

	signedObj, err := rb.bytesToSignedAndValidateSigs(targetsRole, content)
	if err != nil {
		return err
	}

	signedTargets, err := data.TargetsFromSigned(signedObj, roleName)
	if err != nil {
		return err
	}

	if err := signed.VerifyVersion(&(signedTargets.Signed.SignedCommon), minVersion); err != nil {
		return err
	}

	if !allowExpired { // check must go at the end because all other validation should pass
		if err := signed.VerifyExpiry(&(signedTargets.Signed.SignedCommon), roleName); err != nil {
			return err
		}
	}

	signedTargets.Signatures = signedObj.Signatures
	rb.repo.Targets[roleName] = signedTargets
	return nil
}

func (rb *repoBuilder) loadDelegation(roleName string, content []byte, minVersion int, allowExpired bool) error {
	delegationRole, err := rb.repo.GetDelegationRole(roleName)
	if err != nil {
		return err
	}

	// bytesToSigned checks checksum
	signedObj, err := rb.bytesToSigned(content, roleName)
	if err != nil {
		return err
	}

	signedTargets, err := data.TargetsFromSigned(signedObj, roleName)
	if err != nil {
		return err
	}

	if err := signed.VerifyVersion(&(signedTargets.Signed.SignedCommon), minVersion); err != nil {
		// don't capture in invalidRoles because the role we received is a rollback
		return err
	}

	// verify signature
	if err := signed.VerifySignatures(signedObj, delegationRole.BaseRole); err != nil {
		rb.invalidRoles.Targets[roleName] = signedTargets
		return err
	}

	if !allowExpired { // check must go at the end because all other validation should pass
		if err := signed.VerifyExpiry(&(signedTargets.Signed.SignedCommon), roleName); err != nil {
			rb.invalidRoles.Targets[roleName] = signedTargets
			return err
		}
	}

	signedTargets.Signatures = signedObj.Signatures
	rb.repo.Targets[roleName] = signedTargets
	return nil
}

func (rb *repoBuilder) validateChecksumsFromTimestamp(ts *data.SignedTimestamp) error {
	sn, ok := rb.loadedNotChecksummed[data.CanonicalSnapshotRole]
	if ok {
		// by this point, the SignedTimestamp has been validated so it must have a snapshot hash
		snMeta := ts.Signed.Meta[data.CanonicalSnapshotRole].Hashes
		if err := data.CheckHashes(sn, data.CanonicalSnapshotRole, snMeta); err != nil {
			return err
		}
		delete(rb.loadedNotChecksummed, data.CanonicalSnapshotRole)
	}
	return nil
}

func (rb *repoBuilder) validateChecksumsFromSnapshot(sn *data.SignedSnapshot) error {
	var goodRoles []string
	for roleName, loadedBytes := range rb.loadedNotChecksummed {
		switch roleName {
		case data.CanonicalSnapshotRole, data.CanonicalTimestampRole:
			break
		default:
			if err := data.CheckHashes(loadedBytes, roleName, sn.Signed.Meta[roleName].Hashes); err != nil {
				return err
			}
			goodRoles = append(goodRoles, roleName)
		}
	}
	for _, roleName := range goodRoles {
		delete(rb.loadedNotChecksummed, roleName)
	}
	return nil
}

func (rb *repoBuilder) validateChecksumFor(content []byte, roleName string) error {
	// validate the bootstrap checksum for root, if provided
	if roleName == data.CanonicalRootRole && rb.bootstrappedRootChecksum != nil {
		if err := data.CheckHashes(content, roleName, rb.bootstrappedRootChecksum.Hashes); err != nil {
			return err
		}
	}

	// but we also want to cache the root content, so that when the snapshot is
	// loaded it is validated (to make sure everything in the repo is self-consistent)
	checksums := rb.getChecksumsFor(roleName)
	if checksums != nil { // as opposed to empty, in which case hash check should fail
		if err := data.CheckHashes(content, roleName, *checksums); err != nil {
			return err
		}
	} else if roleName != data.CanonicalTimestampRole {
		// timestamp is the only role which does not need to be checksummed, but
		// for everything else, cache the contents in the list of roles that have
		// not been checksummed by the snapshot/timestamp yet
		rb.loadedNotChecksummed[roleName] = content
	}

	return nil
}

// Checksums the given bytes, and if they validate, convert to a data.Signed object.
// If a checksums are nil (as opposed to empty), adds the bytes to the list of roles that
// haven't been checksummed (unless it's a timestamp, which has no checksum reference).
func (rb *repoBuilder) bytesToSigned(content []byte, roleName string) (*data.Signed, error) {
	if err := rb.validateChecksumFor(content, roleName); err != nil {
		return nil, err
	}

	// unmarshal to signed
	signedObj := &data.Signed{}
	if err := json.Unmarshal(content, signedObj); err != nil {
		return nil, err
	}

	return signedObj, nil
}

func (rb *repoBuilder) bytesToSignedAndValidateSigs(role data.BaseRole, content []byte) (*data.Signed, error) {

	signedObj, err := rb.bytesToSigned(content, role.Name)
	if err != nil {
		return nil, err
	}

	// verify signature
	if err := signed.VerifySignatures(signedObj, role); err != nil {
		return nil, err
	}

	return signedObj, nil
}

// If the checksum reference (the loaded timestamp for the snapshot role, and
// the loaded snapshot for every other role except timestamp and snapshot) is nil,
// then return nil for the checksums, meaning that the checksum is not yet
// available.  If the checksum reference *is* loaded, then always returns the
// Hashes object for the given role - if it doesn't exist, returns an empty Hash
// object (against which any checksum validation would fail).
func (rb *repoBuilder) getChecksumsFor(role string) *data.Hashes {
	var hashes data.Hashes
	switch role {
	case data.CanonicalTimestampRole:
		return nil
	case data.CanonicalSnapshotRole:
		if rb.repo.Timestamp == nil {
			return nil
		}
		hashes = rb.repo.Timestamp.Signed.Meta[data.CanonicalSnapshotRole].Hashes
	default:
		if rb.repo.Snapshot == nil {
			return nil
		}
		hashes = rb.repo.Snapshot.Signed.Meta[role].Hashes
	}
	return &hashes
}
