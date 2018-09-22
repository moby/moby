package continuity

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/continuity/devices"
	driverpkg "github.com/containerd/continuity/driver"
	"github.com/containerd/continuity/pathdriver"

	"github.com/opencontainers/go-digest"
)

var (
	// ErrNotFound represents the resource not found
	ErrNotFound = fmt.Errorf("not found")
	// ErrNotSupported represents the resource not supported
	ErrNotSupported = fmt.Errorf("not supported")
)

// Context represents a file system context for accessing resources. The
// responsibility of the context is to convert system specific resources to
// generic Resource objects. Most of this is safe path manipulation, as well
// as extraction of resource details.
type Context interface {
	Apply(Resource) error
	Verify(Resource) error
	Resource(string, os.FileInfo) (Resource, error)
	Walk(filepath.WalkFunc) error
}

// SymlinkPath is intended to give the symlink target value
// in a root context. Target and linkname are absolute paths
// not under the given root.
type SymlinkPath func(root, linkname, target string) (string, error)

// ContextOptions represents options to create a new context.
type ContextOptions struct {
	Digester   Digester
	Driver     driverpkg.Driver
	PathDriver pathdriver.PathDriver
	Provider   ContentProvider
}

// context represents a file system context for accessing resources.
// Generally, all path qualified access and system considerations should land
// here.
type context struct {
	driver     driverpkg.Driver
	pathDriver pathdriver.PathDriver
	root       string
	digester   Digester
	provider   ContentProvider
}

// NewContext returns a Context associated with root. The default driver will
// be used, as returned by NewDriver.
func NewContext(root string) (Context, error) {
	return NewContextWithOptions(root, ContextOptions{})
}

// NewContextWithOptions returns a Context associate with the root.
func NewContextWithOptions(root string, options ContextOptions) (Context, error) {
	// normalize to absolute path
	pathDriver := options.PathDriver
	if pathDriver == nil {
		pathDriver = pathdriver.LocalPathDriver
	}

	root = pathDriver.FromSlash(root)
	root, err := pathDriver.Abs(pathDriver.Clean(root))
	if err != nil {
		return nil, err
	}

	driver := options.Driver
	if driver == nil {
		driver, err = driverpkg.NewSystemDriver()
		if err != nil {
			return nil, err
		}
	}

	digester := options.Digester
	if digester == nil {
		digester = simpleDigester{digest.Canonical}
	}

	// Check the root directory. Need to be a little careful here. We are
	// allowing a link for now, but this may have odd behavior when
	// canonicalizing paths. As long as all files are opened through the link
	// path, this should be okay.
	fi, err := driver.Stat(root)
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		return nil, &os.PathError{Op: "NewContext", Path: root, Err: os.ErrInvalid}
	}

	return &context{
		root:       root,
		driver:     driver,
		pathDriver: pathDriver,
		digester:   digester,
		provider:   options.Provider,
	}, nil
}

// Resource returns the resource as path p, populating the entry with info
// from fi. The path p should be the path of the resource in the context,
// typically obtained through Walk or from the value of Resource.Path(). If fi
// is nil, it will be resolved.
func (c *context) Resource(p string, fi os.FileInfo) (Resource, error) {
	fp, err := c.fullpath(p)
	if err != nil {
		return nil, err
	}

	if fi == nil {
		fi, err = c.driver.Lstat(fp)
		if err != nil {
			return nil, err
		}
	}

	base, err := newBaseResource(p, fi)
	if err != nil {
		return nil, err
	}

	base.xattrs, err = c.resolveXAttrs(fp, fi, base)
	if err == ErrNotSupported {
		log.Printf("resolving xattrs on %s not supported", fp)
	} else if err != nil {
		return nil, err
	}

	// TODO(stevvooe): Handle windows alternate data streams.

	if fi.Mode().IsRegular() {
		dgst, err := c.digest(p)
		if err != nil {
			return nil, err
		}

		return newRegularFile(*base, base.paths, fi.Size(), dgst)
	}

	if fi.Mode().IsDir() {
		return newDirectory(*base)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		// We handle relative links vs absolute links by including a
		// beginning slash for absolute links. Effectively, the bundle's
		// root is treated as the absolute link anchor.
		target, err := c.driver.Readlink(fp)
		if err != nil {
			return nil, err
		}

		return newSymLink(*base, target)
	}

	if fi.Mode()&os.ModeNamedPipe != 0 {
		return newNamedPipe(*base, base.paths)
	}

	if fi.Mode()&os.ModeDevice != 0 {
		deviceDriver, ok := c.driver.(driverpkg.DeviceInfoDriver)
		if !ok {
			log.Printf("device extraction not supported %s", fp)
			return nil, ErrNotSupported
		}

		// character and block devices merely need to recover the
		// major/minor device number.
		major, minor, err := deviceDriver.DeviceInfo(fi)
		if err != nil {
			return nil, err
		}

		return newDevice(*base, base.paths, major, minor)
	}

	log.Printf("%q (%v) is not supported", fp, fi.Mode())
	return nil, ErrNotFound
}

func (c *context) verifyMetadata(resource, target Resource) error {
	if target.Mode() != resource.Mode() {
		return fmt.Errorf("resource %q has incorrect mode: %v != %v", target.Path(), target.Mode(), resource.Mode())
	}

	if target.UID() != resource.UID() {
		return fmt.Errorf("unexpected uid for %q: %v != %v", target.Path(), target.UID(), resource.GID())
	}

	if target.GID() != resource.GID() {
		return fmt.Errorf("unexpected gid for %q: %v != %v", target.Path(), target.GID(), target.GID())
	}

	if xattrer, ok := resource.(XAttrer); ok {
		txattrer, tok := target.(XAttrer)
		if !tok {
			return fmt.Errorf("resource %q has xattrs but target does not support them", resource.Path())
		}

		// For xattrs, only ensure that we have those defined in the resource
		// and their values match. We can ignore other xattrs. In other words,
		// we only verify that target has the subset defined by resource.
		txattrs := txattrer.XAttrs()
		for attr, value := range xattrer.XAttrs() {
			tvalue, ok := txattrs[attr]
			if !ok {
				return fmt.Errorf("resource %q target missing xattr %q", resource.Path(), attr)
			}

			if !bytes.Equal(value, tvalue) {
				return fmt.Errorf("xattr %q value differs for resource %q", attr, resource.Path())
			}
		}
	}

	switch r := resource.(type) {
	case RegularFile:
		// TODO(stevvooe): Another reason to use a record-based approach. We
		// have to do another type switch to get this to work. This could be
		// fixed with an Equal function, but let's study this a little more to
		// be sure.
		t, ok := target.(RegularFile)
		if !ok {
			return fmt.Errorf("resource %q target not a regular file", r.Path())
		}

		if t.Size() != r.Size() {
			return fmt.Errorf("resource %q target has incorrect size: %v != %v", t.Path(), t.Size(), r.Size())
		}
	case Directory:
		t, ok := target.(Directory)
		if !ok {
			return fmt.Errorf("resource %q target not a directory", t.Path())
		}
	case SymLink:
		t, ok := target.(SymLink)
		if !ok {
			return fmt.Errorf("resource %q target not a symlink", t.Path())
		}

		if t.Target() != r.Target() {
			return fmt.Errorf("resource %q target has mismatched target: %q != %q", t.Path(), t.Target(), r.Target())
		}
	case Device:
		t, ok := target.(Device)
		if !ok {
			return fmt.Errorf("resource %q is not a device", t.Path())
		}

		if t.Major() != r.Major() || t.Minor() != r.Minor() {
			return fmt.Errorf("resource %q has mismatched major/minor numbers: %d,%d != %d,%d", t.Path(), t.Major(), t.Minor(), r.Major(), r.Minor())
		}
	case NamedPipe:
		t, ok := target.(NamedPipe)
		if !ok {
			return fmt.Errorf("resource %q is not a named pipe", t.Path())
		}
	default:
		return fmt.Errorf("cannot verify resource: %v", resource)
	}

	return nil
}

// Verify the resource in the context. An error will be returned a discrepancy
// is found.
func (c *context) Verify(resource Resource) error {
	fp, err := c.fullpath(resource.Path())
	if err != nil {
		return err
	}

	fi, err := c.driver.Lstat(fp)
	if err != nil {
		return err
	}

	target, err := c.Resource(resource.Path(), fi)
	if err != nil {
		return err
	}

	if target.Path() != resource.Path() {
		return fmt.Errorf("resource paths do not match: %q != %q", target.Path(), resource.Path())
	}

	if err := c.verifyMetadata(resource, target); err != nil {
		return err
	}

	if h, isHardlinkable := resource.(Hardlinkable); isHardlinkable {
		hardlinkKey, err := newHardlinkKey(fi)
		if err == errNotAHardLink {
			if len(h.Paths()) > 1 {
				return fmt.Errorf("%q is not a hardlink to %q", h.Paths()[1], resource.Path())
			}
		} else if err != nil {
			return err
		}

		for _, path := range h.Paths()[1:] {
			fpLink, err := c.fullpath(path)
			if err != nil {
				return err
			}

			fiLink, err := c.driver.Lstat(fpLink)
			if err != nil {
				return err
			}

			targetLink, err := c.Resource(path, fiLink)
			if err != nil {
				return err
			}

			hardlinkKeyLink, err := newHardlinkKey(fiLink)
			if err != nil {
				return err
			}

			if hardlinkKeyLink != hardlinkKey {
				return fmt.Errorf("%q is not a hardlink to %q", path, resource.Path())
			}

			if err := c.verifyMetadata(resource, targetLink); err != nil {
				return err
			}
		}
	}

	switch r := resource.(type) {
	case RegularFile:
		t, ok := target.(RegularFile)
		if !ok {
			return fmt.Errorf("resource %q target not a regular file", r.Path())
		}

		// TODO(stevvooe): This may need to get a little more sophisticated
		// for digest comparison. We may want to actually calculate the
		// provided digests, rather than the implementations having an
		// overlap.
		if !digestsMatch(t.Digests(), r.Digests()) {
			return fmt.Errorf("digests for resource %q do not match: %v != %v", t.Path(), t.Digests(), r.Digests())
		}
	}

	return nil
}

func (c *context) checkoutFile(fp string, rf RegularFile) error {
	if c.provider == nil {
		return fmt.Errorf("no file provider")
	}
	var (
		r   io.ReadCloser
		err error
	)
	for _, dgst := range rf.Digests() {
		r, err = c.provider.Reader(dgst)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("file content could not be provided: %v", err)
	}
	defer r.Close()

	return atomicWriteFile(fp, r, rf.Size(), rf.Mode())
}

// Apply the resource to the contexts. An error will be returned if the
// operation fails. Depending on the resource type, the resource may be
// created. For resource that cannot be resolved, an error will be returned.
func (c *context) Apply(resource Resource) error {
	fp, err := c.fullpath(resource.Path())
	if err != nil {
		return err
	}

	if !strings.HasPrefix(fp, c.root) {
		return fmt.Errorf("resource %v escapes root", resource)
	}

	var chmod = true
	fi, err := c.driver.Lstat(fp)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	switch r := resource.(type) {
	case RegularFile:
		if fi == nil {
			if err := c.checkoutFile(fp, r); err != nil {
				return fmt.Errorf("error checking out file %q: %v", resource.Path(), err)
			}
			chmod = false
		} else {
			if !fi.Mode().IsRegular() {
				return fmt.Errorf("file %q should be a regular file, but is not", resource.Path())
			}
			if fi.Size() != r.Size() {
				if err := c.checkoutFile(fp, r); err != nil {
					return fmt.Errorf("error checking out file %q: %v", resource.Path(), err)
				}
			} else {
				for _, dgst := range r.Digests() {
					f, err := os.Open(fp)
					if err != nil {
						return fmt.Errorf("failure opening file for read %q: %v", resource.Path(), err)
					}
					compared, err := dgst.Algorithm().FromReader(f)
					if err == nil && dgst != compared {
						if err := c.checkoutFile(fp, r); err != nil {
							return fmt.Errorf("error checking out file %q: %v", resource.Path(), err)
						}
						break
					}
					if err1 := f.Close(); err == nil {
						err = err1
					}
					if err != nil {
						return fmt.Errorf("error checking digest for %q: %v", resource.Path(), err)
					}
				}
			}
		}
	case Directory:
		if fi == nil {
			if err := c.driver.Mkdir(fp, resource.Mode()); err != nil {
				return err
			}
		} else if !fi.Mode().IsDir() {
			return fmt.Errorf("%q should be a directory, but is not", resource.Path())
		}

	case SymLink:
		var target string // only possibly set if target resource is a symlink

		if fi != nil {
			if fi.Mode()&os.ModeSymlink != 0 {
				target, err = c.driver.Readlink(fp)
				if err != nil {
					return err
				}
			}
		}

		if target != r.Target() {
			if fi != nil {
				if err := c.driver.Remove(fp); err != nil { // RemoveAll in case of directory?
					return err
				}
			}

			if err := c.driver.Symlink(r.Target(), fp); err != nil {
				return err
			}
		}

	case Device:
		if fi == nil {
			if err := c.driver.Mknod(fp, resource.Mode(), int(r.Major()), int(r.Minor())); err != nil {
				return err
			}
		} else if (fi.Mode() & os.ModeDevice) == 0 {
			return fmt.Errorf("%q should be a device, but is not", resource.Path())
		} else {
			major, minor, err := devices.DeviceInfo(fi)
			if err != nil {
				return err
			}
			if major != r.Major() || minor != r.Minor() {
				if err := c.driver.Remove(fp); err != nil {
					return err
				}

				if err := c.driver.Mknod(fp, resource.Mode(), int(r.Major()), int(r.Minor())); err != nil {
					return err
				}
			}
		}

	case NamedPipe:
		if fi == nil {
			if err := c.driver.Mkfifo(fp, resource.Mode()); err != nil {
				return err
			}
		} else if (fi.Mode() & os.ModeNamedPipe) == 0 {
			return fmt.Errorf("%q should be a named pipe, but is not", resource.Path())
		}
	}

	if h, isHardlinkable := resource.(Hardlinkable); isHardlinkable {
		for _, path := range h.Paths() {
			if path == resource.Path() {
				continue
			}

			lp, err := c.fullpath(path)
			if err != nil {
				return err
			}

			if _, fi := c.driver.Lstat(lp); fi == nil {
				c.driver.Remove(lp)
			}
			if err := c.driver.Link(fp, lp); err != nil {
				return err
			}
		}
	}

	// Update filemode if file was not created
	if chmod {
		if err := c.driver.Lchmod(fp, resource.Mode()); err != nil {
			return err
		}
	}

	if err := c.driver.Lchown(fp, resource.UID(), resource.GID()); err != nil {
		return err
	}

	if xattrer, ok := resource.(XAttrer); ok {
		// For xattrs, only ensure that we have those defined in the resource
		// and their values are set. We can ignore other xattrs. In other words,
		// we only set xattres defined by resource but never remove.

		if _, ok := resource.(SymLink); ok {
			lxattrDriver, ok := c.driver.(driverpkg.LXAttrDriver)
			if !ok {
				return fmt.Errorf("unsupported symlink xattr for resource %q", resource.Path())
			}
			if err := lxattrDriver.LSetxattr(fp, xattrer.XAttrs()); err != nil {
				return err
			}
		} else {
			xattrDriver, ok := c.driver.(driverpkg.XAttrDriver)
			if !ok {
				return fmt.Errorf("unsupported xattr for resource %q", resource.Path())
			}
			if err := xattrDriver.Setxattr(fp, xattrer.XAttrs()); err != nil {
				return err
			}
		}
	}

	return nil
}

// Walk provides a convenience function to call filepath.Walk correctly for
// the context. Otherwise identical to filepath.Walk, the path argument is
// corrected to be contained within the context.
func (c *context) Walk(fn filepath.WalkFunc) error {
	root := c.root
	fi, err := c.driver.Lstat(c.root)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		root, err = c.driver.Readlink(c.root)
		if err != nil {
			return err
		}
	}
	return c.pathDriver.Walk(root, func(p string, fi os.FileInfo, err error) error {
		contained, err := c.containWithRoot(p, root)
		return fn(contained, fi, err)
	})
}

// fullpath returns the system path for the resource, joined with the context
// root. The path p must be a part of the context.
func (c *context) fullpath(p string) (string, error) {
	p = c.pathDriver.Join(c.root, p)
	if !strings.HasPrefix(p, c.root) {
		return "", fmt.Errorf("invalid context path")
	}

	return p, nil
}

// contain cleans and santizes the filesystem path p to be an absolute path,
// effectively relative to the context root.
func (c *context) contain(p string) (string, error) {
	return c.containWithRoot(p, c.root)
}

// containWithRoot cleans and santizes the filesystem path p to be an absolute path,
// effectively relative to the passed root. Extra care should be used when calling this
// instead of contain. This is needed for Walk, as if context root is a symlink,
// it must be evaluated prior to the Walk
func (c *context) containWithRoot(p string, root string) (string, error) {
	sanitized, err := c.pathDriver.Rel(root, p)
	if err != nil {
		return "", err
	}

	// ZOMBIES(stevvooe): In certain cases, we may want to remap these to a
	// "containment error", so the caller can decide what to do.
	return c.pathDriver.Join("/", c.pathDriver.Clean(sanitized)), nil
}

// digest returns the digest of the file at path p, relative to the root.
func (c *context) digest(p string) (digest.Digest, error) {
	f, err := c.driver.Open(c.pathDriver.Join(c.root, p))
	if err != nil {
		return "", err
	}
	defer f.Close()

	return c.digester.Digest(f)
}

// resolveXAttrs attempts to resolve the extended attributes for the resource
// at the path fp, which is the full path to the resource. If the resource
// cannot have xattrs, nil will be returned.
func (c *context) resolveXAttrs(fp string, fi os.FileInfo, base *resource) (map[string][]byte, error) {
	if fi.Mode().IsRegular() || fi.Mode().IsDir() {
		xattrDriver, ok := c.driver.(driverpkg.XAttrDriver)
		if !ok {
			log.Println("xattr extraction not supported")
			return nil, ErrNotSupported
		}

		return xattrDriver.Getxattr(fp)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		lxattrDriver, ok := c.driver.(driverpkg.LXAttrDriver)
		if !ok {
			log.Println("xattr extraction for symlinks not supported")
			return nil, ErrNotSupported
		}

		return lxattrDriver.LGetxattr(fp)
	}

	return nil, nil
}
