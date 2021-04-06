// +build windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package archive

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/backuptar"
	"github.com/Microsoft/hcsshim"
	"github.com/containerd/containerd/sys"
	"github.com/pkg/errors"
)

var (
	// mutatedFiles is a list of files that are mutated by the import process
	// and must be backed up and restored.
	mutatedFiles = map[string]string{
		"UtilityVM/Files/EFI/Microsoft/Boot/BCD":      "bcd.bak",
		"UtilityVM/Files/EFI/Microsoft/Boot/BCD.LOG":  "bcd.log.bak",
		"UtilityVM/Files/EFI/Microsoft/Boot/BCD.LOG1": "bcd.log1.bak",
		"UtilityVM/Files/EFI/Microsoft/Boot/BCD.LOG2": "bcd.log2.bak",
	}
)

// tarName returns platform-specific filepath
// to canonical posix-style path for tar archival. p is relative
// path.
func tarName(p string) (string, error) {
	// windows: convert windows style relative path with backslashes
	// into forward slashes. Since windows does not allow '/' or '\'
	// in file names, it is mostly safe to replace however we must
	// check just in case
	if strings.Contains(p, "/") {
		return "", fmt.Errorf("windows path contains forward slash: %s", p)
	}

	return strings.Replace(p, string(os.PathSeparator), "/", -1), nil
}

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.
func chmodTarEntry(perm os.FileMode) os.FileMode {
	perm &= 0755
	// Add the x bit: make everything +x from windows
	perm |= 0111

	return perm
}

func setHeaderForSpecialDevice(*tar.Header, string, os.FileInfo) error {
	// do nothing. no notion of Rdev, Inode, Nlink in stat on Windows
	return nil
}

func open(p string) (*os.File, error) {
	// We use sys.OpenSequential to ensure we use sequential file
	// access on Windows to avoid depleting the standby list.
	return sys.OpenSequential(p)
}

func openFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	// Source is regular file. We use sys.OpenFileSequential to use sequential
	// file access to avoid depleting the standby list on Windows.
	return sys.OpenFileSequential(name, flag, perm)
}

func mkdir(path string, perm os.FileMode) error {
	return os.Mkdir(path, perm)
}

func skipFile(hdr *tar.Header) bool {
	// Windows does not support filenames with colons in them. Ignore
	// these files. This is not a problem though (although it might
	// appear that it is). Let's suppose a client is running docker pull.
	// The daemon it points to is Windows. Would it make sense for the
	// client to be doing a docker pull Ubuntu for example (which has files
	// with colons in the name under /usr/share/man/man3)? No, absolutely
	// not as it would really only make sense that they were pulling a
	// Windows image. However, for development, it is necessary to be able
	// to pull Linux images which are in the repository.
	//
	// TODO Windows. Once the registry is aware of what images are Windows-
	// specific or Linux-specific, this warning should be changed to an error
	// to cater for the situation where someone does manage to upload a Linux
	// image but have it tagged as Windows inadvertently.
	return strings.Contains(hdr.Name, ":")
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo
func handleTarTypeBlockCharFifo(hdr *tar.Header, path string) error {
	return nil
}

func handleLChmod(hdr *tar.Header, path string, hdrInfo os.FileInfo) error {
	return nil
}

func getxattr(path, attr string) ([]byte, error) {
	return nil, nil
}

func setxattr(path, key, value string) error {
	// Return not support error, do not wrap underlying not supported
	// since xattrs should not exist in windows diff archives
	return errors.New("xattrs not supported on Windows")
}

func copyDirInfo(fi os.FileInfo, path string) error {
	if err := os.Chmod(path, fi.Mode()); err != nil {
		return errors.Wrapf(err, "failed to chmod %s", path)
	}
	return nil
}

func copyUpXAttrs(dst, src string) error {
	return nil
}

// applyWindowsLayer applies a tar stream of an OCI style diff tar of a Windows
// layer using the hcsshim layer writer and backup streams.
// See https://github.com/opencontainers/image-spec/blob/master/layer.md#applying-changesets
func applyWindowsLayer(ctx context.Context, root string, r io.Reader, options ApplyOptions) (size int64, err error) {
	home, id := filepath.Split(root)
	info := hcsshim.DriverInfo{
		HomeDir: home,
	}

	w, err := hcsshim.NewLayerWriter(info, id, options.Parents)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err2 := w.Close(); err2 != nil {
			// This error should not be discarded as a failure here
			// could result in an invalid layer on disk
			if err == nil {
				err = err2
			}
		}
	}()

	tr := tar.NewReader(r)
	buf := bufio.NewWriter(nil)
	hdr, nextErr := tr.Next()
	// Iterate through the files in the archive.
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		if nextErr == io.EOF {
			// end of tar archive
			break
		}
		if nextErr != nil {
			return 0, nextErr
		}

		// Note: path is used instead of filepath to prevent OS specific handling
		// of the tar path
		base := path.Base(hdr.Name)
		if strings.HasPrefix(base, whiteoutPrefix) {
			dir := path.Dir(hdr.Name)
			originalBase := base[len(whiteoutPrefix):]
			originalPath := path.Join(dir, originalBase)
			if err := w.Remove(filepath.FromSlash(originalPath)); err != nil {
				return 0, err
			}
			hdr, nextErr = tr.Next()
		} else if hdr.Typeflag == tar.TypeLink {
			err := w.AddLink(filepath.FromSlash(hdr.Name), filepath.FromSlash(hdr.Linkname))
			if err != nil {
				return 0, err
			}
			hdr, nextErr = tr.Next()
		} else {
			name, fileSize, fileInfo, err := backuptar.FileInfoFromHeader(hdr)
			if err != nil {
				return 0, err
			}
			if err := w.Add(filepath.FromSlash(name), fileInfo); err != nil {
				return 0, err
			}
			size += fileSize
			hdr, nextErr = tarToBackupStreamWithMutatedFiles(buf, w, tr, hdr, root)
		}
	}

	return
}

// tarToBackupStreamWithMutatedFiles reads data from a tar stream and
// writes it to a backup stream, and also saves any files that will be mutated
// by the import layer process to a backup location.
func tarToBackupStreamWithMutatedFiles(buf *bufio.Writer, w io.Writer, t *tar.Reader, hdr *tar.Header, root string) (nextHdr *tar.Header, err error) {
	var (
		bcdBackup       *os.File
		bcdBackupWriter *winio.BackupFileWriter
	)
	if backupPath, ok := mutatedFiles[hdr.Name]; ok {
		bcdBackup, err = os.Create(filepath.Join(root, backupPath))
		if err != nil {
			return nil, err
		}
		defer func() {
			cerr := bcdBackup.Close()
			if err == nil {
				err = cerr
			}
		}()

		bcdBackupWriter = winio.NewBackupFileWriter(bcdBackup, false)
		defer func() {
			cerr := bcdBackupWriter.Close()
			if err == nil {
				err = cerr
			}
		}()

		buf.Reset(io.MultiWriter(w, bcdBackupWriter))
	} else {
		buf.Reset(w)
	}

	defer func() {
		ferr := buf.Flush()
		if err == nil {
			err = ferr
		}
	}()

	return backuptar.WriteBackupStreamFromTarFile(buf, t, hdr)
}
