package archive

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/containerd/log"

	"github.com/moby/go-archive/compression"
)

// UnpackLayer unpack `layer` to a `dest`. The stream `layer` can be
// compressed or uncompressed.
// Returns the size in bytes of the contents of the layer.
func UnpackLayer(dest string, layer io.Reader, options *TarOptions) (size int64, err error) {
	root, err := os.OpenRoot(dest)
	if err != nil {
		return 0, err
	}
	defer root.Close()

	tr := tar.NewReader(layer)

	var dirs []unpackedDir
	// unpackedPaths tracks root-relative paths already written in this layer
	// so that the AUFS opaque-whiteout walk knows which paths to preserve.
	unpackedPaths := make(map[string]struct{})

	if options == nil {
		options = &TarOptions{}
	}

	aufsTempdir := ""
	aufsHardlinks := make(map[string]*tar.Header)

	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			// end of tar archive
			break
		}
		if err != nil {
			return 0, err
		}

		size += hdr.Size

		// Strip a leading "/" so absolute entries stay root-relative, and
		// normalize the POSIX tar path. Skip entries referring to the extraction
		// root and reject paths that escape it.
		name := path.Clean(strings.TrimLeft(hdr.Name, "/"))
		if name == "." {
			continue
		}
		if !filepath.IsLocal(name) {
			return 0, breakoutError(fmt.Errorf("invalid entry name %q", hdr.Name))
		}
		hdr.Name = name

		// Skip entries whose name (or hardlink target) Windows cannot represent.
		if err := unrepresentableOnWindows(hdr); err != nil {
			log.G(context.TODO()).Warnf("Windows: ignoring entry: %v", err)
			continue
		}

		// Ensure that the parent directory exists.
		err = createImpliedDirectories(root, hdr, options)
		if err != nil {
			return 0, err
		}

		// Skip AUFS metadata dirs
		if strings.HasPrefix(hdr.Name, WhiteoutMetaPrefix) {
			// Regular files inside /.wh..wh.plnk can be used as hardlink targets
			// We don't want this directory, but we need the files in them so that
			// such hardlinks can be resolved.
			if strings.HasPrefix(hdr.Name, WhiteoutLinkDir) && hdr.Typeflag == tar.TypeReg {
				basename := path.Base(hdr.Name)
				aufsHardlinks[basename] = hdr
				if aufsTempdir == "" {
					if aufsTempdir, err = os.MkdirTemp(dest, "dockerplnk"); err != nil {
						return 0, err
					}
					defer os.RemoveAll(aufsTempdir)
				}
				aufsRoot, err := os.OpenRoot(aufsTempdir)
				if err != nil {
					return 0, err
				}
				cerr := createTarFile(aufsRoot, basename, hdr, tr, options)
				_ = aufsRoot.Close()
				if cerr != nil {
					return 0, cerr
				}
			}

			if hdr.Name != WhiteoutOpaqueDir {
				continue
			}
		}
		// dstPath is the native (host-separator) form of the entry name,
		// used at all filesystem boundaries (os.Root methods, fsRootPath).
		// The tar-header name (hdr.Name) is POSIX, so convert it here.
		dstPath := filepath.FromSlash(hdr.Name)
		base := filepath.Base(dstPath)

		if strings.HasPrefix(base, WhiteoutPrefix) {
			dir := filepath.Dir(dstPath)
			if base == WhiteoutOpaqueDir {
				_, err := root.Lstat(dir)
				if err != nil {
					return 0, err
				}
				// Walk the absolute directory so we can call os.RemoveAll on
				// paths outside the walk callback's reach, then convert each
				// walked path back to a root-relative name for the
				// unpackedPaths check.
				// fsRootPath walks each path component and bounds any symlinks
				// within the root to prevent TOCTOU symlink attacks.
				absDir, err := fsRootPath(root.Name(), dir)
				if err != nil {
					return 0, err
				}
				err = filepath.WalkDir(absDir, func(p string, info os.DirEntry, err error) error {
					if err != nil {
						if os.IsNotExist(err) {
							return nil // parent was deleted
						}
						return err
					}
					if p == absDir {
						return nil
					}
					rel, err := filepath.Rel(root.Name(), p)
					if err != nil {
						return err
					}

					// unpackedPaths is keyed by root-relative slash paths; convert
					// filepath.WalkDir's native path before looking it up.
					if _, exists := unpackedPaths[filepath.ToSlash(rel)]; !exists {
						return root.RemoveAll(rel)
					}
					return nil
				})
				if err != nil {
					return 0, err
				}
			} else {
				originalBase := base[len(WhiteoutPrefix):]
				originalPath := filepath.Join(dir, originalBase)
				if err := root.RemoveAll(originalPath); err != nil {
					return 0, err
				}
			}
		} else {
			// If dstPath exists we almost always just want to remove and replace it.
			// The only exception is when it is a directory *and* the file from
			// the layer is also a directory. Then we want to merge them (i.e.
			// just apply the metadata from the layer).
			if fi, err := root.Lstat(dstPath); err == nil {
				if !fi.IsDir() || hdr.Typeflag != tar.TypeDir {
					if err := root.RemoveAll(dstPath); err != nil {
						return 0, err
					}
				}
			}

			srcData := io.Reader(tr)
			srcHdr := hdr

			// Hard links into /.wh..wh.plnk don't work, as we don't extract that directory, so
			// we manually retarget these into the temporary files we extracted them into
			if hdr.Typeflag == tar.TypeLink && strings.HasPrefix(path.Clean(hdr.Linkname), WhiteoutLinkDir) {
				linkBasename := path.Base(hdr.Linkname)
				srcHdr = aufsHardlinks[linkBasename]
				if srcHdr == nil {
					return 0, errors.New("invalid aufs hardlink")
				}
				tmpFile, err := os.Open(filepath.Join(aufsTempdir, linkBasename))
				if err != nil {
					return 0, err
				}
				defer tmpFile.Close()
				srcData = tmpFile
			}

			if err := remapIDs(options.IDMap, srcHdr); err != nil {
				return 0, err
			}

			if err := createTarFile(root, dstPath, srcHdr, srcData, options); err != nil {
				return 0, err
			}

			// Directory mtimes must be handled at the end to avoid further
			// file creation in them to modify the directory mtime
			if hdr.Typeflag == tar.TypeDir {
				dirs = append(dirs, unpackedDir{hdr: hdr, name: dstPath})
			}
			// unpackedPaths is keyed by the POSIX (forward-slash) name so it
			// matches the ToSlash'd lookup in the opaque-whiteout walk above.
			unpackedPaths[hdr.Name] = struct{}{}
		}
	}

	for _, d := range dirs {
		if err := root.Chtimes(d.name, boundTime(latestTime(d.hdr.AccessTime, d.hdr.ModTime)), boundTime(d.hdr.ModTime)); err != nil {
			return 0, err
		}
	}

	return size, nil
}

// ApplyLayer parses a diff in the standard layer format from `layer`,
// and applies it to the directory `dest`. The stream `layer` can be
// compressed or uncompressed.
// Returns the size in bytes of the contents of the layer.
func ApplyLayer(dest string, layer io.Reader) (int64, error) {
	return applyLayerHandler(dest, layer, &TarOptions{}, true)
}

// ApplyUncompressedLayer parses a diff in the standard layer format from
// `layer`, and applies it to the directory `dest`. The stream `layer`
// can only be uncompressed.
// Returns the size in bytes of the contents of the layer.
func ApplyUncompressedLayer(dest string, layer io.Reader, options *TarOptions) (int64, error) {
	return applyLayerHandler(dest, layer, options, false)
}

// IsEmpty checks if the tar archive is empty (doesn't contain any entries).
func IsEmpty(rd io.Reader) (bool, error) {
	decompRd, err := compression.DecompressStream(rd)
	if err != nil {
		return true, fmt.Errorf("failed to decompress archive: %w", err)
	}
	defer decompRd.Close()

	tarReader := tar.NewReader(decompRd)
	if _, err := tarReader.Next(); err != nil {
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		return false, fmt.Errorf("failed to read next archive header: %w", err)
	}

	return false, nil
}

// do the bulk load of ApplyLayer, but allow for not calling DecompressStream
func applyLayerHandler(dest string, layer io.Reader, options *TarOptions, decompress bool) (int64, error) {
	dest = filepath.Clean(dest)

	// We need to be able to set any perms
	restore := overrideUmask(0)
	defer restore()

	if decompress {
		decompLayer, err := compression.DecompressStream(layer)
		if err != nil {
			return 0, err
		}
		defer decompLayer.Close()
		layer = decompLayer
	}
	return UnpackLayer(dest, layer, options)
}
