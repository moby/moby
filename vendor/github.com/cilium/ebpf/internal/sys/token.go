package sys

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/mountinfo"
	"github.com/cilium/ebpf/internal/platform"
	"github.com/cilium/ebpf/internal/testutils/testmain"
	"github.com/cilium/ebpf/internal/unix"
)

var (
	// ErrTokenCapabilities indicates that the operation failed due to
	// insufficient capabilities being delegated to the token.
	ErrTokenCapabilities = errors.New("token insufficiently privileged")
)

// Token is a BPF token that can be used to create BPF objects that normally
// require CAP_SYS_ADMIN or CAP_NET_ADMIN.
//
// With a valid token, equivalent BPF objects can be created while only being
// granted CAP_BPF.
type Token struct {
	fd *FD

	// Token info is available as of Linux 6.17, may be nil.
	info *TokenInfo
}

// FD returns the raw file descriptor of the token.
func (t *Token) FD() int {
	return t.fd.Int()
}

// Capable returns an error if the token does not allow using cmd or some of its
// provided attributes.
//
// Always returns nil on kernels before 6.17, as they don't provide token info.
func (t *Token) Capable(cmd Cmd, attr unsafe.Pointer) error {
	if t.info == nil {
		return nil
	}

	if !t.cmd(cmd) {
		return fmt.Errorf("command %s: %w", cmd, ErrTokenCapabilities)
	}

	switch cmd {
	case BPF_MAP_CREATE:
		attr := (*MapCreateAttr)(attr)
		if !t.mapType(attr.MapType) {
			return fmt.Errorf("map type %s: %w", attr.MapType, ErrTokenCapabilities)
		}

	case BPF_PROG_LOAD:
		attr := (*ProgLoadAttr)(attr)
		if !t.progType(attr.ProgType) {
			return fmt.Errorf("program type %s: %w", attr.ProgType, ErrTokenCapabilities)
		}

		if !t.attachType(attr.ExpectedAttachType) {
			return fmt.Errorf("attach type %s: %w", attr.ExpectedAttachType, ErrTokenCapabilities)
		}
	}

	return nil
}

// cmd returns true if cmd is allowed by the Token.
func (t *Token) cmd(cmd Cmd) bool {
	if t.info == nil {
		return true
	}
	return t.info.AllowedCmds&(uint64(1)<<cmd) != 0
}

// mapType returns true if the token allows creation of the given MapType.
func (t *Token) mapType(mt MapType) bool {
	if t.info == nil {
		return true
	}
	return t.info.AllowedMaps&(uint64(1)<<mt) != 0
}

// progType returns true if the token allows creation of the given ProgType.
func (t *Token) progType(pt ProgType) bool {
	if t.info == nil {
		return true
	}
	return t.info.AllowedProgs&(uint64(1)<<pt) != 0
}

// attachType returns true if the token allows using the given AttachType.
func (t *Token) attachType(at AttachType) bool {
	if t.info == nil {
		return true
	}
	return t.info.AllowedAttachs&(uint64(1)<<at) != 0
}

var (
	tokenOnce sync.Once
	token     *Token
	tokenErr  error
)

// getToken attempts to create a BPF token by enumerating all bpffs mounts and
// issuing a token from each of them until it yields a valid one. Its result is
// cached for the lifetime of the process.
//
// Returns a nil token if one could not be created. If the OS is not Linux,
// additionally returns an error wrapping [internal.ErrNotSupportedOnOS].
func getToken() (*Token, error) {
	tokenOnce.Do(func() {
		token, tokenErr = findToken()

		// Forget the fd, we are intentionally leaking it for caching purposes.
		if token != nil {
			testmain.ForgetFD(token.FD())
		}
	})

	return token, tokenErr
}

func findToken() (*Token, error) {
	if !platform.IsLinux {
		return nil, fmt.Errorf("bpf token: %w", internal.ErrNotSupportedOnOS)
	}

	mounts, err := mountinfo.FindByFSType("bpf")
	if err != nil {
		return nil, fmt.Errorf("get bpffs mounts: %w", err)
	}

	for _, mount := range mounts {
		tok, err := newToken(mount)
		if errors.Is(err, unix.EACCES) || // lacking privileges to open mount dir
			errors.Is(err, unix.EINVAL) || // tokens not supported or mount not a bpffs
			errors.Is(err, unix.EPERM) || // CAP_BPF missing or mount not owned by current user namespace
			errors.Is(err, unix.EOPNOTSUPP) || // cannot use token in init user namespace
			errors.Is(err, unix.ENOENT) { // no permissions delegated to this bpffs
			continue
		}
		if err != nil {
			// Fail on any unexpected errors, since the list of mounts is already
			// filtered down to bpffs only, and we don't expect any transient errors
			// from the kernel. If errors occur like EACCES due to ACLs or EMFILE due
			// to fd limits, it's better to flag this to the user to avoid surprises.
			return nil, fmt.Errorf("create token from %q: %w", mount, err)
		}
		if tok != nil {
			return tok, nil
		}
	}

	return nil, nil
}

func newToken(mount string) (*Token, error) {
	fsfd, err := unix.Open(mount, unix.O_DIRECTORY|unix.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open bpffs mount %q: %w", mount, err)
	}
	defer unix.Close(fsfd)

	tfd, err := TokenCreate(&TokenCreateAttr{BpffsFd: uint32(fsfd)})
	if err != nil {
		return nil, err
	}

	t := &Token{
		fd: tfd,
	}

	var ti TokenInfo
	if err := ObjInfo(tfd, &ti); err == nil {
		t.info = &ti
	} else if !errors.Is(err, unix.EINVAL) {
		_ = tfd.Close()
		return nil, fmt.Errorf("get token info: %w", err)
	}

	return t, nil
}

// tokenAttr sets the appropriate token fields in the BPF syscall attribute
// struct for the given command, if a token is available.
func tokenAttr(cmd Cmd, attr unsafe.Pointer) (*Token, error) {
	switch cmd {
	case BPF_MAP_CREATE, BPF_PROG_LOAD, BPF_BTF_LOAD:
	default:
		return nil, nil
	}

	tok, err := getToken()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	if tok == nil {
		return nil, nil
	}

	switch cmd {
	case BPF_MAP_CREATE:
		attr := (*MapCreateAttr)(attr)
		attr.MapTokenFd = int32(tok.FD())
		attr.MapFlags |= BPF_F_TOKEN_FD

	case BPF_PROG_LOAD:
		attr := (*ProgLoadAttr)(attr)
		attr.ProgTokenFd = int32(tok.FD())
		attr.ProgFlags |= BPF_F_TOKEN_FD

	case BPF_BTF_LOAD:
		attr := (*BtfLoadAttr)(attr)
		attr.BtfTokenFd = int32(tok.FD())
		attr.BtfFlags |= BPF_F_TOKEN_FD
	}

	return tok, nil
}
