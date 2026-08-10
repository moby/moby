//go:build windows
// +build windows

package cimfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/winapi"
	winapitypes "github.com/Microsoft/hcsshim/internal/winapi/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// CimFsWriter represents a writer to a single CimFS filesystem instance. On disk, the
// image is composed of a filesystem file and several object ID and region files.
// Note: The CimFsWriter isn't thread safe!
type CimFsWriter struct {
	// name of this cim. Usually a <name>.cim file will be created to represent this cim.
	name string
	// handle is the CIMFS_IMAGE_HANDLE that must be passed when calling CIMFS APIs.
	handle winapitypes.FsHandle
	// name of the active file i.e the file to which we are currently writing.
	activeName string
	// stream to currently active file.
	activeStream winapitypes.StreamHandle
	// amount of bytes that can be written to the activeStream.
	activeLeft uint64
	// if true the CIM will be sealed after the writer is closed.
	sealOnClose bool
}

// Create creates a new cim image. The CimFsWriter returned can then be used to do
// operations on this cim.  If `oldFSName` is provided the new image is "forked" from the
// CIM with name `oldFSName` located under `imagePath`.
func Create(imagePath string, oldFSName string, newFSName string) (_ *CimFsWriter, err error) {
	var oldNameBytes *uint16
	// CimCreateImage API call has different behavior if the value of oldNameBytes / newNameBytes
	// is empty than if it is nil. So we have to convert those strings into *uint16 here.
	fsName := oldFSName
	if oldFSName != "" {
		oldNameBytes, err = windows.UTF16PtrFromString(oldFSName)
		if err != nil {
			return nil, err
		}
	}
	var newNameBytes *uint16
	if newFSName != "" {
		fsName = newFSName
		newNameBytes, err = windows.UTF16PtrFromString(newFSName)
		if err != nil {
			return nil, err
		}
	}
	var handle winapitypes.FsHandle
	if err := winapi.CimCreateImage(imagePath, oldNameBytes, newNameBytes, &handle); err != nil {
		return nil, fmt.Errorf("failed to create cim image at path %s, oldName: %s, newName: %s: %w", imagePath, oldFSName, newFSName, err)
	}
	return &CimFsWriter{handle: handle, name: filepath.Join(imagePath, fsName)}, nil
}

// blockCIMConfig represents options for creating or merging block CIMs
type blockCIMConfig struct {
	// ensures that the generted CIM is identical every time when created from the same source data.
	// This is mostly required for image layers. Dissabled by default.
	consistentCIM bool
	// enables data integrity checking, which means the CIM will be verified and sealed on close.
	// This is useful for ensuring that the CIM is tamper-proof. Disabled by default.
	dataIntegrity bool
}

// BlockCIMOpt is a function type for configuring block CIM creation options
type BlockCIMOpt func(*blockCIMConfig) error

// enabled consistent CIM creation, this ensures that CIMs created from identical source data  will always be identical (i.e. SHA256 digest of the CIM will remain same)
func WithConsistentCIM() BlockCIMOpt {
	return func(opts *blockCIMConfig) error {
		opts.consistentCIM = true
		return nil
	}
}

// WithDataIntegrity enables data integrity checking (verified CIM with sealing on close)
func WithDataIntegrity() BlockCIMOpt {
	return func(opts *blockCIMConfig) error {
		opts.dataIntegrity = true
		return nil
	}
}

// CreateBlockCIMWithOptions creates a new block CIM with the specified options and opens it for writing.
// The CimFsWriter returned can then be used to add/remove files to/from this CIM.
func CreateBlockCIMWithOptions(ctx context.Context, bCIM *BlockCIM, options ...BlockCIMOpt) (_ *CimFsWriter, err error) {
	// Apply default options
	config := &blockCIMConfig{}

	// Apply provided options
	for _, option := range options {
		if err = option(config); err != nil {
			return nil, err
		}
	}

	// Validate options
	if bCIM.BlockPath == "" || bCIM.CimName == "" {
		return nil, fmt.Errorf("both blockPath & name must be non empty: %w", os.ErrInvalid)
	}

	if bCIM.Type == BlockCIMTypeNone {
		return nil, fmt.Errorf("invalid block CIM type `%d`: %w", bCIM.Type, os.ErrInvalid)
	}

	// Check OS support
	if !IsBlockCimWriteSupported() {
		return nil, fmt.Errorf("block CIM not supported on this OS version")
	}

	if config.dataIntegrity && !IsVerifiedCimWriteSupported() {
		return nil, fmt.Errorf("verified CIMs are not supported on this OS version")
	}

	// Build create flags based on options
	var createFlags uint32
	if config.consistentCIM {
		createFlags |= CimCreateFlagConsistentCim
	}
	if config.dataIntegrity {
		createFlags |= CimCreateFlagVerifiedCim
	}

	switch bCIM.Type {
	case BlockCIMTypeDevice:
		createFlags |= CimCreateFlagBlockDeviceCim
	case BlockCIMTypeSingleFile:
		createFlags |= CimCreateFlagSingleFileCim
	default:
		return nil, fmt.Errorf("invalid block CIM type `%d`: %w", bCIM.Type, os.ErrInvalid)
	}

	winapi.LogCimDLLSupport()

	var newNameUTF16 *uint16
	newNameUTF16, err = windows.UTF16PtrFromString(bCIM.CimName)
	if err != nil {
		return nil, err
	}

	var handle winapitypes.FsHandle
	if err := winapi.CimCreateImage2(bCIM.BlockPath, createFlags, nil, newNameUTF16, &handle); err != nil {
		return nil, fmt.Errorf("failed to create block CIM at path %s,%s: %w", bCIM.BlockPath, bCIM.CimName, err)
	}

	return &CimFsWriter{
		handle:      handle,
		name:        filepath.Join(bCIM.BlockPath, bCIM.CimName),
		sealOnClose: config.dataIntegrity, // Seal on close if data integrity is enabled
	}, nil
}

// Create creates a new block CIM and opens it for writing. The CimFsWriter
// returned can then be used to add/remove files to/from this CIM.
func CreateBlockCIM(blockPath, name string, blockType BlockCIMType) (_ *CimFsWriter, err error) {
	return CreateBlockCIMWithOptions(context.Background(), &BlockCIM{
		Type:      blockType,
		BlockPath: blockPath,
		CimName:   name,
	}, WithConsistentCIM())
}

// CreateAlternateStream creates alternate stream of given size at the given path inside the cim. This will
// replace the current active stream. Always, finish writing current active stream and then create an
// alternate stream.
func (c *CimFsWriter) CreateAlternateStream(path string, size uint64) (err error) {
	err = c.closeStream()
	if err != nil {
		return err
	}
	err = winapi.CimCreateAlternateStream(c.handle, path, size, &c.activeStream)
	if err != nil {
		return fmt.Errorf("failed to create alternate stream for path %s: %w", path, err)
	}
	c.activeName = path
	return nil
}

// closes the currently active stream.
func (c *CimFsWriter) closeStream() error {
	if c.activeStream == 0 {
		return nil
	}
	err := winapi.CimCloseStream(c.activeStream)
	if err == nil && c.activeLeft > 0 {
		// Validate here because CimCloseStream does not and this improves error
		// reporting. Otherwise the error will occur in the context of
		// cimWriteStream.
		err = fmt.Errorf("incomplete write, %d bytes left in the stream %s", c.activeLeft, c.activeName)
	}
	if err != nil {
		err = &PathError{Cim: c.name, Op: "closeStream", Path: c.activeName, Err: err}
	}
	c.activeLeft = 0
	c.activeStream = 0
	c.activeName = ""
	return err
}

// AddFile adds a new file to the image. The file is added at the specified path. After
// calling this function, the file is set as the active stream for the image, so data can
// be written by calling `Write`.
func (c *CimFsWriter) AddFile(path string, info *winio.FileBasicInfo, fileSize int64, securityDescriptor []byte, extendedAttributes []byte, reparseData []byte) error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	fileMetadata := &winapitypes.CimFsFileMetadata{
		Attributes:     info.FileAttributes,
		FileSize:       fileSize,
		CreationTime:   info.CreationTime,
		LastWriteTime:  info.LastWriteTime,
		ChangeTime:     info.ChangeTime,
		LastAccessTime: info.LastAccessTime,
	}
	if len(securityDescriptor) == 0 {
		// Passing an empty security descriptor creates a CIM in a weird state.
		// Pass the NULL DACL.
		securityDescriptor = nullSd
	}
	fileMetadata.SecurityDescriptorBuffer = unsafe.Pointer(&securityDescriptor[0])
	fileMetadata.SecurityDescriptorSize = uint32(len(securityDescriptor))
	if len(reparseData) > 0 {
		fileMetadata.ReparseDataBuffer = unsafe.Pointer(&reparseData[0])
		fileMetadata.ReparseDataSize = uint32(len(reparseData))
	}
	if len(extendedAttributes) > 0 {
		fileMetadata.ExtendedAttributes = unsafe.Pointer(&extendedAttributes[0])
		fileMetadata.EACount = uint32(len(extendedAttributes))
	}
	// remove the trailing `\` if present, otherwise it trips off the cim writer
	path = strings.TrimSuffix(path, "\\")
	err = winapi.CimCreateFile(c.handle, path, fileMetadata, &c.activeStream)
	if err != nil {
		return &PathError{Cim: c.name, Op: "addFile", Path: path, Err: err}
	}
	c.activeName = path
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY) == 0 {
		c.activeLeft = uint64(fileSize)
	}
	return nil
}

// Write writes bytes to the active stream.
func (c *CimFsWriter) Write(p []byte) (int, error) {
	if c.activeStream == 0 {
		return 0, fmt.Errorf("no active stream")
	}
	if uint64(len(p)) > c.activeLeft {
		return 0, &PathError{Cim: c.name, Op: "write", Path: c.activeName, Err: fmt.Errorf("wrote too much")}
	}
	err := winapi.CimWriteStream(c.activeStream, uintptr(unsafe.Pointer(&p[0])), uint32(len(p)))
	if err != nil {
		err = &PathError{Cim: c.name, Op: "write", Path: c.activeName, Err: err}
		return 0, err
	}
	c.activeLeft -= uint64(len(p))
	return len(p), nil
}

// AddLink adds a hard link at `newPath` that points to `oldPath`.
func (c *CimFsWriter) AddLink(oldPath string, newPath string) error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	err = winapi.CimCreateHardLink(c.handle, newPath, oldPath)
	if err != nil {
		err = &LinkError{Cim: c.name, Op: "addLink", Old: oldPath, New: newPath, Err: err}
	}
	return err
}

// AddMergedLink adds a hard link at `newPath` that points to `oldPath` in the
// image. However unlike AddLink this link is resolved at merge time. This allows us to
// create links to files that are in other CIMs.
func (c *CimFsWriter) AddMergedLink(oldPath string, newPath string) error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	err = winapi.CimCreateMergeLink(c.handle, newPath, oldPath)
	if err != nil {
		err = &LinkError{Cim: c.name, Op: "addMergedLink", Old: oldPath, New: newPath, Err: err}
	}
	return err
}

// Unlink deletes the file at `path` from the image. Note that the file MUST have been
// already added to the image.
func (c *CimFsWriter) Unlink(path string) error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	return winapi.CimDeletePath(c.handle, path)
}

// Adds a tombstone at given path. This ensures that when the the CIMs are merged, the
// file at this path from lower layers won't show up in a mounted CIM. In case of Unlink,
// the file from the lower layers still shows up after merge.
func (c *CimFsWriter) AddTombstone(path string) error {
	err := c.closeStream()
	if err != nil {
		return err
	}

	return winapi.CimTombstoneFile(c.handle, path)
}

func (c *CimFsWriter) commit() error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	err = winapi.CimCommitImage(c.handle)
	if err != nil {
		err = &OpError{Cim: c.name, Op: "commit", Err: err}
	}
	return err
}

// Close closes the CimFS filesystem.
func (c *CimFsWriter) Close() (err error) {
	if c.handle == 0 {
		return fmt.Errorf("invalid writer")
	}
	if err = c.commit(); err != nil {
		return &OpError{Cim: c.name, Op: "commit", Err: err}
	}
	err = winapi.CimCloseImage(c.handle)
	c.handle = 0
	if err != nil {
		return &OpError{Cim: c.name, Op: "close", Err: err}
	}
	if c.sealOnClose {
		if err = sealBlockCIM(filepath.Dir(c.name)); err != nil {
			return &OpError{Cim: filepath.Dir(c.name), Op: "seal", Err: err}
		}
	}
	return nil
}

// DestroyCim finds out the region files, object files of this cim and then delete the
// region files, object files and the <layer-id>.cim file itself.  Note that any other
// CIMs that were forked off of this CIM would become unusable after this operation.  This
// should not be used for block CIMs, os.Remove is sufficient for block CIMs.
func DestroyCim(ctx context.Context, cimPath string) (retErr error) {
	regionFilePaths, err := getRegionFilePaths(ctx, cimPath)
	if err != nil {
		log.G(ctx).WithError(err).Warnf("get region files for cim %s", cimPath)
		if retErr == nil { //nolint:govet // nilness: consistency with below
			retErr = err
		}
	}
	objectFilePaths, err := getObjectIDFilePaths(ctx, cimPath)
	if err != nil {
		log.G(ctx).WithError(err).Warnf("get objectid file for cim %s", cimPath)
		if retErr == nil {
			retErr = err
		}
	}

	log.G(ctx).WithFields(logrus.Fields{
		"cimPath":     cimPath,
		"regionFiles": regionFilePaths,
		"objectFiles": objectFilePaths,
	}).Debug("destroy cim")

	for _, regFilePath := range regionFilePaths {
		if err := os.Remove(regFilePath); err != nil {
			log.G(ctx).WithError(err).Warnf("remove file %s", regFilePath)
			if retErr == nil {
				retErr = err
			}
		}
	}

	for _, objFilePath := range objectFilePaths {
		if err := os.Remove(objFilePath); err != nil {
			log.G(ctx).WithError(err).Warnf("remove file %s", objFilePath)
			if retErr == nil {
				retErr = err
			}
		}
	}

	if err := os.Remove(cimPath); err != nil {
		log.G(ctx).WithError(err).Warnf("remove file %s", cimPath)
		if retErr == nil {
			retErr = err
		}
	}
	return retErr
}

// GetCimUsage returns the total disk usage in bytes by the cim at path `cimPath`.
func GetCimUsage(ctx context.Context, cimPath string) (uint64, error) {
	regionFilePaths, err := getRegionFilePaths(ctx, cimPath)
	if err != nil {
		return 0, fmt.Errorf("get region file paths for cim %s: %w", cimPath, err)
	}
	objectFilePaths, err := getObjectIDFilePaths(ctx, cimPath)
	if err != nil {
		return 0, fmt.Errorf("get objectid file for cim %s: %w", cimPath, err)
	}

	var totalUsage uint64
	for _, f := range append(regionFilePaths, objectFilePaths...) {
		fi, err := os.Stat(f)
		if err != nil {
			return 0, fmt.Errorf("stat file %s: %w", f, err)
		}
		totalUsage += uint64(fi.Size())
	}
	return totalUsage, nil
}

// MergeBlockCIMs creates a new merged BlockCIM from the provided source BlockCIMs.  CIM
// at index 0 is considered to be topmost CIM and the CIM at index `length-1` is
// considered the base CIM. (i.e file with the same path in CIM at index 0 will shadow
// files with the same path at all other CIMs) When mounting this merged CIM the source
// CIMs MUST be provided in the exact same order.
func MergeBlockCIMsWithOpts(ctx context.Context, mergedCIM *BlockCIM, sourceCIMs []*BlockCIM, opts ...BlockCIMOpt) (err error) {
	if !IsMergedCimWriteSupported() {
		return fmt.Errorf("merged CIMs aren't supported on this OS version")
	} else if len(sourceCIMs) < 2 {
		return fmt.Errorf("need at least 2 source CIMs, got %d: %w", len(sourceCIMs), os.ErrInvalid)
	}

	// Apply default options
	config := &blockCIMConfig{}

	// Apply provided options
	for _, opt := range opts {
		if err = opt(config); err != nil {
			return err
		}
	}

	for _, sCIM := range sourceCIMs {
		if sCIM.Type != mergedCIM.Type {
			return fmt.Errorf("source CIM (%s) type MUST match with merged CIM type: %w", sCIM.String(), os.ErrInvalid)
		}
	}

	var mergeFlags uint32
	switch mergedCIM.Type {
	case BlockCIMTypeDevice:
		mergeFlags = CimMergeFlagBlockDevice
	case BlockCIMTypeSingleFile:
		mergeFlags = CimMergeFlagSingleFile
	default:
		return fmt.Errorf("invalid block CIM type `%d`: %w", mergedCIM.Type, os.ErrInvalid)
	}

	if config.dataIntegrity {
		mergeFlags |= CimMergeFlagVerifiedCim
	}

	cim, err := CreateBlockCIMWithOptions(ctx, mergedCIM, opts...)
	if err != nil {
		return fmt.Errorf("create merged CIM: %w", err)
	}
	defer func() {
		cErr := cim.Close()
		if err == nil {
			err = cErr
		}
	}()

	// CimAddFsToMergedImage expects that topmost CIM is added first and the bottom
	// most CIM is added last.
	for _, sCIM := range sourceCIMs {
		fullPath := filepath.Join(sCIM.BlockPath, sCIM.CimName)
		if err := winapi.CimAddFsToMergedImage2(cim.handle, fullPath, mergeFlags); err != nil {
			return fmt.Errorf("add cim to merged image: %w", err)
		}
	}
	return nil
}

// MergeBlockCIMs creates a new merged BlockCIM from the provided source BlockCIMs.  CIM
// at index 0 is considered to be topmost CIM and the CIM at index `length-1` is
// considered the base CIM. (i.e file with the same path in CIM at index 0 will shadow
// files with the same path at all other CIMs) When mounting this merged CIM the source
// CIMs MUST be provided in the exact same order.
func MergeBlockCIMs(mergedCIM *BlockCIM, sourceCIMs []*BlockCIM) (err error) {
	return MergeBlockCIMsWithOpts(context.Background(), mergedCIM, sourceCIMs, WithConsistentCIM())
}

// sealBlockCIM seals a blockCIM at the given path so that no further modifications are allowed on it. This also writes a
// root hash in the block header so that in future any reads happening on the CIM can be easily verified against this root hash
// to detect tampering.
func sealBlockCIM(blockPath string) error {
	var hashSize, fixedHeaderSize uint64
	hashBuf := make([]byte, cimHashSize)

	// the blockPath could be a path to a block device or a file. In either case there should be no trailing backslash.
	blockPath = strings.TrimSuffix(blockPath, "\\")

	if err := winapi.CimSealImage(blockPath, &hashSize, &fixedHeaderSize, &hashBuf[0]); err != nil {
		return fmt.Errorf("failed to seal block CIM: %w", err)
	} else if hashSize != cimHashSize {
		return fmt.Errorf("unexpected cim hash size %d", hashSize)
	}
	return nil
}

// GetVerificationInfo returns the root digest of the given block CIM. This is only
// applicable for CIMs that are sealed after writing.
func GetVerificationInfo(blockPath string) ([]byte, error) {
	var (
		isSealed        uint32
		hashSize        uint64
		signatureSize   uint64
		fixedHeaderSize uint64
		hash            = make([]byte, cimHashSize)
	)
	if err := winapi.CimGetVerificationInformation(blockPath, &isSealed, &hashSize, &signatureSize, &fixedHeaderSize, &hash[0], nil); err != nil {
		return nil, fmt.Errorf("failed to get verification info from the CIM: %w", err)
	} else if hashSize != cimHashSize {
		return nil, fmt.Errorf("unexpected cim hash size %d", hashSize)
	} else if isSealed == 0 {
		return nil, fmt.Errorf("cim is not sealed")
	}
	return hash, nil
}
