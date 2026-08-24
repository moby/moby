package launcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/log"
	"github.com/moby/extensions"
)

// Binaries lists executable extensions directly under dir.
// Each is named after its file, without .exe on Windows.
// A missing directory yields no binaries.
//
// Discovery is a root-code-execution boundary.
// World-writable entries, entries owned by an untrusted user, and files without
// valid extension ids are skipped.
// Other trust decisions, including group policy and symlinks, belong to the
// operator.
func Binaries(ctx context.Context, dir string) ([]string, error) {
	entries, err := extensionEntries(ctx, dir)
	if err != nil || entries == nil {
		return nil, err
	}
	var bins []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat extension %q: %w", filepath.Join(dir, entry.Name()), err)
		}
		if !isExecutable(info) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// The file name is the extension id. Validate it before launching so a
		// shared directory cannot execute arbitrary helper binaries.
		name := extensionName(entry.Name())
		if err := extensions.ValidateExtensionID(extensions.ExtensionID(name)); err != nil {
			log.G(ctx).WithError(err).Warnf("extensions: skipping %q: not a valid extension binary name", path)
			continue
		}
		if worldWritable(info) {
			log.G(ctx).Warnf("extensions: refusing to run world-writable extension binary %q", path)
			continue
		}
		if uid, untrusted := untrustedOwner(info); untrusted {
			log.G(ctx).Warnf("extensions: refusing to run extension binary %q owned by untrusted uid %d", path, uid)
			continue
		}
		bins = append(bins, path)
	}
	return bins, nil
}

func extensionEntries(ctx context.Context, dir string) ([]os.DirEntry, error) {
	dirInfo, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat extension dir %q: %w", dir, err)
	}
	if worldWritable(dirInfo) {
		log.G(ctx).Warnf("extensions: ignoring world-writable extension directory %q", dir)
		return nil, nil
	}
	if uid, untrusted := untrustedOwner(dirInfo); untrusted {
		log.G(ctx).Warnf("extensions: ignoring extension directory %q owned by untrusted uid %d", dir, uid)
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read extension dir %q: %w", dir, err)
	}
	return entries, nil
}

// untrustedOwner reports whether info is owned by a uid that is neither the
// superuser (0) nor the daemon's own effective user, returning that uid when so.
// A binary or directory owned by any other user could be rewritten by them and
// then executed as the daemon, so it is not trusted. This complements the
// world-writable check: that catches a file anyone can rewrite, this catches one
// a specific untrusted owner can. Ownership is not determinable on every platform
// (notably Windows, where access is governed by ACLs the mode does not reflect);
// there it is not enforced, and broader owner and group policy remains the
// operator's, per the security model in the design docs.
func untrustedOwner(info fs.FileInfo) (int, bool) {
	uid, ok := fileUID(info)
	if !ok {
		return 0, false
	}
	if uid == 0 || uid == os.Geteuid() {
		return 0, false
	}
	return uid, true
}

func isExecutable(info fs.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(info.Name()), ".exe")
	}
	return info.Mode().Perm()&0o111 != 0
}

func extensionName(file string) string {
	if runtime.GOOS == "windows" {
		ext := filepath.Ext(file)
		if strings.EqualFold(ext, ".exe") {
			return strings.TrimSuffix(file, ext)
		}
	}
	return strings.TrimSuffix(file, ".exe")
}

// worldWritable reports whether info is writable by others (the o+w bit). A
// world-writable binary or directory on the daemon's exec path lets any local
// user run code as the daemon, so it is not trusted. The bit is only meaningful
// on Unix; on Windows access is governed by ACLs the mode does not reflect, so
// this check does not apply there.
func worldWritable(info fs.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return info.Mode().Perm()&0o002 != 0
}
