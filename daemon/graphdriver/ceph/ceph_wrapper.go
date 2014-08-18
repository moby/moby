// +build linux

package ceph

/*
#cgo LDFLAGS: -L. -lrados -lrbd
#include <errno.h>
#include <rados/librados.h>
#include <rbd/librbd.h>
*/
import "C"

import (
	"errors"
	"fmt"
)

type (
	Rados      C.rados_t
	RadosIoCtx C.rados_ioctx_t
	RbdImage   C.rbd_image_t
)

var (
	RbdNotFoundError  = errors.New("No such image or snapshot")
	RbdBusyError      = errors.New("Image or snapshot busy")
	RbdIvalidArgError = errors.New("Invalid argument")
)

func radosCreate(client string) (Rados, error) {
	rados := Rados(nil)

	if rv := int(C.rados_create((*C.rados_t)(&rados), C.CString(client))); rv < 0 {
		return nil, errors.New("Cannot initialize Rados")
	}

	return rados, nil
}

func radosConfReadFile(rados Rados, path string) error {
	if rv := int(C.rados_conf_read_file((C.rados_t)(rados), C.CString(path))); rv < 0 {
		return fmt.Errorf("Cannot read Rados configuration file: %s", path)
	}

	return nil
}

func radosConnect(rados Rados) error {
	if rv := int(C.rados_connect((C.rados_t)(rados))); rv < 0 {
		return errors.New("Cannot connect to Rados")
	}

	return nil
}

func radosDisconnect(rados Rados) {
	C.rados_shutdown((C.rados_t)(rados))
}

func radosIoCtxCreate(rados Rados, pool string) (RadosIoCtx, error) {
	ioctx := RadosIoCtx(nil)

	if rv := int(C.rados_ioctx_create((C.rados_t)(rados), C.CString(pool),
		(*C.rados_ioctx_t)(&ioctx))); rv < 0 {
		return nil, fmt.Errorf("Cannot create Rados IO Context for pool: %s", pool)
	}

	return ioctx, nil
}

func radosIoCtxDestroy(ioctx RadosIoCtx) {
	C.rados_ioctx_destroy((C.rados_ioctx_t)(ioctx))
}

func rbdCreate(ioctx RadosIoCtx, name string, size int) error {
	order := C.int(0)

	if rv := int(C.rbd_create2((C.rados_ioctx_t)(ioctx), C.CString(name),
		C.uint64_t(size), C.RBD_FEATURE_LAYERING, &order)); rv < 0 {
		return fmt.Errorf("Cannot create RBD image: %s (size %i)", name, size)
	}

	return nil
}

func rbdRemove(ioctx RadosIoCtx, name string) error {
	if rv := int(C.rbd_remove((C.rados_ioctx_t)(ioctx), C.CString(name))); rv < 0 {
		if rv == -C.ENOENT {
			return RbdNotFoundError
		}

		return fmt.Errorf("Cannot remove RBD image: %s", name)
	}

	return nil
}

func rbdOpen(ioctx RadosIoCtx, name string) (RbdImage, error) {
	image := RbdImage(nil)

	if rv := int(C.rbd_open((C.rados_ioctx_t)(ioctx), C.CString(name),
		(*C.rbd_image_t)(&image), nil)); rv < 0 {
		if rv == -C.ENOENT {
			return nil, RbdNotFoundError
		}

		return nil, fmt.Errorf("Cannot open RBD image: %s", name)
	}

	return image, nil
}

func rbdOpenSnapshot(ioctx RadosIoCtx, name,
	snapshot string) (RbdImage, error) {
	image := RbdImage(nil)

	if rv := int(C.rbd_open((C.rados_ioctx_t)(ioctx), C.CString(name),
		(*C.rbd_image_t)(&image), C.CString(snapshot))); rv < 0 {
		if rv == -C.ENOENT {
			return nil, RbdNotFoundError
		}

		return nil, errors.New("Cannot open RBD image snapshot")
	}

	return image, nil
}

func rbdClose(image RbdImage) {
	C.rbd_close((C.rbd_image_t)(image))
}

func rbdSnapshotCreate(image RbdImage, name string) error {
	if rv := int(C.rbd_snap_create((C.rbd_image_t)(image), C.CString(name))); rv < 0 {
		return errors.New("Cannot create RBD image snapshot")
	}

	return nil
}

func rbdSnapshotProtect(image RbdImage, name string) error {
	if rv := int(C.rbd_snap_protect((C.rbd_image_t)(image), C.CString(name))); rv < 0 {
		if rv == -C.EBUSY {
			return RbdBusyError
		}

		return errors.New("Cannot protect RBD image snapshot")
	}

	return nil
}

func rbdClone(ioctx RadosIoCtx, parent, snapshot, name string) error {
	order := C.int(0)

	if rv := int(C.rbd_clone((C.rados_ioctx_t)(ioctx), C.CString(parent),
		C.CString(snapshot), (C.rados_ioctx_t)(ioctx), C.CString(name),
		C.RBD_FEATURE_LAYERING, &order)); rv < 0 {
		return errors.New("Cannot clone RBD image snapshot")
	}

	return nil
}

func rbdSnapshotUnprotect(image RbdImage, name string) error {
	if rv := int(C.rbd_snap_unprotect((C.rbd_image_t)(image), C.CString(name))); rv < 0 {
		if rv == -C.EINVAL {
			return RbdIvalidArgError
		}

		return errors.New("Cannot unprotect RBD image snapshot")
	}

	return nil
}

func rbdSnapshotRemove(image RbdImage, name string) error {
	if rv := int(C.rbd_snap_remove((C.rbd_image_t)(image), C.CString(name))); rv < 0 {
		if rv == -C.ENOENT {
			return RbdNotFoundError
		}

		return errors.New("Cannot remove RBD image snapshot")
	}

	return nil
}
