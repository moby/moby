// +build !linux,!windows

package hvsock

import (
	"errors"
	"time"
)

const (
	sysAF_HYPERV     = 42
	sysSHV_PROTO_RAW = 1
)

type hvsockListener struct {
	acceptFD int
	laddr    HypervAddr
}

//
// System call wrapper
//
func connect(s int, a *HypervAddr) (err error) {
	return errors.New("connect() not implemented")
}

func bind(s int, a HypervAddr) error {
	return errors.New("bind() not implemented")
}

func accept(s int, a *HypervAddr) (int, error) {
	return 0, errors.New("accept() not implemented")
}

type hvsockConn struct {
	fd     int
	local  HypervAddr
	remote HypervAddr
}

func newHVsockConn(fd int, local HypervAddr, remote HypervAddr) (*HVsockConn, error) {
	v := &hvsockConn{local: local, remote: remote}
	return &HVsockConn{hvsockConn: *v}, errors.New("newHVsockConn() not implemented")
}

func (v *HVsockConn) close() error {
	return errors.New("close() not implemented")
}

func (v *HVsockConn) read(buf []byte) (int, error) {
	return 0, errors.New("read() not implemented")
}

func (v *HVsockConn) write(buf []byte) (int, error) {
	return 0, errors.New("write() not implemented")
}

// SetReadDeadline dummy doc to silence lint
func (v *HVsockConn) SetReadDeadline(t time.Time) error {
	return nil // FIXME
}

// SetWriteDeadline dummy doc to silence lint
func (v *HVsockConn) SetWriteDeadline(t time.Time) error {
	return nil // FIXME
}

// SetDeadline dummy doc to silence lint
func (v *HVsockConn) SetDeadline(t time.Time) error {
	return nil // FIXME
}
