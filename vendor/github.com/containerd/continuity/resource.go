package continuity

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"

	pb "github.com/containerd/continuity/proto"
	"github.com/opencontainers/go-digest"
)

// TODO(stevvooe): A record based model, somewhat sketched out at the bottom
// of this file, will be more flexible. Another possibly is to tie the package
// interface directly to the protobuf type. This will have efficiency
// advantages at the cost coupling the nasty codegen types to the exported
// interface.

type Resource interface {
	// Path provides the primary resource path relative to the bundle root. In
	// cases where resources have more than one path, such as with hard links,
	// this will return the primary path, which is often just the first entry.
	Path() string

	// Mode returns the
	Mode() os.FileMode

	UID() int64
	GID() int64
}

// ByPath provides the canonical sort order for a set of resources. Use with
// sort.Stable for deterministic sorting.
type ByPath []Resource

func (bp ByPath) Len() int           { return len(bp) }
func (bp ByPath) Swap(i, j int)      { bp[i], bp[j] = bp[j], bp[i] }
func (bp ByPath) Less(i, j int) bool { return bp[i].Path() < bp[j].Path() }

type XAttrer interface {
	XAttrs() map[string][]byte
}

// Hardlinkable is an interface that a resource type satisfies if it can be a
// hardlink target.
type Hardlinkable interface {
	// Paths returns all paths of the resource, including the primary path
	// returned by Resource.Path. If len(Paths()) > 1, the resource is a hard
	// link.
	Paths() []string
}

type RegularFile interface {
	Resource
	XAttrer
	Hardlinkable

	Size() int64
	Digests() []digest.Digest
}

// Merge two or more Resources into new file. Typically, this should be
// used to merge regular files as hardlinks. If the files are not identical,
// other than Paths and Digests, the merge will fail and an error will be
// returned.
func Merge(fs ...Resource) (Resource, error) {
	if len(fs) < 1 {
		return nil, fmt.Errorf("please provide a resource to merge")
	}

	if len(fs) == 1 {
		return fs[0], nil
	}

	var paths []string
	var digests []digest.Digest
	bypath := map[string][]Resource{}

	// The attributes are all compared against the first to make sure they
	// agree before adding to the above collections. If any of these don't
	// correctly validate, the merge fails.
	prototype := fs[0]
	xattrs := make(map[string][]byte)

	// initialize xattrs for use below. All files must have same xattrs.
	if prototypeXAttrer, ok := prototype.(XAttrer); ok {
		for attr, value := range prototypeXAttrer.XAttrs() {
			xattrs[attr] = value
		}
	}

	for _, f := range fs {
		h, isHardlinkable := f.(Hardlinkable)
		if !isHardlinkable {
			return nil, errNotAHardLink
		}

		if f.Mode() != prototype.Mode() {
			return nil, fmt.Errorf("modes do not match: %v != %v", f.Mode(), prototype.Mode())
		}

		if f.UID() != prototype.UID() {
			return nil, fmt.Errorf("uid does not match: %v != %v", f.UID(), prototype.UID())
		}

		if f.GID() != prototype.GID() {
			return nil, fmt.Errorf("gid does not match: %v != %v", f.GID(), prototype.GID())
		}

		if xattrer, ok := f.(XAttrer); ok {
			fxattrs := xattrer.XAttrs()
			if !reflect.DeepEqual(fxattrs, xattrs) {
				return nil, fmt.Errorf("resource %q xattrs do not match: %v != %v", f, fxattrs, xattrs)
			}
		}

		for _, p := range h.Paths() {
			pfs, ok := bypath[p]
			if !ok {
				// ensure paths are unique by only appending on a new path.
				paths = append(paths, p)
			}

			bypath[p] = append(pfs, f)
		}

		if regFile, isRegFile := f.(RegularFile); isRegFile {
			prototypeRegFile, prototypeIsRegFile := prototype.(RegularFile)
			if !prototypeIsRegFile {
				return nil, errors.New("prototype is not a regular file")
			}

			if regFile.Size() != prototypeRegFile.Size() {
				return nil, fmt.Errorf("size does not match: %v != %v", regFile.Size(), prototypeRegFile.Size())
			}

			digests = append(digests, regFile.Digests()...)
		} else if device, isDevice := f.(Device); isDevice {
			prototypeDevice, prototypeIsDevice := prototype.(Device)
			if !prototypeIsDevice {
				return nil, errors.New("prototype is not a device")
			}

			if device.Major() != prototypeDevice.Major() {
				return nil, fmt.Errorf("major number does not match: %v != %v", device.Major(), prototypeDevice.Major())
			}
			if device.Minor() != prototypeDevice.Minor() {
				return nil, fmt.Errorf("minor number does not match: %v != %v", device.Minor(), prototypeDevice.Minor())
			}
		} else if _, isNamedPipe := f.(NamedPipe); isNamedPipe {
			_, prototypeIsNamedPipe := prototype.(NamedPipe)
			if !prototypeIsNamedPipe {
				return nil, errors.New("prototype is not a named pipe")
			}
		} else {
			return nil, errNotAHardLink
		}
	}

	sort.Stable(sort.StringSlice(paths))

	// Choose a "canonical" file. Really, it is just the first file to sort
	// against. We also effectively select the very first digest as the
	// "canonical" one for this file.
	first := bypath[paths[0]][0]

	resource := resource{
		paths:  paths,
		mode:   first.Mode(),
		uid:    first.UID(),
		gid:    first.GID(),
		xattrs: xattrs,
	}

	switch typedF := first.(type) {
	case RegularFile:
		var err error
		digests, err = uniqifyDigests(digests...)
		if err != nil {
			return nil, err
		}

		return &regularFile{
			resource: resource,
			size:     typedF.Size(),
			digests:  digests,
		}, nil
	case Device:
		return &device{
			resource: resource,
			major:    typedF.Major(),
			minor:    typedF.Minor(),
		}, nil

	case NamedPipe:
		return &namedPipe{
			resource: resource,
		}, nil

	default:
		return nil, errNotAHardLink
	}
}

type Directory interface {
	Resource
	XAttrer

	// Directory is a no-op method to identify directory objects by interface.
	Directory()
}

type SymLink interface {
	Resource

	// Target returns the target of the symlink contained in the .
	Target() string
}

type NamedPipe interface {
	Resource
	Hardlinkable
	XAttrer

	// Pipe is a no-op method to allow consistent resolution of NamedPipe
	// interface.
	Pipe()
}

type Device interface {
	Resource
	Hardlinkable
	XAttrer

	Major() uint64
	Minor() uint64
}

type resource struct {
	paths    []string
	mode     os.FileMode
	uid, gid int64
	xattrs   map[string][]byte
}

var _ Resource = &resource{}

func (r *resource) Path() string {
	if len(r.paths) < 1 {
		return ""
	}

	return r.paths[0]
}

func (r *resource) Mode() os.FileMode {
	return r.mode
}

func (r *resource) UID() int64 {
	return r.uid
}

func (r *resource) GID() int64 {
	return r.gid
}

type regularFile struct {
	resource
	size    int64
	digests []digest.Digest
}

var _ RegularFile = &regularFile{}

// newRegularFile returns the RegularFile, using the populated base resource
// and one or more digests of the content.
func newRegularFile(base resource, paths []string, size int64, dgsts ...digest.Digest) (RegularFile, error) {
	if !base.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}

	base.paths = make([]string, len(paths))
	copy(base.paths, paths)

	// make our own copy of digests
	ds := make([]digest.Digest, len(dgsts))
	copy(ds, dgsts)

	return &regularFile{
		resource: base,
		size:     size,
		digests:  ds,
	}, nil
}

func (rf *regularFile) Paths() []string {
	paths := make([]string, len(rf.paths))
	copy(paths, rf.paths)
	return paths
}

func (rf *regularFile) Size() int64 {
	return rf.size
}

func (rf *regularFile) Digests() []digest.Digest {
	digests := make([]digest.Digest, len(rf.digests))
	copy(digests, rf.digests)
	return digests
}

func (rf *regularFile) XAttrs() map[string][]byte {
	xattrs := make(map[string][]byte, len(rf.xattrs))

	for attr, value := range rf.xattrs {
		xattrs[attr] = append(xattrs[attr], value...)
	}

	return xattrs
}

type directory struct {
	resource
}

var _ Directory = &directory{}

func newDirectory(base resource) (Directory, error) {
	if !base.Mode().IsDir() {
		return nil, fmt.Errorf("not a directory")
	}

	return &directory{
		resource: base,
	}, nil
}

func (d *directory) Directory() {}

func (d *directory) XAttrs() map[string][]byte {
	xattrs := make(map[string][]byte, len(d.xattrs))

	for attr, value := range d.xattrs {
		xattrs[attr] = append(xattrs[attr], value...)
	}

	return xattrs
}

type symLink struct {
	resource
	target string
}

var _ SymLink = &symLink{}

func newSymLink(base resource, target string) (SymLink, error) {
	if base.Mode()&os.ModeSymlink == 0 {
		return nil, fmt.Errorf("not a symlink")
	}

	return &symLink{
		resource: base,
		target:   target,
	}, nil
}

func (l *symLink) Target() string {
	return l.target
}

type namedPipe struct {
	resource
}

var _ NamedPipe = &namedPipe{}

func newNamedPipe(base resource, paths []string) (NamedPipe, error) {
	if base.Mode()&os.ModeNamedPipe == 0 {
		return nil, fmt.Errorf("not a namedpipe")
	}

	base.paths = make([]string, len(paths))
	copy(base.paths, paths)

	return &namedPipe{
		resource: base,
	}, nil
}

func (np *namedPipe) Pipe() {}

func (np *namedPipe) Paths() []string {
	paths := make([]string, len(np.paths))
	copy(paths, np.paths)
	return paths
}

func (np *namedPipe) XAttrs() map[string][]byte {
	xattrs := make(map[string][]byte, len(np.xattrs))

	for attr, value := range np.xattrs {
		xattrs[attr] = append(xattrs[attr], value...)
	}

	return xattrs
}

type device struct {
	resource
	major, minor uint64
}

var _ Device = &device{}

func newDevice(base resource, paths []string, major, minor uint64) (Device, error) {
	if base.Mode()&os.ModeDevice == 0 {
		return nil, fmt.Errorf("not a device")
	}

	base.paths = make([]string, len(paths))
	copy(base.paths, paths)

	return &device{
		resource: base,
		major:    major,
		minor:    minor,
	}, nil
}

func (d *device) Paths() []string {
	paths := make([]string, len(d.paths))
	copy(paths, d.paths)
	return paths
}

func (d *device) XAttrs() map[string][]byte {
	xattrs := make(map[string][]byte, len(d.xattrs))

	for attr, value := range d.xattrs {
		xattrs[attr] = append(xattrs[attr], value...)
	}

	return xattrs
}

func (d device) Major() uint64 {
	return d.major
}

func (d device) Minor() uint64 {
	return d.minor
}

// toProto converts a resource to a protobuf record. We'd like to push this
// the individual types but we want to keep this all together during
// prototyping.
func toProto(resource Resource) *pb.Resource {
	b := &pb.Resource{
		Path: []string{resource.Path()},
		Mode: uint32(resource.Mode()),
		Uid:  resource.UID(),
		Gid:  resource.GID(),
	}

	if xattrer, ok := resource.(XAttrer); ok {
		// Sorts the XAttrs by name for consistent ordering.
		keys := []string{}
		xattrs := xattrer.XAttrs()
		for k := range xattrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			b.Xattr = append(b.Xattr, &pb.XAttr{Name: k, Data: xattrs[k]})
		}
	}

	switch r := resource.(type) {
	case RegularFile:
		b.Path = r.Paths()
		b.Size = uint64(r.Size())

		for _, dgst := range r.Digests() {
			b.Digest = append(b.Digest, dgst.String())
		}
	case SymLink:
		b.Target = r.Target()
	case Device:
		b.Major, b.Minor = r.Major(), r.Minor()
		b.Path = r.Paths()
	case NamedPipe:
		b.Path = r.Paths()
	}

	// enforce a few stability guarantees that may not be provided by the
	// resource implementation.
	sort.Strings(b.Path)

	return b
}

// fromProto converts from a protobuf Resource to a Resource interface.
func fromProto(b *pb.Resource) (Resource, error) {
	base := &resource{
		paths: b.Path,
		mode:  os.FileMode(b.Mode),
		uid:   b.Uid,
		gid:   b.Gid,
	}

	base.xattrs = make(map[string][]byte, len(b.Xattr))

	for _, attr := range b.Xattr {
		base.xattrs[attr.Name] = attr.Data
	}

	switch {
	case base.Mode().IsRegular():
		dgsts := make([]digest.Digest, len(b.Digest))
		for i, dgst := range b.Digest {
			// TODO(stevvooe): Should we be validating at this point?
			dgsts[i] = digest.Digest(dgst)
		}

		return newRegularFile(*base, b.Path, int64(b.Size), dgsts...)
	case base.Mode().IsDir():
		return newDirectory(*base)
	case base.Mode()&os.ModeSymlink != 0:
		return newSymLink(*base, b.Target)
	case base.Mode()&os.ModeNamedPipe != 0:
		return newNamedPipe(*base, b.Path)
	case base.Mode()&os.ModeDevice != 0:
		return newDevice(*base, b.Path, b.Major, b.Minor)
	}

	return nil, fmt.Errorf("unknown resource record (%#v): %s", b, base.Mode())
}

// NOTE(stevvooe): An alternative model that supports inline declaration.
// Convenient for unit testing where inline declarations may be desirable but
// creates an awkward API for the standard use case.

// type ResourceKind int

// const (
// 	ResourceRegularFile = iota + 1
// 	ResourceDirectory
// 	ResourceSymLink
// 	Resource
// )

// type Resource struct {
// 	Kind         ResourceKind
// 	Paths        []string
// 	Mode         os.FileMode
// 	UID          string
// 	GID          string
// 	Size         int64
// 	Digests      []digest.Digest
// 	Target       string
// 	Major, Minor int
// 	XAttrs       map[string][]byte
// }

// type RegularFile struct {
// 	Paths   []string
//  Size 	int64
// 	Digests []digest.Digest
// 	Perm    os.FileMode // os.ModePerm + sticky, setuid, setgid
// }
