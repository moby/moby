// Package archive provides helper functions for dealing with archive files.
package archive

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/moby/patternmatcher"
	"github.com/moby/sys/sequential"
	"github.com/moby/sys/user"

	"github.com/moby/go-archive/compression"
	"github.com/moby/go-archive/tarheader"
)

// ImpliedDirectoryMode represents the mode (Unix permissions) applied to directories that are implied by files in a
// tar, but that do not have their own header entry.
//
// The permissions mask is stored in a constant instead of locally to ensure that magic numbers do not
// proliferate in the codebase. The default value 0755 has been selected based on the default umask of 0022, and
// a convention of mkdir(1) calling mkdir(2) with permissions of 0777, resulting in a final value of 0755.
//
// This value is currently implementation-defined, and not captured in any cross-runtime specification. Thus, it is
// subject to change in Moby at any time -- image authors who require consistent or known directory permissions
// should explicitly control them by ensuring that header entries exist for any applicable path.
const ImpliedDirectoryMode = 0o755

type (
	// Compression is the state represents if compressed or not.
	//
	// Deprecated: use [compression.Compression].
	Compression = compression.Compression
	// WhiteoutFormat is the format of whiteouts unpacked
	WhiteoutFormat int

	ChownOpts struct {
		UID int
		GID int
	}

	// TarOptions wraps the tar options.
	TarOptions struct {
		IncludeFiles     []string
		ExcludePatterns  []string
		Compression      compression.Compression
		NoLchown         bool
		IDMap            user.IdentityMapping
		ChownOpts        *ChownOpts
		IncludeSourceDir bool
		// WhiteoutFormat is the expected on disk format for whiteout files.
		// This format will be converted to the standard format on pack
		// and from the standard format on unpack.
		WhiteoutFormat WhiteoutFormat
		// When unpacking, specifies whether overwriting a directory with a
		// non-directory is allowed and vice versa.
		NoOverwriteDirNonDir bool
		// For each include when creating an archive, the included name will be
		// replaced with the matching name from this map.
		RebaseNames map[string]string
		InUserNS    bool
		// Allow unpacking to succeed in spite of failures to set extended
		// attributes on the unpacked files due to the destination filesystem
		// not supporting them or a lack of permissions. Extended attributes
		// were probably in the archive for a reason, so set this option at
		// your own peril.
		BestEffortXattrs bool
	}
)

// Archiver implements the Archiver interface and allows the reuse of most utility functions of
// this package with a pluggable Untar function. Also, to facilitate the passing of specific id
// mappings for untar, an Archiver can be created with maps which will then be passed to Untar operations.
type Archiver struct {
	Untar     func(io.Reader, string, *TarOptions) error
	IDMapping user.IdentityMapping
}

// NewDefaultArchiver returns a new Archiver without any IdentityMapping
func NewDefaultArchiver() *Archiver {
	return &Archiver{Untar: Untar}
}

// breakoutError is used to differentiate errors related to breaking out
// When testing archive breakout in the unit tests, this error is expected
// in order for the test to pass.
type breakoutError error

const (
	Uncompressed = compression.None  // Deprecated: use [compression.None].
	Bzip2        = compression.Bzip2 // Deprecated: use [compression.Bzip2].
	Gzip         = compression.Gzip  // Deprecated: use [compression.Gzip].
	Xz           = compression.Xz    // Deprecated: use [compression.Xz].
	Zstd         = compression.Zstd  // Deprecated: use [compression.Zstd].
)

const (
	AUFSWhiteoutFormat    WhiteoutFormat = 0 // AUFSWhiteoutFormat is the default format for whiteouts
	OverlayWhiteoutFormat WhiteoutFormat = 1 // OverlayWhiteoutFormat formats whiteout according to the overlay standard.
)

// IsArchivePath checks if the (possibly compressed) file at the given path
// starts with a tar file header.
func IsArchivePath(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	rdr, err := compression.DecompressStream(file)
	if err != nil {
		return false
	}
	defer rdr.Close()
	r := tar.NewReader(rdr)
	_, err = r.Next()
	return err == nil
}

// DetectCompression detects the compression algorithm of the source.
//
// Deprecated: use [compression.Detect].
func DetectCompression(source []byte) compression.Compression {
	return compression.Detect(source)
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
//
// Deprecated: use [compression.DecompressStream].
func DecompressStream(archive io.Reader) (io.ReadCloser, error) {
	return compression.DecompressStream(archive)
}

// CompressStream compresses the dest with specified compression algorithm.
//
// Deprecated: use [compression.CompressStream].
func CompressStream(dest io.Writer, comp compression.Compression) (io.WriteCloser, error) {
	return compression.CompressStream(dest, comp)
}

// TarModifierFunc is a function that can be passed to ReplaceFileTarWrapper to
// modify the contents or header of an entry in the archive. If the file already
// exists in the archive the TarModifierFunc will be called with the Header and
// a reader which will return the files content. If the file does not exist both
// header and content will be nil.
type TarModifierFunc func(path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error)

// ReplaceFileTarWrapper converts inputTarStream to a new tar stream. Files in the
// tar stream are modified if they match any of the keys in mods.
func ReplaceFileTarWrapper(inputTarStream io.ReadCloser, mods map[string]TarModifierFunc) io.ReadCloser {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		tarReader := tar.NewReader(inputTarStream)
		tarWriter := tar.NewWriter(pipeWriter)
		defer inputTarStream.Close()
		defer tarWriter.Close()

		modify := func(name string, original *tar.Header, modifier TarModifierFunc, tarReader io.Reader) error {
			header, data, err := modifier(name, original, tarReader)
			switch {
			case err != nil:
				return err
			case header == nil:
				return nil
			}

			if header.Name == "" {
				header.Name = name
			}
			header.Size = int64(len(data))
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}
			if len(data) != 0 {
				if _, err := tarWriter.Write(data); err != nil {
					return err
				}
			}
			return nil
		}

		var err error
		var originalHeader *tar.Header
		for {
			originalHeader, err = tarReader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				pipeWriter.CloseWithError(err)
				return
			}

			modifier, ok := mods[originalHeader.Name]
			if !ok {
				// No modifiers for this file, copy the header and data
				if err := tarWriter.WriteHeader(originalHeader); err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
				if err := copyWithBuffer(tarWriter, tarReader); err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
				continue
			}
			delete(mods, originalHeader.Name)

			if err := modify(originalHeader.Name, originalHeader, modifier, tarReader); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
		}

		// Apply the modifiers that haven't matched any files in the archive
		for name, modifier := range mods {
			if err := modify(name, nil, modifier, nil); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
		}

		pipeWriter.Close()
	}()
	return pipeReader
}

// FileInfoHeaderNoLookups creates a partially-populated tar.Header from fi.
//
// Deprecated: use [tarheader.FileInfoHeaderNoLookups].
func FileInfoHeaderNoLookups(fi os.FileInfo, link string) (*tar.Header, error) {
	return tarheader.FileInfoHeaderNoLookups(fi, link)
}

// FileInfoHeader creates a populated Header from fi.
//
// Compared to the archive/tar package, this function fills in less information
// but is safe to call from a chrooted process. The AccessTime and ChangeTime
// fields are not set in the returned header, ModTime is truncated to one-second
// precision, and the Uname and Gname fields are only set when fi is a FileInfo
// value returned from tar.Header.FileInfo().
func FileInfoHeader(name string, fi os.FileInfo, link string) (*tar.Header, error) {
	hdr, err := tarheader.FileInfoHeaderNoLookups(fi, link)
	if err != nil {
		return nil, err
	}
	hdr.Format = tar.FormatPAX
	hdr.ModTime = hdr.ModTime.Truncate(time.Second)
	hdr.AccessTime = time.Time{}
	hdr.ChangeTime = time.Time{}
	hdr.Mode = int64(chmodTarEntry(os.FileMode(hdr.Mode)))
	hdr.Name = canonicalTarName(name, fi.IsDir())
	return hdr, nil
}

const paxSchilyXattr = "SCHILY.xattr."

// ReadSecurityXattrToTarHeader reads security.capability xattr from filesystem
// to a tar header
func ReadSecurityXattrToTarHeader(path string, hdr *tar.Header) error {
	const (
		// Values based on linux/include/uapi/linux/capability.h
		xattrCapsSz2    = 20
		versionOffset   = 3
		vfsCapRevision2 = 2
		vfsCapRevision3 = 3
	)
	capability, _ := lgetxattr(path, "security.capability")
	if capability != nil {
		if capability[versionOffset] == vfsCapRevision3 {
			// Convert VFS_CAP_REVISION_3 to VFS_CAP_REVISION_2 as root UID makes no
			// sense outside the user namespace the archive is built in.
			capability[versionOffset] = vfsCapRevision2
			capability = capability[:xattrCapsSz2]
		}
		if hdr.PAXRecords == nil {
			hdr.PAXRecords = make(map[string]string)
		}
		hdr.PAXRecords[paxSchilyXattr+"security.capability"] = string(capability)
	}
	return nil
}

type tarWhiteoutConverter interface {
	ConvertWrite(*tar.Header, string, os.FileInfo) (*tar.Header, error)
	ConvertRead(*tar.Header, string) (bool, error)
}

type tarAppender struct {
	TarWriter *tar.Writer

	// for hardlink mapping
	SeenFiles       map[uint64]string
	IdentityMapping user.IdentityMapping
	ChownOpts       *ChownOpts

	// For packing and unpacking whiteout files in the
	// non standard format. The whiteout files defined
	// by the AUFS standard are used as the tar whiteout
	// standard.
	WhiteoutConverter tarWhiteoutConverter
}

func newTarAppender(idMapping user.IdentityMapping, writer io.Writer, chownOpts *ChownOpts) *tarAppender {
	return &tarAppender{
		SeenFiles:       make(map[uint64]string),
		TarWriter:       tar.NewWriter(writer),
		IdentityMapping: idMapping,
		ChownOpts:       chownOpts,
	}
}

// canonicalTarName provides a platform-independent and consistent POSIX-style
// path for files and directories to be archived regardless of the platform.
func canonicalTarName(name string, isDir bool) string {
	name = filepath.ToSlash(name)

	// suffix with '/' for directories
	if isDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

// addTarFile adds to the tar archive a file from `path` as `name`
func (ta *tarAppender) addTarFile(path, name string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	var link string
	if fi.Mode()&os.ModeSymlink != 0 {
		var err error
		link, err = os.Readlink(path)
		if err != nil {
			return err
		}
	}

	hdr, err := FileInfoHeader(name, fi, link)
	if err != nil {
		return err
	}
	if err := ReadSecurityXattrToTarHeader(path, hdr); err != nil {
		return err
	}

	// if it's not a directory and has more than 1 link,
	// it's hard linked, so set the type flag accordingly
	if !fi.IsDir() && hasHardlinks(fi) {
		inode, err := getInodeFromStat(fi.Sys())
		if err != nil {
			return err
		}
		// a link should have a name that it links too
		// and that linked name should be first in the tar archive
		if oldpath, ok := ta.SeenFiles[inode]; ok {
			hdr.Typeflag = tar.TypeLink
			hdr.Linkname = oldpath
			hdr.Size = 0 // This Must be here for the writer math to add up!
		} else {
			ta.SeenFiles[inode] = name
		}
	}

	// check whether the file is overlayfs whiteout
	// if yes, skip re-mapping container ID mappings.
	isOverlayWhiteout := fi.Mode()&os.ModeCharDevice != 0 && hdr.Devmajor == 0 && hdr.Devminor == 0

	// handle re-mapping container ID mappings back to host ID mappings before
	// writing tar headers/files. We skip whiteout files because they were written
	// by the kernel and already have proper ownership relative to the host
	if !isOverlayWhiteout && !strings.HasPrefix(filepath.Base(hdr.Name), WhiteoutPrefix) && !ta.IdentityMapping.Empty() {
		uid, gid, err := getFileUIDGID(fi.Sys())
		if err != nil {
			return err
		}
		hdr.Uid, hdr.Gid, err = ta.IdentityMapping.ToContainer(uid, gid)
		if err != nil {
			return err
		}
	}

	// explicitly override with ChownOpts
	if ta.ChownOpts != nil {
		hdr.Uid = ta.ChownOpts.UID
		hdr.Gid = ta.ChownOpts.GID
	}

	if ta.WhiteoutConverter != nil {
		wo, err := ta.WhiteoutConverter.ConvertWrite(hdr, path, fi)
		if err != nil {
			return err
		}

		// If a new whiteout file exists, write original hdr, then
		// replace hdr with wo to be written after. Whiteouts should
		// always be written after the original. Note the original
		// hdr may have been updated to be a whiteout with returning
		// a whiteout header
		if wo != nil {
			if err := ta.TarWriter.WriteHeader(hdr); err != nil {
				return err
			}
			if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
				return fmt.Errorf("tar: cannot use whiteout for non-empty file")
			}
			hdr = wo
		}
	}

	if err := ta.TarWriter.WriteHeader(hdr); err != nil {
		return err
	}

	if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
		// We use sequential file access to avoid depleting the standby list on
		// Windows. On Linux, this equates to a regular os.Open.
		file, err := sequential.Open(path)
		if err != nil {
			return err
		}

		err = copyWithBuffer(ta.TarWriter, file)
		file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func createTarFile(path, extractDir string, hdr *tar.Header, reader io.Reader, opts *TarOptions) error {
	var (
		Lchown                     = true
		inUserns, bestEffortXattrs bool
		chownOpts                  *ChownOpts
	)

	// TODO(thaJeztah): make opts a required argument.
	if opts != nil {
		Lchown = !opts.NoLchown
		inUserns = opts.InUserNS // TODO(thaJeztah): consider deprecating opts.InUserNS and detect locally.
		chownOpts = opts.ChownOpts
		bestEffortXattrs = opts.BestEffortXattrs
	}

	// hdr.Mode is in linux format, which we can use for sycalls,
	// but for os.Foo() calls we need the mode converted to os.FileMode,
	// so use hdrInfo.Mode() (they differ for e.g. setuid bits)
	hdrInfo := hdr.FileInfo()

	switch hdr.Typeflag {
	case tar.TypeDir:
		// Create directory unless it exists as a directory already.
		// In that case we just want to merge the two
		if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
			if err := os.Mkdir(path, hdrInfo.Mode()); err != nil {
				return err
			}
		}

	case tar.TypeReg:
		// Source is regular file. We use sequential file access to avoid depleting
		// the standby list on Windows. On Linux, this equates to a regular os.OpenFile.
		file, err := sequential.OpenFile(path, os.O_CREATE|os.O_WRONLY, hdrInfo.Mode())
		if err != nil {
			return err
		}
		if err := copyWithBuffer(file, reader); err != nil {
			_ = file.Close()
			return err
		}
		_ = file.Close()

	case tar.TypeBlock, tar.TypeChar:
		if inUserns { // cannot create devices in a userns
			log.G(context.TODO()).WithFields(log.Fields{"path": path, "type": hdr.Typeflag}).Debug("skipping device nodes in a userns")
			return nil
		}
		// Handle this is an OS-specific way
		if err := handleTarTypeBlockCharFifo(hdr, path); err != nil {
			return err
		}

	case tar.TypeFifo:
		// Handle this is an OS-specific way
		if err := handleTarTypeBlockCharFifo(hdr, path); err != nil {
			if inUserns && errors.Is(err, syscall.EPERM) {
				// In most cases, cannot create a fifo if running in user namespace
				log.G(context.TODO()).WithFields(log.Fields{"error": err, "path": path, "type": hdr.Typeflag}).Debug("creating fifo node in a userns")
				return nil
			}
			return err
		}

	case tar.TypeLink:
		// #nosec G305 -- The target path is checked for path traversal.
		targetPath := filepath.Join(extractDir, hdr.Linkname)
		// check for hardlink breakout
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid hardlink %q -> %q", targetPath, hdr.Linkname))
		}
		if err := os.Link(targetPath, path); err != nil {
			return err
		}

	case tar.TypeSymlink:
		// 	path 				-> hdr.Linkname = targetPath
		// e.g. /extractDir/path/to/symlink 	-> ../2/file	= /extractDir/path/2/file
		targetPath := filepath.Join(filepath.Dir(path), hdr.Linkname) // #nosec G305 -- The target path is checked for path traversal.

		// the reason we don't need to check symlinks in the path (with FollowSymlinkInScope) is because
		// that symlink would first have to be created, which would be caught earlier, at this very check:
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid symlink %q -> %q", path, hdr.Linkname))
		}
		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return err
		}

	case tar.TypeXGlobalHeader:
		log.G(context.TODO()).Debug("PAX Global Extended Headers found and ignored")
		return nil

	default:
		return fmt.Errorf("unhandled tar header type %d", hdr.Typeflag)
	}

	// Lchown is not supported on Windows.
	if Lchown && runtime.GOOS != "windows" {
		if chownOpts == nil {
			chownOpts = &ChownOpts{UID: hdr.Uid, GID: hdr.Gid}
		}
		if err := os.Lchown(path, chownOpts.UID, chownOpts.GID); err != nil {
			var msg string
			if inUserns && errors.Is(err, syscall.EINVAL) {
				msg = " (try increasing the number of subordinate IDs in /etc/subuid and /etc/subgid)"
			}
			return fmt.Errorf("failed to Lchown %q for UID %d, GID %d%s: %w", path, hdr.Uid, hdr.Gid, msg, err)
		}
	}

	var xattrErrs []string
	for key, value := range hdr.PAXRecords {
		xattr, ok := strings.CutPrefix(key, paxSchilyXattr)
		if !ok {
			continue
		}
		if err := lsetxattr(path, xattr, []byte(value), 0); err != nil {
			if bestEffortXattrs && errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EPERM) {
				// EPERM occurs if modifying xattrs is not allowed. This can
				// happen when running in userns with restrictions (ChromeOS).
				xattrErrs = append(xattrErrs, err.Error())
				continue
			}
			return err
		}
	}

	if len(xattrErrs) > 0 {
		log.G(context.TODO()).WithFields(log.Fields{
			"errors": xattrErrs,
		}).Warn("ignored xattrs in archive: underlying filesystem doesn't support them")
	}

	// There is no LChmod, so ignore mode for symlink. Also, this
	// must happen after chown, as that can modify the file mode
	if err := handleLChmod(hdr, path, hdrInfo); err != nil {
		return err
	}

	aTime := boundTime(latestTime(hdr.AccessTime, hdr.ModTime))
	mTime := boundTime(hdr.ModTime)

	// chtimes doesn't support a NOFOLLOW flag atm
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := chtimes(path, aTime, mTime); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := chtimes(path, aTime, mTime); err != nil {
			return err
		}
	} else {
		if err := lchtimes(path, aTime, mTime); err != nil {
			return err
		}
	}
	return nil
}

// Tar creates an archive from the directory at `path`, and returns it as a
// stream of bytes.
func Tar(path string, comp compression.Compression) (io.ReadCloser, error) {
	return TarWithOptions(path, &TarOptions{Compression: comp})
}

// TarWithOptions creates an archive from the directory at `path`, only including files whose relative
// paths are included in `options.IncludeFiles` (if non-nil) or not in `options.ExcludePatterns`.
func TarWithOptions(srcPath string, options *TarOptions) (io.ReadCloser, error) {
	tb, err := NewTarballer(srcPath, options)
	if err != nil {
		return nil, err
	}
	go tb.Do()
	return tb.Reader(), nil
}

// Tarballer is a lower-level interface to TarWithOptions which gives the caller
// control over which goroutine the archiving operation executes on.
type Tarballer struct {
	srcPath           string
	options           *TarOptions
	pm                *patternmatcher.PatternMatcher
	pipeReader        *io.PipeReader
	pipeWriter        *io.PipeWriter
	compressWriter    io.WriteCloser
	whiteoutConverter tarWhiteoutConverter
}

// NewTarballer constructs a new tarballer. The arguments are the same as for
// TarWithOptions.
func NewTarballer(srcPath string, options *TarOptions) (*Tarballer, error) {
	pm, err := patternmatcher.New(options.ExcludePatterns)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()

	compressWriter, err := compression.CompressStream(pipeWriter, options.Compression)
	if err != nil {
		return nil, err
	}

	return &Tarballer{
		// Fix the source path to work with long path names. This is a no-op
		// on platforms other than Windows.
		srcPath:           addLongPathPrefix(srcPath),
		options:           options,
		pm:                pm,
		pipeReader:        pipeReader,
		pipeWriter:        pipeWriter,
		compressWriter:    compressWriter,
		whiteoutConverter: getWhiteoutConverter(options.WhiteoutFormat),
	}, nil
}

// Reader returns the reader for the created archive.
func (t *Tarballer) Reader() io.ReadCloser {
	return t.pipeReader
}

// Do performs the archiving operation in the background. The resulting archive
// can be read from t.Reader(). Do should only be called once on each Tarballer
// instance.
func (t *Tarballer) Do() {
	ta := newTarAppender(
		t.options.IDMap,
		t.compressWriter,
		t.options.ChownOpts,
	)
	ta.WhiteoutConverter = t.whiteoutConverter

	defer func() {
		// Make sure to check the error on Close.
		if err := ta.TarWriter.Close(); err != nil {
			log.G(context.TODO()).Errorf("Can't close tar writer: %s", err)
		}
		if err := t.compressWriter.Close(); err != nil {
			log.G(context.TODO()).Errorf("Can't close compress writer: %s", err)
		}
		if err := t.pipeWriter.Close(); err != nil {
			log.G(context.TODO()).Errorf("Can't close pipe writer: %s", err)
		}
	}()

	// In general we log errors here but ignore them because
	// during e.g. a diff operation the container can continue
	// mutating the filesystem and we can see transient errors
	// from this

	stat, err := os.Lstat(t.srcPath)
	if err != nil {
		return
	}

	if !stat.IsDir() {
		// We can't later join a non-dir with any includes because the
		// 'walk' will error if "file/." is stat-ed and "file" is not a
		// directory. So, we must split the source path and use the
		// basename as the include.
		if len(t.options.IncludeFiles) > 0 {
			log.G(context.TODO()).Warn("Tar: Can't archive a file with includes")
		}

		dir, base := SplitPathDirEntry(t.srcPath)
		t.srcPath = dir
		t.options.IncludeFiles = []string{base}
	}

	if len(t.options.IncludeFiles) == 0 {
		t.options.IncludeFiles = []string{"."}
	}

	seen := make(map[string]bool)

	for _, include := range t.options.IncludeFiles {
		rebaseName := t.options.RebaseNames[include]

		var (
			parentMatchInfo []patternmatcher.MatchInfo
			parentDirs      []string
		)

		walkRoot := getWalkRoot(t.srcPath, include)
		// TODO(thaJeztah): should this error be handled?
		_ = filepath.WalkDir(walkRoot, func(filePath string, f os.DirEntry, err error) error {
			if err != nil {
				log.G(context.TODO()).Errorf("Tar: Can't stat file %s to tar: %s", t.srcPath, err)
				return nil
			}

			relFilePath, err := filepath.Rel(t.srcPath, filePath)
			if err != nil || (!t.options.IncludeSourceDir && relFilePath == "." && f.IsDir()) {
				// Error getting relative path OR we are looking
				// at the source directory path. Skip in both situations.
				return nil
			}

			if t.options.IncludeSourceDir && include == "." && relFilePath != "." {
				relFilePath = strings.Join([]string{".", relFilePath}, string(filepath.Separator))
			}

			skip := false

			// If "include" is an exact match for the current file
			// then even if there's an "excludePatterns" pattern that
			// matches it, don't skip it. IOW, assume an explicit 'include'
			// is asking for that file no matter what - which is true
			// for some files, like .dockerignore and Dockerfile (sometimes)
			if include != relFilePath {
				for len(parentDirs) != 0 {
					lastParentDir := parentDirs[len(parentDirs)-1]
					if strings.HasPrefix(relFilePath, lastParentDir+string(os.PathSeparator)) {
						break
					}
					parentDirs = parentDirs[:len(parentDirs)-1]
					parentMatchInfo = parentMatchInfo[:len(parentMatchInfo)-1]
				}

				var matchInfo patternmatcher.MatchInfo
				if len(parentMatchInfo) != 0 {
					skip, matchInfo, err = t.pm.MatchesUsingParentResults(relFilePath, parentMatchInfo[len(parentMatchInfo)-1])
				} else {
					skip, matchInfo, err = t.pm.MatchesUsingParentResults(relFilePath, patternmatcher.MatchInfo{})
				}
				if err != nil {
					log.G(context.TODO()).Errorf("Error matching %s: %v", relFilePath, err)
					return err
				}

				if f.IsDir() {
					parentDirs = append(parentDirs, relFilePath)
					parentMatchInfo = append(parentMatchInfo, matchInfo)
				}
			}

			if skip {
				// If we want to skip this file and its a directory
				// then we should first check to see if there's an
				// excludes pattern (e.g. !dir/file) that starts with this
				// dir. If so then we can't skip this dir.

				// Its not a dir then so we can just return/skip.
				if !f.IsDir() {
					return nil
				}

				// No exceptions (!...) in patterns so just skip dir
				if !t.pm.Exclusions() {
					return filepath.SkipDir
				}

				dirSlash := relFilePath + string(filepath.Separator)

				for _, pat := range t.pm.Patterns() {
					if !pat.Exclusion() {
						continue
					}
					if strings.HasPrefix(pat.String()+string(filepath.Separator), dirSlash) {
						// found a match - so can't skip this dir
						return nil
					}
				}

				// No matching exclusion dir so just skip dir
				return filepath.SkipDir
			}

			if seen[relFilePath] {
				return nil
			}
			seen[relFilePath] = true

			// Rename the base resource.
			if rebaseName != "" {
				var replacement string
				if rebaseName != string(filepath.Separator) {
					// Special case the root directory to replace with an
					// empty string instead so that we don't end up with
					// double slashes in the paths.
					replacement = rebaseName
				}

				relFilePath = strings.Replace(relFilePath, include, replacement, 1)
			}

			if err := ta.addTarFile(filePath, relFilePath); err != nil {
				log.G(context.TODO()).Errorf("Can't add file %s to tar: %s", filePath, err)
				// if pipe is broken, stop writing tar stream to it
				if errors.Is(err, io.ErrClosedPipe) {
					return err
				}
			}
			return nil
		})
	}
}

// Unpack unpacks the decompressedArchive to dest with options.
func Unpack(decompressedArchive io.Reader, dest string, options *TarOptions) error {
	tr := tar.NewReader(decompressedArchive)

	var dirs []*tar.Header
	whiteoutConverter := getWhiteoutConverter(options.WhiteoutFormat)

	// Iterate through the files in the archive.
loop:
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}

		// ignore XGlobalHeader early to avoid creating parent directories for them
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			log.G(context.TODO()).Debugf("PAX Global Extended Headers found for %s and ignored", hdr.Name)
			continue
		}

		// Normalize name, for safety and for a simple is-root check
		// This keeps "../" as-is, but normalizes "/../" to "/". Or Windows:
		// This keeps "..\" as-is, but normalizes "\..\" to "\".
		hdr.Name = filepath.Clean(hdr.Name)

		for _, exclude := range options.ExcludePatterns {
			if strings.HasPrefix(hdr.Name, exclude) {
				continue loop
			}
		}

		// Ensure that the parent directory exists.
		err = createImpliedDirectories(dest, hdr, options)
		if err != nil {
			return err
		}

		// #nosec G305 -- The joined path is checked for path traversal.
		path := filepath.Join(dest, hdr.Name)
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return breakoutError(fmt.Errorf("%q is outside of %q", hdr.Name, dest))
		}

		// If path exits we almost always just want to remove and replace it
		// The only exception is when it is a directory *and* the file from
		// the layer is also a directory. Then we want to merge them (i.e.
		// just apply the metadata from the layer).
		if fi, err := os.Lstat(path); err == nil {
			if options.NoOverwriteDirNonDir && fi.IsDir() && hdr.Typeflag != tar.TypeDir {
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing directory with a non-directory from the archive.
				return fmt.Errorf("cannot overwrite directory %q with non-directory %q", path, dest)
			}

			if options.NoOverwriteDirNonDir && !fi.IsDir() && hdr.Typeflag == tar.TypeDir {
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing non-directory with a directory from the archive.
				return fmt.Errorf("cannot overwrite non-directory %q with directory %q", path, dest)
			}

			if fi.IsDir() && hdr.Name == "." {
				continue
			}

			if !fi.IsDir() || hdr.Typeflag != tar.TypeDir {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}

		if err := remapIDs(options.IDMap, hdr); err != nil {
			return err
		}

		if whiteoutConverter != nil {
			writeFile, err := whiteoutConverter.ConvertRead(hdr, path)
			if err != nil {
				return err
			}
			if !writeFile {
				continue
			}
		}

		if err := createTarFile(path, dest, hdr, tr, options); err != nil {
			return err
		}

		// Directory mtimes must be handled at the end to avoid further
		// file creation in them to modify the directory mtime
		if hdr.Typeflag == tar.TypeDir {
			dirs = append(dirs, hdr)
		}
	}

	for _, hdr := range dirs {
		// #nosec G305 -- The header was checked for path traversal before it was appended to the dirs slice.
		path := filepath.Join(dest, hdr.Name)

		if err := chtimes(path, boundTime(latestTime(hdr.AccessTime, hdr.ModTime)), boundTime(hdr.ModTime)); err != nil {
			return err
		}
	}
	return nil
}

// createImpliedDirectories will create all parent directories of the current path with default permissions, if they do
// not already exist. This is possible as the tar format supports 'implicit' directories, where their existence is
// defined by the paths of files in the tar, but there are no header entries for the directories themselves, and thus
// we most both create them and choose metadata like permissions.
//
// The caller should have performed filepath.Clean(hdr.Name), so hdr.Name will now be in the filepath format for the OS
// on which the daemon is running. This precondition is required because this function assumes a OS-specific path
// separator when checking that a path is not the root.
func createImpliedDirectories(dest string, hdr *tar.Header, options *TarOptions) error {
	// Not the root directory, ensure that the parent directory exists
	if !strings.HasSuffix(hdr.Name, string(os.PathSeparator)) {
		parent := filepath.Dir(hdr.Name)
		parentPath := filepath.Join(dest, parent)
		if _, err := os.Lstat(parentPath); err != nil && os.IsNotExist(err) {
			// RootPair() is confined inside this loop as most cases will not require a call, so we can spend some
			// unneeded function calls in the uncommon case to encapsulate logic -- implied directories are a niche
			// usage that reduces the portability of an image.
			uid, gid := options.IDMap.RootPair()

			err = user.MkdirAllAndChown(parentPath, ImpliedDirectoryMode, uid, gid, user.WithOnlyNew)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive may be compressed with one of the following algorithms:
// identity (uncompressed), gzip, bzip2, xz.
//
// FIXME: specify behavior when target path exists vs. doesn't exist.
func Untar(tarArchive io.Reader, dest string, options *TarOptions) error {
	return untarHandler(tarArchive, dest, options, true)
}

// UntarUncompressed reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive must be an uncompressed stream.
func UntarUncompressed(tarArchive io.Reader, dest string, options *TarOptions) error {
	return untarHandler(tarArchive, dest, options, false)
}

// Handler for teasing out the automatic decompression
func untarHandler(tarArchive io.Reader, dest string, options *TarOptions, decompress bool) error {
	if tarArchive == nil {
		return errors.New("empty archive")
	}
	dest = filepath.Clean(dest)
	if options == nil {
		options = &TarOptions{}
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}

	r := tarArchive
	if decompress {
		decompressedArchive, err := compression.DecompressStream(tarArchive)
		if err != nil {
			return err
		}
		defer decompressedArchive.Close()
		r = decompressedArchive
	}

	return Unpack(r, dest, options)
}

// TarUntar is a convenience function which calls Tar and Untar, with the output of one piped into the other.
// If either Tar or Untar fails, TarUntar aborts and returns the error.
func (archiver *Archiver) TarUntar(src, dst string) error {
	archive, err := Tar(src, compression.None)
	if err != nil {
		return err
	}
	defer archive.Close()
	return archiver.Untar(archive, dst, &TarOptions{
		IDMap: archiver.IDMapping,
	})
}

// UntarPath untar a file from path to a destination, src is the source tar file path.
func (archiver *Archiver) UntarPath(src, dst string) error {
	archive, err := os.Open(src)
	if err != nil {
		return err
	}
	defer archive.Close()
	return archiver.Untar(archive, dst, &TarOptions{
		IDMap: archiver.IDMapping,
	})
}

// CopyWithTar creates a tar archive of filesystem path `src`, and
// unpacks it at filesystem path `dst`.
// The archive is streamed directly with fixed buffering and no
// intermediary disk IO.
func (archiver *Archiver) CopyWithTar(src, dst string) error {
	srcSt, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcSt.IsDir() {
		return archiver.CopyFileWithTar(src, dst)
	}

	// if this Archiver is set up with ID mapping we need to create
	// the new destination directory with the remapped root UID/GID pair
	// as owner
	uid, gid := archiver.IDMapping.RootPair()
	// Create dst, copy src's content into it
	if err := user.MkdirAllAndChown(dst, 0o755, uid, gid, user.WithOnlyNew); err != nil {
		return err
	}
	return archiver.TarUntar(src, dst)
}

// CopyFileWithTar emulates the behavior of the 'cp' command-line
// for a single file. It copies a regular file from path `src` to
// path `dst`, and preserves all its metadata.
func (archiver *Archiver) CopyFileWithTar(src, dst string) (err error) {
	srcSt, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcSt.IsDir() {
		return errors.New("can't copy a directory")
	}

	// Clean up the trailing slash. This must be done in an operating
	// system specific manner.
	if dst[len(dst)-1] == os.PathSeparator {
		dst = filepath.Join(dst, filepath.Base(src))
	}
	// Create the holding directory if necessary
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}

	r, w := io.Pipe()
	errC := make(chan error, 1)

	go func() {
		defer close(errC)

		errC <- func() error {
			defer w.Close()

			srcF, err := os.Open(src)
			if err != nil {
				return err
			}
			defer srcF.Close()

			hdr, err := tarheader.FileInfoHeaderNoLookups(srcSt, "")
			if err != nil {
				return err
			}
			hdr.Format = tar.FormatPAX
			hdr.ModTime = hdr.ModTime.Truncate(time.Second)
			hdr.AccessTime = time.Time{}
			hdr.ChangeTime = time.Time{}
			hdr.Name = filepath.Base(dst)
			hdr.Mode = int64(chmodTarEntry(os.FileMode(hdr.Mode)))

			if err := remapIDs(archiver.IDMapping, hdr); err != nil {
				return err
			}

			tw := tar.NewWriter(w)
			defer tw.Close()
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if err := copyWithBuffer(tw, srcF); err != nil {
				return err
			}
			return nil
		}()
	}()
	defer func() {
		if er := <-errC; err == nil && er != nil {
			err = er
		}
	}()

	err = archiver.Untar(r, filepath.Dir(dst), nil)
	if err != nil {
		r.CloseWithError(err)
	}
	return err
}

// IdentityMapping returns the IdentityMapping of the archiver.
func (archiver *Archiver) IdentityMapping() user.IdentityMapping {
	return archiver.IDMapping
}

func remapIDs(idMapping user.IdentityMapping, hdr *tar.Header) error {
	uid, gid, err := idMapping.ToHost(hdr.Uid, hdr.Gid)
	hdr.Uid, hdr.Gid = uid, gid
	return err
}
