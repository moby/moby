package hvsock

// On Linux we have to deal with two different implementations. The
// "legacy" implementation never made it into the kernel, but several
// kernels, including the LinuxKit one carried patches for it for
// quite a while. The legacy version defined a new address family
// while the new version sits on top of the existing VMware/virtio
// socket implementation.
//
// We try to determine at init if we are on a kernel with the legacy
// implementation or the new version and set "legacyMode" accordingly.
//
// We can't just reuse the vsock implementation as we still need to
// emulated CloseRead()/CloseWrite() as not all Windows builds support
// it.

/*
#include <sys/socket.h>

struct sockaddr_hv {
	unsigned short shv_family;
	unsigned short reserved;
	unsigned char  shv_vm_id[16];
	unsigned char  shv_service_id[16];
};
int bind_sockaddr_hv(int fd, const struct sockaddr_hv *sa_hv) {
    return bind(fd, (const struct sockaddr*)sa_hv, sizeof(*sa_hv));
}
int connect_sockaddr_hv(int fd, const struct sockaddr_hv *sa_hv) {
    return connect(fd, (const struct sockaddr*)sa_hv, sizeof(*sa_hv));
}
int accept_hv(int fd, struct sockaddr_hv *sa_hv, socklen_t *sa_hv_len) {
    return accept4(fd, (struct sockaddr *)sa_hv, sa_hv_len, 0);
}

struct sockaddr_vsock {
	sa_family_t svm_family;
	unsigned short svm_reserved1;
	unsigned int svm_port;
	unsigned int svm_cid;
	unsigned char svm_zero[sizeof(struct sockaddr) -
		sizeof(sa_family_t) - sizeof(unsigned short) -
		sizeof(unsigned int) - sizeof(unsigned int)];
};
int bind_sockaddr_vsock(int fd, const struct sockaddr_vsock *sa_vsock) {
    return bind(fd, (const struct sockaddr*)sa_vsock, sizeof(*sa_vsock));
}
int connect_sockaddr_vsock(int fd, const struct sockaddr_vsock *sa_vsock) {
    return connect(fd, (const struct sockaddr*)sa_vsock, sizeof(*sa_vsock));
}
int accept_vsock(int fd, struct sockaddr_vsock *sa_vsock, socklen_t *sa_vsock_len) {
    return accept4(fd, (struct sockaddr *)sa_vsock, sa_vsock_len, 0);
}
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/linuxkit/virtsock/pkg/vsock"
)

var (
	legacyMode bool
)

const (
	sysAF_HYPERV     = 43
	sysAF_VSOCK      = 40
	sysSHV_PROTO_RAW = 1

	guidTmpl = "00000000-facb-11e6-bd58-64006a7986d3"
)

type hvsockListener struct {
	acceptFD int
	laddr    HypervAddr
}

// Try to determine if we need to run in legacy mode.
// 4.11 defines AF_SMC as 43 but it doesn't support protocol 1 so the
// socket() call should fail.
func init() {
	fd, err := syscall.Socket(sysAF_HYPERV, syscall.SOCK_STREAM, sysSHV_PROTO_RAW)
	if err != nil {
		legacyMode = false
	} else {
		legacyMode = true
		syscall.Close(fd)
	}
}

//
// System call wrapper
//
func hvsocket(typ, proto int) (int, error) {
	if legacyMode {
		return syscall.Socket(sysAF_HYPERV, typ, proto)
	}
	return syscall.Socket(sysAF_VSOCK, typ, 0)
}

func connect(s int, a *HypervAddr) (err error) {
	if legacyMode {
		sa := C.struct_sockaddr_hv{}
		sa.shv_family = sysAF_HYPERV
		sa.reserved = 0

		for i := 0; i < 16; i++ {
			sa.shv_vm_id[i] = C.uchar(a.VMID[i])
		}
		for i := 0; i < 16; i++ {
			sa.shv_service_id[i] = C.uchar(a.ServiceID[i])
		}

		if ret, errno := C.connect_sockaddr_hv(C.int(s), &sa); ret != 0 {
			return errors.New(fmt.Sprintf(
				"connect(%s:%s) returned %d, errno %d: %s",
				a.VMID, a.ServiceID, ret, errno, errno))
		}
		return nil
	}

	sa := C.struct_sockaddr_vsock{}
	sa.svm_family = sysAF_VSOCK
	sa.svm_port = C.uint(binary.LittleEndian.Uint32(a.ServiceID[0:4]))
	// Ignore what's passed in. Use CIDAny as this is an accepted value
	sa.svm_cid = C.uint(vsock.CIDAny)

	if ret, errno := C.connect_sockaddr_vsock(C.int(s), &sa); ret != 0 {
		return errors.New(fmt.Sprintf(
			"connect(%08x.%08x) returned %d, errno %d: %s",
			sa.svm_cid, sa.svm_port, ret, errno, errno))
	}
	return nil
}

func bind(s int, a HypervAddr) error {
	if legacyMode {
		sa := C.struct_sockaddr_hv{}
		sa.shv_family = sysAF_HYPERV
		sa.reserved = 0

		for i := 0; i < 16; i++ {
			sa.shv_vm_id[i] = C.uchar(GUIDZero[i])
		}
		for i := 0; i < 16; i++ {
			sa.shv_service_id[i] = C.uchar(a.ServiceID[i])
		}

		if ret, errno := C.bind_sockaddr_hv(C.int(s), &sa); ret != 0 {
			return errors.New(fmt.Sprintf(
				"bind(%s:%s) returned %d, errno %d: %s",
				GUIDZero, a.ServiceID, ret, errno, errno))
		}
		return nil
	}

	sa := C.struct_sockaddr_vsock{}
	sa.svm_family = sysAF_VSOCK
	sa.svm_port = C.uint(binary.LittleEndian.Uint32(a.ServiceID[0:4]))
	// Ignore what's passed in. Use CIDAny as this is the only accepted value
	sa.svm_cid = C.uint(vsock.CIDAny)

	if ret, errno := C.bind_sockaddr_vsock(C.int(s), &sa); ret != 0 {
		return errors.New(fmt.Sprintf(
			"connect(%08x.%08x) returned %d, errno %d: %s",
			sa.svm_cid, sa.svm_port, ret, errno, errno))
	}
	return nil
}

func accept(s int, a *HypervAddr) (int, error) {
	if legacyMode {
		var acceptSA C.struct_sockaddr_hv
		var acceptSALen C.socklen_t

		acceptSALen = C.sizeof_struct_sockaddr_hv
		fd, err := C.accept_hv(C.int(s), &acceptSA, &acceptSALen)
		if err != nil {
			return -1, err
		}

		a.VMID = guidFromC(acceptSA.shv_vm_id)
		a.ServiceID = guidFromC(acceptSA.shv_service_id)

		return int(fd), nil
	}

	var acceptSA C.struct_sockaddr_vsock
	var acceptSALen C.socklen_t

	acceptSALen = C.sizeof_struct_sockaddr_vsock
	fd, err := C.accept_vsock(C.int(s), &acceptSA, &acceptSALen)
	if err != nil {
		return -1, err
	}

	a.VMID = GUIDParent
	a.ServiceID, _ = GUIDFromString(guidTmpl)
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, uint32(acceptSA.svm_port))
	a.ServiceID[0] = tmp[0]
	a.ServiceID[1] = tmp[1]
	a.ServiceID[2] = tmp[2]
	a.ServiceID[3] = tmp[3]
	return int(fd), nil
}

// Internal representation. Complex mostly due to asynch send()/recv() syscalls.
type hvsockConn struct {
	fd     int
	hvsock *os.File
	local  HypervAddr
	remote HypervAddr
}

// Main constructor
func newHVsockConn(fd int, local HypervAddr, remote HypervAddr) (*HVsockConn, error) {
	hvsock := os.NewFile(uintptr(fd), fmt.Sprintf("hvsock:%d", fd))
	v := &hvsockConn{fd: fd, hvsock: hvsock, local: local, remote: remote}

	return &HVsockConn{hvsockConn: *v}, nil
}

func (v *HVsockConn) close() error {
	return v.hvsock.Close()
}

func (v *HVsockConn) read(buf []byte) (int, error) {
	return v.hvsock.Read(buf)
}

func (v *HVsockConn) write(buf []byte) (int, error) {
	return v.hvsock.Write(buf)
}

// SetReadDeadline is un-implemented
func (v *HVsockConn) SetReadDeadline(t time.Time) error {
	return nil // FIXME
}

// SetWriteDeadline is un-implemented
func (v *HVsockConn) SetWriteDeadline(t time.Time) error {
	return nil // FIXME
}

// SetDeadline is un-implemented
func (v *HVsockConn) SetDeadline(t time.Time) error {
	return nil // FIXME
}

func guidFromC(cg [16]C.uchar) GUID {
	var g GUID
	for i := 0; i < 16; i++ {
		g[i] = byte(cg[i])
	}
	return g
}
