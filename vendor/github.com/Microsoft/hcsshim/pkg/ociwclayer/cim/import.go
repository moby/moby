//go:build windows
// +build windows

package cim

import (
	"archive/tar"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio/backuptar"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/wclayer/cim"
	"github.com/Microsoft/hcsshim/pkg/ociwclayer"
	"golang.org/x/sys/windows"
)

// ImportCimLayerFromTar reads a layer from an OCI layer tar stream and extracts it into
// the CIM format at the specified path. The caller must specify the parent layers, if
// any, ordered from lowest to highest layer.
// This function expects that the layer paths (both the layer that is being imported & the parent layers) are
// formatted like `.../snapshots/<id>` and the corresponding layer CIMs are located/will be created at
// `.../snapshots/cim-layers/<id>.cim`. Each CIM file also has corresponding region & objectID files and those
// files will also be stored inside the `cim-layers` directory.
//
// This function returns the total size of the layer's files, in bytes.
func ImportCimLayerFromTar(ctx context.Context, r io.Reader, layerPath string, parentLayerPaths []string) (int64, error) {
	err := os.MkdirAll(layerPath, 0)
	if err != nil {
		return 0, err
	}

	w, err := cim.NewCimLayerWriter(ctx, layerPath, parentLayerPaths)
	if err != nil {
		return 0, err
	}

	n, err := writeCimLayerFromTar(ctx, r, w, layerPath)
	cerr := w.Close(ctx)
	if err != nil {
		return 0, err
	}
	if cerr != nil {
		return 0, cerr
	}
	return n, nil
}

func writeCimLayerFromTar(ctx context.Context, r io.Reader, w *cim.CimLayerWriter, layerPath string) (int64, error) {
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

func DestroyCimLayer(layerPath string) error {
	return cim.DestroyCimLayer(context.Background(), layerPath)
}
