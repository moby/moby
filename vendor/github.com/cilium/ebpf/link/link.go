package link

import (
	"errors"
	"fmt"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// Type is the kind of link.
type Type = sys.LinkType

var ErrNotSupported = internal.ErrNotSupported

// Link represents a Program attached to a BPF hook.
type Link interface {
	// Replace the current program with a new program.
	//
	// Passing a nil program is an error. May return an error wrapping ErrNotSupported.
	Update(*ebpf.Program) error

	// Persist a link by pinning it into a bpffs.
	//
	// May return an error wrapping ErrNotSupported.
	Pin(string) error

	// Undo a previous call to Pin.
	//
	// May return an error wrapping ErrNotSupported.
	Unpin() error

	// Close frees resources.
	//
	// The link will be broken unless it has been successfully pinned.
	// A link may continue past the lifetime of the process if Close is
	// not called.
	Close() error

	// Detach the link from its corresponding attachment point.
	//
	// May return an error wrapping ErrNotSupported.
	Detach() error

	// Info returns metadata on a link.
	//
	// May return an error wrapping ErrNotSupported.
	Info() (*Info, error)

	// Prevent external users from implementing this interface.
	isLink()
}

// NewFromFD creates a link from a raw fd.
//
// You should not use fd after calling this function.
func NewFromFD(fd int) (Link, error) {
	sysFD, err := sys.NewFD(fd)
	if err != nil {
		return nil, err
	}

	return wrapRawLink(&RawLink{fd: sysFD})
}

// NewFromID returns the link associated with the given id.
//
// Returns ErrNotExist if there is no link with the given id.
func NewFromID(id ID) (Link, error) {
	getFdAttr := &sys.LinkGetFdByIdAttr{Id: id}
	fd, err := sys.LinkGetFdById(getFdAttr)
	if err != nil {
		return nil, fmt.Errorf("get link fd from ID %d: %w", id, err)
	}

	return wrapRawLink(&RawLink{fd, ""})
}

// LoadPinnedLink loads a Link from a pin (file) on the BPF virtual filesystem.
//
// Requires at least Linux 5.7.
func LoadPinnedLink(fileName string, opts *ebpf.LoadPinOptions) (Link, error) {
	raw, err := loadPinnedRawLink(fileName, opts)
	if err != nil {
		return nil, err
	}

	return wrapRawLink(raw)
}

// ID uniquely identifies a BPF link.
type ID = sys.LinkID

// RawLinkOptions control the creation of a raw link.
type RawLinkOptions struct {
	// File descriptor to attach to. This differs for each attach type.
	Target int
	// Program to attach.
	Program *ebpf.Program
	// Attach must match the attach type of Program.
	Attach ebpf.AttachType
	// BTF is the BTF of the attachment target.
	BTF btf.TypeID
	// Flags control the attach behaviour.
	Flags uint32
}

// Info contains metadata on a link.
type Info struct {
	Type    Type
	ID      ID
	Program ebpf.ProgramID
	extra   any
}

// RawLink is the low-level API to bpf_link.
//
// You should consider using the higher level interfaces in this
// package instead.
type RawLink struct {
	fd         *sys.FD
	pinnedPath string
}

func loadPinnedRawLink(fileName string, opts *ebpf.LoadPinOptions) (*RawLink, error) {
	fd, typ, err := sys.ObjGetTyped(&sys.ObjGetAttr{
		Pathname:  sys.NewStringPointer(fileName),
		FileFlags: opts.Marshal(),
	})
	if err != nil {
		return nil, fmt.Errorf("load pinned link: %w", err)
	}

	if typ != sys.BPF_TYPE_LINK {
		_ = fd.Close()
		return nil, fmt.Errorf("%s is not a Link", fileName)
	}

	return &RawLink{fd, fileName}, nil
}

func (l *RawLink) isLink() {}

// FD returns the raw file descriptor.
func (l *RawLink) FD() int {
	return l.fd.Int()
}

// Close breaks the link.
//
// Use Pin if you want to make the link persistent.
func (l *RawLink) Close() error {
	return l.fd.Close()
}

// Pin persists a link past the lifetime of the process.
//
// Calling Close on a pinned Link will not break the link
// until the pin is removed.
func (l *RawLink) Pin(fileName string) error {
	if err := sys.Pin(l.pinnedPath, fileName, l.fd); err != nil {
		return err
	}
	l.pinnedPath = fileName
	return nil
}

// Unpin implements the Link interface.
func (l *RawLink) Unpin() error {
	if err := sys.Unpin(l.pinnedPath); err != nil {
		return err
	}
	l.pinnedPath = ""
	return nil
}

// IsPinned returns true if the Link has a non-empty pinned path.
func (l *RawLink) IsPinned() bool {
	return l.pinnedPath != ""
}

// Update implements the Link interface.
func (l *RawLink) Update(new *ebpf.Program) error {
	return l.UpdateArgs(RawLinkUpdateOptions{
		New: new,
	})
}

// RawLinkUpdateOptions control the behaviour of RawLink.UpdateArgs.
type RawLinkUpdateOptions struct {
	New   *ebpf.Program
	Old   *ebpf.Program
	Flags uint32
}

// UpdateArgs updates a link based on args.
func (l *RawLink) UpdateArgs(opts RawLinkUpdateOptions) error {
	newFd := opts.New.FD()
	if newFd < 0 {
		return fmt.Errorf("invalid program: %s", sys.ErrClosedFd)
	}

	var oldFd int
	if opts.Old != nil {
		oldFd = opts.Old.FD()
		if oldFd < 0 {
			return fmt.Errorf("invalid replacement program: %s", sys.ErrClosedFd)
		}
	}

	attr := sys.LinkUpdateAttr{
		LinkFd:    l.fd.Uint(),
		NewProgFd: uint32(newFd),
		OldProgFd: uint32(oldFd),
		Flags:     opts.Flags,
	}
	if err := sys.LinkUpdate(&attr); err != nil {
		return fmt.Errorf("update link: %w", err)
	}
	return nil
}

// Detach the link from its corresponding attachment point.
func (l *RawLink) Detach() error {
	attr := sys.LinkDetachAttr{
		LinkFd: l.fd.Uint(),
	}

	err := sys.LinkDetach(&attr)

	switch {
	case errors.Is(err, unix.EOPNOTSUPP):
		return internal.ErrNotSupported
	case err != nil:
		return fmt.Errorf("detach link: %w", err)
	default:
		return nil
	}
}

// Info returns metadata about the link.
//
// Linktype specific metadata is not included and can be retrieved
// via the linktype specific Info() method.
func (l *RawLink) Info() (*Info, error) {
	var info sys.LinkInfo

	if err := sys.ObjInfo(l.fd, &info); err != nil {
		return nil, fmt.Errorf("link info: %s", err)
	}

	return &Info{
		info.Type,
		info.Id,
		ebpf.ProgramID(info.ProgId),
		nil,
	}, nil
}

// Iterator allows iterating over links attached into the kernel.
type Iterator struct {
	// The ID of the current link. Only valid after a call to Next
	ID ID
	// The current link. Only valid until a call to Next.
	// See Take if you want to retain the link.
	Link Link
	err  error
}

// Next retrieves the next link.
//
// Returns true if another link was found. Call [Iterator.Err] after the function returns false.
func (it *Iterator) Next() bool {
	id := it.ID
	for {
		getIdAttr := &sys.LinkGetNextIdAttr{Id: id}
		err := sys.LinkGetNextId(getIdAttr)
		if errors.Is(err, os.ErrNotExist) {
			// There are no more links.
			break
		} else if err != nil {
			it.err = fmt.Errorf("get next link ID: %w", err)
			break
		}

		id = getIdAttr.NextId
		l, err := NewFromID(id)
		if errors.Is(err, os.ErrNotExist) {
			// Couldn't load the link fast enough. Try next ID.
			continue
		} else if err != nil {
			it.err = fmt.Errorf("get link for ID %d: %w", id, err)
			break
		}

		if it.Link != nil {
			it.Link.Close()
		}
		it.ID, it.Link = id, l
		return true
	}

	// No more links or we encountered an error.
	if it.Link != nil {
		it.Link.Close()
	}
	it.Link = nil
	return false
}

// Take the ownership of the current link.
//
// It's the callers responsibility to close the link.
func (it *Iterator) Take() Link {
	l := it.Link
	it.Link = nil
	return l
}

// Err returns an error if iteration failed for some reason.
func (it *Iterator) Err() error {
	return it.err
}

func (it *Iterator) Close() {
	if it.Link != nil {
		it.Link.Close()
	}
}
