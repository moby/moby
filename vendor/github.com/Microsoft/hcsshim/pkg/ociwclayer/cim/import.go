//go:build windows
// +build windows

package cim

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio/backuptar"
	"github.com/Microsoft/hcsshim/ext4/tar2ext4"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/wclayer/cim"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
	"github.com/Microsoft/hcsshim/pkg/ociwclayer"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// ImportCimLayerFromTar reads a layer from an OCI layer tar stream and extracts it into
// the CIM format at the specified path.
// `layerPath` is the directory which can be used to store intermediate files generated during layer extraction (and these file are also used when extracting children layers of this layer)
// `cimPath` is the path to the CIM in which layer files must be stored. Note that region & object files are created when writing to a CIM, these files will be created next to the `cimPath`.
// `parentLayerCimPaths` are paths to the parent layer CIMs, ordered from highest to lowest, i.e the CIM at `parentLayerCimPaths[0]` will be the immediate parent of the layer that is being extracted here.
// `parentLayerPaths` are paths to the parent layer directories. Ordered from highest to lowest.
//
// This function returns the total size of the layer's files, in bytes.
func ImportCimLayerFromTar(ctx context.Context, r io.Reader, layerPath, cimPath string, parentLayerPaths, parentLayerCimPaths []string) (_ int64, err error) {
	log.G(ctx).WithFields(logrus.Fields{
		"layer path":             layerPath,
		"layer cim path":         cimPath,
		"parent layer paths":     strings.Join(parentLayerPaths, ", "),
		"parent layer CIM paths": strings.Join(parentLayerCimPaths, ", "),
	}).Debug("Importing cim layer from tar")

	err = os.MkdirAll(layerPath, 0755)
	if err != nil {
		return 0, err
	}

	w, err := cim.NewForkedCimLayerWriter(ctx, layerPath, cimPath, parentLayerPaths, parentLayerCimPaths)
	if err != nil {
		return 0, err
	}

	n, err := writeCimLayerFromTar(ctx, r, w)
	cerr := w.Close(ctx)
	if err != nil {
		return 0, err
	}
	if cerr != nil {
		return 0, cerr
	}
	return n, nil
}

type blockCIMLayerImportConfig struct {
	// import layers with integrity enabled CIMs
	dataIntegrity bool
	// parent layers
	parentLayers []*cimfs.BlockCIM
	// append VHD footer to the import CIMs
	appendVHDFooter bool
}

// BlockCIMOpt is a function type for configuring block CIM creation options
type BlockCIMLayerImportOpt func(*blockCIMLayerImportConfig) error

func WithLayerIntegrity() BlockCIMLayerImportOpt {
	return func(opts *blockCIMLayerImportConfig) error {
		opts.dataIntegrity = true
		return nil
	}
}

func WithVHDFooter() BlockCIMLayerImportOpt {
	return func(opts *blockCIMLayerImportConfig) error {
		opts.appendVHDFooter = true
		return nil
	}
}

func WithParentLayers(parentLayers []*cimfs.BlockCIM) BlockCIMLayerImportOpt {
	return func(opts *blockCIMLayerImportConfig) error {
		opts.parentLayers = parentLayers
		return nil
	}
}

func writeIntegrityChecksumInfoFile(ctx context.Context, blockPath string) error {
	log.G(ctx).Debugf("writing integrity checksum file for block CIM `%s`", blockPath)
	// for convenience write a file that has the base64 encoded root digest of the generated verified CIM.
	// this same base64 string can be used in the confidential policy.
	digest, err := cimfs.GetVerificationInfo(blockPath)
	if err != nil {
		return fmt.Errorf("failed to query verified info of the CIM layer: %w", err)
	}

	digestFile, err := os.Create(filepath.Join(filepath.Dir(blockPath), "integrity_checksum"))
	if err != nil {
		return fmt.Errorf("failed to create verification info file: %w", err)
	}
	defer digestFile.Close()

	digestStr := base64.URLEncoding.EncodeToString(digest)
	if wn, err := digestFile.WriteString(digestStr); err != nil {
		return fmt.Errorf("failed to write verification info: %w", err)
	} else if wn != len(digestStr) {
		return fmt.Errorf("incomplete write of verification info: %w", err)
	}
	return nil
}

func ImportBlockCIMLayerWithOpts(ctx context.Context, r io.Reader, layer *cimfs.BlockCIM, opts ...BlockCIMLayerImportOpt) (_ int64, err error) {
	log.G(ctx).WithField("layer", layer).Debug("Importing block CIM layer from tar")

	err = os.MkdirAll(filepath.Dir(layer.BlockPath), 0755)
	if err != nil {
		return 0, err
	}

	config := &blockCIMLayerImportConfig{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return 0, fmt.Errorf("block CIM import config option failure: %w", err)
		}
	}

	log.G(ctx).WithField("config", *config).Debug("layer import config")

	bcimWriterOpts := []cimfs.BlockCIMOpt{}
	if config.dataIntegrity {
		bcimWriterOpts = append(bcimWriterOpts, cimfs.WithDataIntegrity())
	}

	w, err := cim.NewBlockCIMLayerWriterWithOpts(ctx, layer, config.parentLayers, bcimWriterOpts...)
	if err != nil {
		return 0, err
	}

	n, err := writeCimLayerFromTar(ctx, r, w)
	cerr := w.Close(ctx)
	if err != nil {
		return 0, err
	}
	if cerr != nil {
		return 0, cerr
	}

	if config.appendVHDFooter {
		log.G(ctx).Debugf("appending VHD footer to block CIM at `%s`", layer.BlockPath)
		if err = tar2ext4.ConvertFileToVhd(layer.BlockPath); err != nil {
			return 0, fmt.Errorf("append VHD footer to block CIM: %w", err)
		}
	}

	if config.dataIntegrity {
		if err = writeIntegrityChecksumInfoFile(ctx, layer.BlockPath); err != nil {
			return 0, err
		}
	}

	return n, nil
}

// ImportSingleFileCimLayerFromTar reads a layer from an OCI layer tar stream and extracts
// it into the SingleFileCIM format.
func ImportSingleFileCimLayerFromTar(ctx context.Context, r io.Reader, layer *cimfs.BlockCIM, parentLayers []*cimfs.BlockCIM) (_ int64, err error) {
	return ImportBlockCIMLayerWithOpts(ctx, r, layer, WithParentLayers(parentLayers))
}

func writeCimLayerFromTar(ctx context.Context, r io.Reader, w cim.CIMLayerWriter) (int64, error) {
	tr := tar.NewReader(r)
	buf := bufio.NewWriter(w)
	size := int64(0)

	// Iterate through the files in the archive.
	hdr, loopErr := tr.Next()
	for loopErr == nil {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		// Note: path is used instead of filepath to prevent OS specific handling
		// of the tar path
		base := path.Base(hdr.Name)
		if strings.HasPrefix(base, ociwclayer.WhiteoutPrefix) {
			name := path.Join(path.Dir(hdr.Name), base[len(ociwclayer.WhiteoutPrefix):])
			if rErr := w.Remove(filepath.FromSlash(name)); rErr != nil {
				return 0, rErr
			}
			hdr, loopErr = tr.Next()
		} else if hdr.Typeflag == tar.TypeLink {
			if linkErr := w.AddLink(filepath.FromSlash(hdr.Name), filepath.FromSlash(hdr.Linkname)); linkErr != nil {
				return 0, linkErr
			}
			hdr, loopErr = tr.Next()
		} else {
			name, fileSize, fileInfo, err := backuptar.FileInfoFromHeader(hdr)
			if err != nil {
				return 0, err
			}
			sddl, err := backuptar.SecurityDescriptorFromTarHeader(hdr)
			if err != nil {
				return 0, err
			}
			eadata, err := backuptar.ExtendedAttributesFromTarHeader(hdr)
			if err != nil {
				return 0, err
			}

			var reparse []byte
			// As of now the only valid reparse data in a layer will be for a symlink. If file is
			// a symlink set reparse attribute and ensure reparse data buffer isn't
			// empty. Otherwise remove the reparse attributed.
			fileInfo.FileAttributes &^= uint32(windows.FILE_ATTRIBUTE_REPARSE_POINT)
			if hdr.Typeflag == tar.TypeSymlink {
				reparse = backuptar.EncodeReparsePointFromTarHeader(hdr)
				if len(reparse) > 0 {
					fileInfo.FileAttributes |= uint32(windows.FILE_ATTRIBUTE_REPARSE_POINT)
				}
			}

			if addErr := w.Add(filepath.FromSlash(name), fileInfo, fileSize, sddl, eadata, reparse); addErr != nil {
				return 0, addErr
			}
			if hdr.Typeflag == tar.TypeReg {
				if _, cpErr := io.Copy(buf, tr); cpErr != nil {
					return 0, cpErr
				}
			}
			size += fileSize

			// Copy all the alternate data streams and return the next non-ADS header.
			var ahdr *tar.Header
			for {
				ahdr, loopErr = tr.Next()
				if loopErr != nil {
					break
				}

				if ahdr.Typeflag != tar.TypeReg || !strings.HasPrefix(ahdr.Name, hdr.Name+":") {
					hdr = ahdr
					break
				}

				// stream names have following format: '<filename>:<stream name>:$DATA'
				// $DATA is one of the valid types of streams. We currently only support
				// data streams so fail if this is some other type of stream.
				if !strings.HasSuffix(ahdr.Name, ":$DATA") {
					return 0, fmt.Errorf("stream types other than $DATA are not supported, found: %s", ahdr.Name)
				}

				if addErr := w.AddAlternateStream(filepath.FromSlash(ahdr.Name), uint64(ahdr.Size)); addErr != nil {
					return 0, addErr
				}

				if _, cpErr := io.Copy(buf, tr); cpErr != nil {
					return 0, cpErr
				}
			}
		}

		if flushErr := buf.Flush(); flushErr != nil {
			if loopErr == nil {
				loopErr = flushErr
			} else {
				log.G(ctx).WithError(flushErr).Warn("flush buffer during layer write failed")
			}
		}
	}
	if !errors.Is(loopErr, io.EOF) {
		return 0, loopErr
	}
	return size, nil
}

// MergeBlockCIMLayersWithOpts create a new block CIM at mergedCIM.BlockPath and then
// creates a new CIM inside that block CIM that merges all the contents of the provided
// sourceCIMs. Note that this is only a metadata merge, so when this merged CIM is being
// mounted, all the sourceCIMs must be provided too and they MUST be provided in the same
// order. Expected order of sourceCIMs is that the base layer should be at the last index
// and the topmost layer should be at 0'th index.  Merge operation can take a long time in
// certain situations, this function respects context deadlines in such cases.  This
// function is NOT thread safe, it is caller's responsibility to handle thread safety.
func MergeBlockCIMLayersWithOpts(ctx context.Context, sourceCIMs []*cimfs.BlockCIM, mergedCIM *cimfs.BlockCIM, opts ...BlockCIMLayerImportOpt) (retErr error) {
	log.G(ctx).WithFields(logrus.Fields{
		"source CIMs": sourceCIMs,
		"merged CIM":  mergedCIM,
	}).Debug("Merging block CIM layers")

	// check if a merged CIM already exists
	_, err := os.Stat(mergedCIM.BlockPath)
	if err == nil {
		return os.ErrExist
	}

	// Apply configuration options
	config := &blockCIMLayerImportConfig{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return fmt.Errorf("apply merge option: %w", err)
		}
	}

	// Prepare options for the underlying cimfs.MergeBlockCIMsWithOpts call
	cimfsOpts := []cimfs.BlockCIMOpt{cimfs.WithConsistentCIM()}
	if config.dataIntegrity {
		cimfsOpts = append(cimfsOpts, cimfs.WithDataIntegrity())
	}

	// Ensure the directory for the merged CIM exists
	if err := os.MkdirAll(filepath.Dir(mergedCIM.BlockPath), 0755); err != nil {
		return fmt.Errorf("create directory for merged CIM: %w", err)
	}
	defer func() {
		if retErr != nil {
			if rmErr := os.Remove(mergedCIM.BlockPath); rmErr != nil {
				log.G(ctx).WithError(retErr).Warnf("error in cleanup on failure: %s", rmErr)
			}
		}
	}()

	// Run the merge operation in a goroutine to handle context cancellation
	// The merge operation can take a long time, so we need to respect context deadlines
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		err := cimfs.MergeBlockCIMsWithOpts(ctx, mergedCIM, sourceCIMs, cimfsOpts...)
		errCh <- err
	}()

	// Wait for either the merge to complete or context to be cancelled
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errCh:
	}
	if err != nil {
		return fmt.Errorf("merge block CIMs failed: %w", err)
	}

	// Handle VHD footer if requested
	if config.appendVHDFooter {
		log.G(ctx).Debugf("appending VHD footer to block CIM at `%s`", mergedCIM.BlockPath)
		if err = tar2ext4.ConvertFileToVhd(mergedCIM.BlockPath); err != nil {
			return fmt.Errorf("append VHD footer to block CIM: %w", err)
		}
	}
	return nil
}
