// +build !linux

package netns

import (
	"errors"
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

func Setns(ns NsHandle, nstype int) (err error) {
	return ErrNotImplemented
}

func Set(ns NsHandle) (err error) {
	return ErrNotImplemented
}

func New() (ns NsHandle, err error) {
	return -1, ErrNotImplemented
}

func NewNamed(name string) (NsHandle, error) {
	return -1, ErrNotImplemented
}

func DeleteNamed(name string) error {
	return ErrNotImplemented
}

func Get() (NsHandle, error) {
	return -1, ErrNotImplemented
}

func GetFromPath(path string) (NsHandle, error) {
	return -1, ErrNotImplemented
}

func GetFromName(name string) (NsHandle, error) {
	return -1, ErrNotImplemented
}

func GetFromPid(pid int) (NsHandle, error) {
	return -1, ErrNotImplemented
}

func GetFromThread(pid, tid int) (NsHandle, error) {
	return -1, ErrNotImplemented
}

func GetFromDocker(id string) (NsHandle, error) {
	return -1, ErrNotImplemented
}
