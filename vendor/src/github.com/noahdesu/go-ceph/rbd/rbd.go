package rbd

// #cgo LDFLAGS: -lrbd
// #include <errno.h>
// #include <stdlib.h>
// #include <rados/librados.h>
// #include <rbd/librbd.h>
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/noahdesu/go-ceph/rados"
	"io"
	"unsafe"
)

//
type RBDError int

var RbdErrorImageNotOpen = errors.New("RBD image not open")
var RbdErrorNotFound = errors.New("RBD image not found")

//Rdb feature
var RbdFeatureLayering = uint64(1 << 0)
var RbdFeatureStripingV2 = uint64(1 << 1)

//
type ImageInfo struct {
	Size              uint64
	Obj_size          uint64
	Num_objs          uint64
	Order             int
	Block_name_prefix string
	Parent_pool       int64
	Parent_name       string
}

//
type SnapInfo struct {
	Id   uint64
	Size uint64
	Name string
}

//
type Locker struct {
	Client string
	Cookie string
	Addr   string
}

//
type Image struct {
	io.Reader
	io.Writer
	io.Seeker
	io.ReaderAt
	io.WriterAt
	name   string
	offset int64
	ioctx  *rados.IOContext
	image  C.rbd_image_t
}

//
type Snapshot struct {
	image *Image
	name  string
}

//
func split(buf []byte) (values []string) {
	tmp := bytes.Split(buf[:len(buf)-1], []byte{0})
	for _, s := range tmp {
		if len(s) > 0 {
			go_s := C.GoString((*C.char)(unsafe.Pointer(&s[0])))
			values = append(values, go_s)
		}
	}
	return values
}

//
func (e RBDError) Error() string {
	return fmt.Sprintf("rbd: ret=%d", e)
}

//
func GetError(err C.int) error {
	if err != 0 {
		return RBDError(err)
	} else {
		return nil
	}
}

//
func Version() (int, int, int) {
	var c_major, c_minor, c_patch C.int
	C.rbd_version(&c_major, &c_minor, &c_patch)
	return int(c_major), int(c_minor), int(c_patch)
}

// GetImageNames returns the list of current RBD images.
func GetImageNames(ioctx *rados.IOContext) (names []string, err error) {
	buf := make([]byte, 4096)
	for {
		size := C.size_t(len(buf))
		ret := C.rbd_list(C.rados_ioctx_t(ioctx.Pointer()),
			(*C.char)(unsafe.Pointer(&buf[0])), &size)
		if ret == -34 { // FIXME
			buf = make([]byte, size)
			continue
		} else if ret < 0 {
			return nil, RBDError(ret)
		}
		tmp := bytes.Split(buf[:size-1], []byte{0})
		for _, s := range tmp {
			if len(s) > 0 {
				name := C.GoString((*C.char)(unsafe.Pointer(&s[0])))
				names = append(names, name)
			}
		}
		return names, nil
	}
}

//
func GetImage(ioctx *rados.IOContext, name string) *Image {
	return &Image{
		ioctx: ioctx,
		name:  name,
	}
}

// int rbd_create(rados_ioctx_t io, const char *name, uint64_t size, int *order);
// int rbd_create2(rados_ioctx_t io, const char *name, uint64_t size,
//          uint64_t features, int *order);
// int rbd_create3(rados_ioctx_t io, const char *name, uint64_t size,
//        uint64_t features, int *order,
//        uint64_t stripe_unit, uint64_t stripe_count);
func Create(ioctx *rados.IOContext, name string, size uint64,
	args ...uint64) (image *Image, err error) {
	var ret C.int
	var c_order C.int
	var c_name *C.char = C.CString(name)
	defer C.free(unsafe.Pointer(c_name))

	switch len(args) {
	case 2:
		ret = C.rbd_create3(C.rados_ioctx_t(ioctx.Pointer()),
			c_name, C.uint64_t(size),
			C.uint64_t(args[0]), &c_order,
			C.uint64_t(args[1]), C.uint64_t(args[2]))
	case 1:
		ret = C.rbd_create2(C.rados_ioctx_t(ioctx.Pointer()),
			c_name, C.uint64_t(size),
			C.uint64_t(args[0]), &c_order)
	case 0:
		ret = C.rbd_create(C.rados_ioctx_t(ioctx.Pointer()),
			c_name, C.uint64_t(size), &c_order)
	default:
		return nil, errors.New("Wrong number of argument")
	}

	if ret < 0 {
		return nil, RBDError(int(ret))
	}

	return &Image{
		ioctx: ioctx,
		name:  name,
	}, nil
}

// int rbd_clone(rados_ioctx_t p_ioctx, const char *p_name,
//           const char *p_snapname, rados_ioctx_t c_ioctx,
//           const char *c_name, uint64_t features, int *c_order);
// int rbd_clone2(rados_ioctx_t p_ioctx, const char *p_name,
//            const char *p_snapname, rados_ioctx_t c_ioctx,
//            const char *c_name, uint64_t features, int *c_order,
//            uint64_t stripe_unit, int stripe_count);
func (image *Image) Clone(snapname string, c_ioctx *rados.IOContext, c_name string, features uint64) (*Image, error) {
	var c_order C.int
	var c_p_name *C.char = C.CString(image.name)
	var c_p_snapname *C.char = C.CString(snapname)
	var c_c_name *C.char = C.CString(c_name)
	defer C.free(unsafe.Pointer(c_p_name))
	defer C.free(unsafe.Pointer(c_p_snapname))
	defer C.free(unsafe.Pointer(c_c_name))

	ret := C.rbd_clone(C.rados_ioctx_t(image.ioctx.Pointer()),
		c_p_name, c_p_snapname,
		C.rados_ioctx_t(c_ioctx.Pointer()),
		c_c_name, C.uint64_t(features), &c_order)
	if ret < 0 {
		return nil, RBDError(int(ret))
	}

	return &Image{
		ioctx: c_ioctx,
		name:  c_name,
	}, nil
}

// int rbd_remove(rados_ioctx_t io, const char *name);
// int rbd_remove_with_progress(rados_ioctx_t io, const char *name,
//                  librbd_progress_fn_t cb, void *cbdata);
func (image *Image) Remove() error {
	var c_name *C.char = C.CString(image.name)
	defer C.free(unsafe.Pointer(c_name))
	return GetError(C.rbd_remove(C.rados_ioctx_t(image.ioctx.Pointer()), c_name))
}

// int rbd_rename(rados_ioctx_t src_io_ctx, const char *srcname, const char *destname);
func (image *Image) Rename(destname string) error {
	var c_srcname *C.char = C.CString(image.name)
	var c_destname *C.char = C.CString(destname)
	defer C.free(unsafe.Pointer(c_srcname))
	defer C.free(unsafe.Pointer(c_destname))
	err := RBDError(C.rbd_rename(C.rados_ioctx_t(image.ioctx.Pointer()),
		c_srcname, c_destname))
	if err == 0 {
		image.name = destname
		return nil
	}
	return err
}

// int rbd_open(rados_ioctx_t io, const char *name, rbd_image_t *image, const char *snap_name);
// int rbd_open_read_only(rados_ioctx_t io, const char *name, rbd_image_t *image,
//                const char *snap_name);
func (image *Image) Open(args ...interface{}) error {
	var c_image C.rbd_image_t
	var c_name *C.char = C.CString(image.name)
	var c_snap_name *C.char
	var ret C.int
	var read_only bool = false

	defer C.free(unsafe.Pointer(c_name))
	for _, arg := range args {
		switch t := arg.(type) {
		case string:
			if t != "" {
				c_snap_name = C.CString(t)
				defer C.free(unsafe.Pointer(c_snap_name))
			}
		case bool:
			read_only = t
		default:
			return errors.New("Unexpected argument")
		}
	}

	if read_only {
		ret = C.rbd_open_read_only(C.rados_ioctx_t(image.ioctx.Pointer()), c_name,
			&c_image, c_snap_name)
	} else {
		ret = C.rbd_open(C.rados_ioctx_t(image.ioctx.Pointer()), c_name,
			&c_image, c_snap_name)
	}

	image.image = c_image

	if ret != 0 {
		if ret == -C.ENOENT {
			return RbdErrorNotFound
		}
		return RBDError(ret)
	}
	return nil
}

// int rbd_close(rbd_image_t image);
func (image *Image) Close() error {
	if image.image == nil {
		return RbdErrorImageNotOpen
	}

	ret := C.rbd_close(image.image)
	if ret != 0 {
		return RBDError(ret)
	}
	image.image = nil
	return nil
}

// int rbd_resize(rbd_image_t image, uint64_t size);
func (image *Image) Resize(size uint64) error {
	if image.image == nil {
		return RbdErrorImageNotOpen
	}

	return GetError(C.rbd_resize(image.image, C.uint64_t(size)))
}

// int rbd_stat(rbd_image_t image, rbd_image_info_t *info, size_t infosize);
func (image *Image) Stat() (info *ImageInfo, err error) {
	if image.image == nil {
		return nil, RbdErrorImageNotOpen
	}

	var c_stat C.rbd_image_info_t
	ret := C.rbd_stat(image.image,
		&c_stat, C.size_t(unsafe.Sizeof(info)))
	if ret < 0 {
		return info, RBDError(int(ret))
	}

	return &ImageInfo{
		Size:              uint64(c_stat.size),
		Obj_size:          uint64(c_stat.obj_size),
		Num_objs:          uint64(c_stat.num_objs),
		Order:             int(c_stat.order),
		Block_name_prefix: C.GoString((*C.char)(&c_stat.block_name_prefix[0])),
		Parent_pool:       int64(c_stat.parent_pool),
		Parent_name:       C.GoString((*C.char)(&c_stat.parent_name[0]))}, nil
}

// int rbd_get_old_format(rbd_image_t image, uint8_t *old);
func (image *Image) IsOldFormat() (old_format bool, err error) {
	if image.image == nil {
		return false, RbdErrorImageNotOpen
	}

	var c_old_format C.uint8_t
	ret := C.rbd_get_old_format(image.image,
		&c_old_format)
	if ret < 0 {
		return false, RBDError(int(ret))
	}

	return c_old_format != 0, nil
}

// int rbd_size(rbd_image_t image, uint64_t *size);
func (image *Image) GetSize() (size uint64, err error) {
	if image.image == nil {
		return 0, RbdErrorImageNotOpen
	}

	ret := C.rbd_get_size(image.image,
		(*C.uint64_t)(&size))
	if ret < 0 {
		return 0, RBDError(int(ret))
	}

	return size, nil
}

// int rbd_get_features(rbd_image_t image, uint64_t *features);
func (image *Image) GetFeatures() (features uint64, err error) {
	if image.image == nil {
		return 0, RbdErrorImageNotOpen
	}

	ret := C.rbd_get_features(image.image,
		(*C.uint64_t)(&features))
	if ret < 0 {
		return 0, RBDError(int(ret))
	}

	return features, nil
}

// int rbd_get_stripe_unit(rbd_image_t image, uint64_t *stripe_unit);
func (image *Image) GetStripeUnit() (stripe_unit uint64, err error) {
	if image.image == nil {
		return 0, RbdErrorImageNotOpen
	}

	ret := C.rbd_get_stripe_unit(image.image, (*C.uint64_t)(&stripe_unit))
	if ret < 0 {
		return 0, RBDError(int(ret))
	}

	return stripe_unit, nil
}

// int rbd_get_stripe_count(rbd_image_t image, uint64_t *stripe_count);
func (image *Image) GetStripeCount() (stripe_count uint64, err error) {
	if image.image == nil {
		return 0, RbdErrorImageNotOpen
	}

	ret := C.rbd_get_stripe_count(image.image, (*C.uint64_t)(&stripe_count))
	if ret < 0 {
		return 0, RBDError(int(ret))
	}

	return stripe_count, nil
}

// int rbd_get_overlap(rbd_image_t image, uint64_t *overlap);
func (image *Image) GetOverlap() (overlap uint64, err error) {
	if image.image == nil {
		return 0, RbdErrorImageNotOpen
	}

	ret := C.rbd_get_overlap(image.image, (*C.uint64_t)(&overlap))
	if ret < 0 {
		return overlap, RBDError(int(ret))
	}

	return overlap, nil
}

// int rbd_copy(rbd_image_t image, rados_ioctx_t dest_io_ctx, const char *destname);
// int rbd_copy2(rbd_image_t src, rbd_image_t dest);
// int rbd_copy_with_progress(rbd_image_t image, rados_ioctx_t dest_p, const char *destname,
//                librbd_progress_fn_t cb, void *cbdata);
// int rbd_copy_with_progress2(rbd_image_t src, rbd_image_t dest,
//                librbd_progress_fn_t cb, void *cbdata);
func (image *Image) Copy(args ...interface{}) error {
	if image.image == nil {
		return RbdErrorImageNotOpen
	}

	switch t := args[0].(type) {
	case rados.IOContext:
		switch t2 := args[1].(type) {
		case string:
			var c_destname *C.char = C.CString(t2)
			defer C.free(unsafe.Pointer(c_destname))
			return RBDError(C.rbd_copy(image.image,
				C.rados_ioctx_t(t.Pointer()),
				c_destname))
		default:
			return errors.New("Must specify destname")
		}
	case Image:
		var dest Image = t
		if dest.image == nil {
			return errors.New(fmt.Sprintf("RBD image %s is not open", dest.name))
		}
		return GetError(C.rbd_copy2(image.image,
			dest.image))
	default:
		return errors.New("Must specify either destination pool " +
			"or destination image")
	}
}

// int rbd_flatten(rbd_image_t image);
func (image *Image) Flatten() error {
	if image.image == nil {
		return errors.New(fmt.Sprintf("RBD image %s is not open", image.name))
	}

	return GetError(C.rbd_flatten(image.image))
}

// ssize_t rbd_list_children(rbd_image_t image, char *pools, size_t *pools_len,
//               char *images, size_t *images_len);
func (image *Image) ListChildren() (pools []string, images []string, err error) {
	if image.image == nil {
		return nil, nil, RbdErrorImageNotOpen
	}

	var c_pools_len, c_images_len C.size_t

	ret := C.rbd_list_children(image.image,
		nil, &c_pools_len,
		nil, &c_images_len)
	if ret < 0 {
		return nil, nil, RBDError(int(ret))
	}

	pools_buf := make([]byte, c_pools_len)
	images_buf := make([]byte, c_images_len)

	ret = C.rbd_list_children(image.image,
		(*C.char)(unsafe.Pointer(&pools_buf[0])),
		&c_pools_len,
		(*C.char)(unsafe.Pointer(&images_buf[0])),
		&c_images_len)
	if ret < 0 {
		return nil, nil, RBDError(int(ret))
	}

	tmp := bytes.Split(pools_buf[:c_pools_len-1], []byte{0})
	for _, s := range tmp {
		if len(s) > 0 {
			name := C.GoString((*C.char)(unsafe.Pointer(&s[0])))
			pools = append(pools, name)
		}
	}

	tmp = bytes.Split(images_buf[:c_images_len-1], []byte{0})
	for _, s := range tmp {
		if len(s) > 0 {
			name := C.GoString((*C.char)(unsafe.Pointer(&s[0])))
			images = append(images, name)
		}
	}

	return pools, images, nil
}

// ssize_t rbd_list_lockers(rbd_image_t image, int *exclusive,
//              char *tag, size_t *tag_len,
//              char *clients, size_t *clients_len,
//              char *cookies, size_t *cookies_len,
//              char *addrs, size_t *addrs_len);
func (image *Image) ListLockers() (tag string, lockers []Locker, err error) {
	if image.image == nil {
		return "", nil, RbdErrorImageNotOpen
	}

	var c_exclusive C.int
	var c_tag_len, c_clients_len, c_cookies_len, c_addrs_len C.size_t

	C.rbd_list_lockers(image.image, &c_exclusive,
		nil, (*C.size_t)(&c_tag_len),
		nil, (*C.size_t)(&c_clients_len),
		nil, (*C.size_t)(&c_cookies_len),
		nil, (*C.size_t)(&c_addrs_len))

	tag_buf := make([]byte, c_tag_len)
	clients_buf := make([]byte, c_clients_len)
	cookies_buf := make([]byte, c_cookies_len)
	addrs_buf := make([]byte, c_addrs_len)

	C.rbd_list_lockers(image.image, &c_exclusive,
		(*C.char)(unsafe.Pointer(&tag_buf[0])), (*C.size_t)(&c_tag_len),
		(*C.char)(unsafe.Pointer(&clients_buf[0])), (*C.size_t)(&c_clients_len),
		(*C.char)(unsafe.Pointer(&cookies_buf[0])), (*C.size_t)(&c_cookies_len),
		(*C.char)(unsafe.Pointer(&addrs_buf[0])), (*C.size_t)(&c_addrs_len))

	clients := split(clients_buf)
	cookies := split(cookies_buf)
	addrs := split(addrs_buf)

	lockers = make([]Locker, c_clients_len)
	for i := 0; i < int(c_clients_len); i++ {
		lockers[i] = Locker{Client: clients[i],
			Cookie: cookies[i],
			Addr:   addrs[i]}
	}

	return string(tag_buf), lockers, nil
}

// int rbd_lock_exclusive(rbd_image_t image, const char *cookie);
func (image *Image) LockExclusive(cookie string) error {
	if image.image == nil {
		return RbdErrorImageNotOpen
	}

	var c_cookie *C.char = C.CString(cookie)
	defer C.free(unsafe.Pointer(c_cookie))

	return GetError(C.rbd_lock_exclusive(image.image, c_cookie))
}

// int rbd_lock_shared(rbd_image_t image, const char *cookie, const char *tag);
func (image *Image) LockShared(cookie string, tag string) error {
	if image.image == nil {
		return RbdErrorImageNotOpen
	}

	var c_cookie *C.char = C.CString(cookie)
	var c_tag *C.char = C.CString(tag)
	defer C.free(unsafe.Pointer(c_cookie))
	defer C.free(unsafe.Pointer(c_tag))

	return GetError(C.rbd_lock_shared(image.image, c_cookie, c_tag))
}

// int rbd_lock_shared(rbd_image_t image, const char *cookie, const char *tag);
func (image *Image) Unlock(cookie string) error {
	if image.image == nil {
		return RbdErrorImageNotOpen
	}

	var c_cookie *C.char = C.CString(cookie)
	defer C.free(unsafe.Pointer(c_cookie))

	return GetError(C.rbd_unlock(image.image, c_cookie))
}

// int rbd_break_lock(rbd_image_t image, const char *client, const char *cookie);
func (image *Image) BreakLock(client string, cookie string) error {
	if image.image == nil {
		return RbdErrorImageNotOpen
	}

	var c_client *C.char = C.CString(client)
	var c_cookie *C.char = C.CString(cookie)
	defer C.free(unsafe.Pointer(c_client))
	defer C.free(unsafe.Pointer(c_cookie))

	return GetError(C.rbd_break_lock(image.image, c_client, c_cookie))
}

// ssize_t rbd_read(rbd_image_t image, uint64_t ofs, size_t len, char *buf);
// TODO: int64_t rbd_read_iterate(rbd_image_t image, uint64_t ofs, size_t len,
//              int (*cb)(uint64_t, size_t, const char *, void *), void *arg);
// TODO: int rbd_read_iterate2(rbd_image_t image, uint64_t ofs, uint64_t len,
//               int (*cb)(uint64_t, size_t, const char *, void *), void *arg);
// TODO: int rbd_diff_iterate(rbd_image_t image,
//              const char *fromsnapname,
//              uint64_t ofs, uint64_t len,
//              int (*cb)(uint64_t, size_t, int, void *), void *arg);
func (image *Image) Read(data []byte) (n int, err error) {
	if image.image == nil {
		return 0, RbdErrorImageNotOpen
	}

	if len(data) == 0 {
		return 0, nil
	}

	ret := int(C.rbd_read(
		image.image,
		(C.uint64_t)(image.offset),
		(C.size_t)(len(data)),
		(*C.char)(unsafe.Pointer(&data[0]))))

	if ret < 0 {
		return 0, RBDError(ret)
	}

	image.offset += int64(ret)
	if ret < n {
		return ret, io.EOF
	}

	return ret, nil
}

// ssize_t rbd_write(rbd_image_t image, uint64_t ofs, size_t len, const char *buf);
func (image *Image) Write(data []byte) (n int, err error) {
	ret := int(C.rbd_write(image.image, C.uint64_t(image.offset),
		C.size_t(len(data)), (*C.char)(unsafe.Pointer(&data[0]))))

	if ret >= 0 {
		image.offset += int64(ret)
	}

	if ret != len(data) {
		err = RBDError(-1)
	}

	return ret, err
}

func (image *Image) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		image.offset = offset
	case 1:
		image.offset += offset
	case 2:
		stats, err := image.Stat()
		if err != nil {
			return 0, err
		}
		image.offset = int64(stats.Size) - offset
	default:
		return 0, errors.New("Wrong value for whence")
	}
	return image.offset, nil
}

// int rbd_discard(rbd_image_t image, uint64_t ofs, uint64_t len);
func (image *Image) Discard(ofs uint64, length uint64) error {
	return RBDError(C.rbd_discard(image.image, C.uint64_t(ofs),
		C.uint64_t(length)))
}

func (image *Image) ReadAt(data []byte, off int64) (n int, err error) {
	_, err = image.Seek(off, 0)
	if err != nil {
		return 0, err
	}
	return image.Read(data)
}

func (image *Image) WriteAt(data []byte, off int64) (n int, err error) {
	_, err = image.Seek(off, 0)
	if err != nil {
		return 0, err
	}
	return image.Write(data)
}

// int rbd_flush(rbd_image_t image);
func (image *Image) Flush() error {
	return GetError(C.rbd_flush(image.image))
}

// int rbd_snap_list(rbd_image_t image, rbd_snap_info_t *snaps, int *max_snaps);
// void rbd_snap_list_end(rbd_snap_info_t *snaps);
func (image *Image) GetSnapshotNames() (snaps []SnapInfo, err error) {
	if image.image == nil {
		return nil, RbdErrorImageNotOpen
	}

	var c_max_snaps C.int = 0

	ret := C.rbd_snap_list(image.image, nil, &c_max_snaps)

	c_snaps := make([]C.rbd_snap_info_t, c_max_snaps)
	snaps = make([]SnapInfo, c_max_snaps)

	ret = C.rbd_snap_list(image.image,
		&c_snaps[0], &c_max_snaps)
	if ret < 0 {
		return nil, RBDError(int(ret))
	}

	for i, s := range c_snaps {
		snaps[i] = SnapInfo{Id: uint64(s.id),
			Size: uint64(s.size),
			Name: C.GoString(s.name)}
	}

	C.rbd_snap_list_end(&c_snaps[0])
	return snaps[:len(snaps)-1], nil
}

// int rbd_snap_create(rbd_image_t image, const char *snapname);
func (image *Image) CreateSnapshot(snapname string) (*Snapshot, error) {
	if image.image == nil {
		return nil, RbdErrorImageNotOpen
	}

	var c_snapname *C.char = C.CString(snapname)
	defer C.free(unsafe.Pointer(c_snapname))

	ret := C.rbd_snap_create(image.image, c_snapname)
	if ret < 0 {
		return nil, RBDError(int(ret))
	}

	return &Snapshot{
		image: image,
		name:  snapname,
	}, nil
}

//
func (image *Image) GetSnapshot(snapname string) *Snapshot {
	return &Snapshot{
		image: image,
		name:  snapname,
	}
}

// int rbd_get_parent_info(rbd_image_t image,
//  char *parent_pool_name, size_t ppool_namelen, char *parent_name,
//  size_t pnamelen, char *parent_snap_name, size_t psnap_namelen)
func (image *Image) GetParentInfo(p_pool, p_name, p_snapname []byte) error {
	ret := C.rbd_get_parent_info(
		image.image,
		(*C.char)(unsafe.Pointer(&p_pool[0])),
		(C.size_t)(len(p_pool)),
		(*C.char)(unsafe.Pointer(&p_name[0])),
		(C.size_t)(len(p_name)),
		(*C.char)(unsafe.Pointer(&p_snapname[0])),
		(C.size_t)(len(p_snapname)))
	if ret == 0 {
		return nil
	} else {
		return RBDError(int(ret))
	}
}

// int rbd_snap_remove(rbd_image_t image, const char *snapname);
func (snapshot *Snapshot) Remove() error {
	var c_snapname *C.char = C.CString(snapshot.name)
	defer C.free(unsafe.Pointer(c_snapname))

	return GetError(C.rbd_snap_remove(snapshot.image.image, c_snapname))
}

// int rbd_snap_rollback(rbd_image_t image, const char *snapname);
// int rbd_snap_rollback_with_progress(rbd_image_t image, const char *snapname,
//                  librbd_progress_fn_t cb, void *cbdata);
func (snapshot *Snapshot) Rollback() error {
	var c_snapname *C.char = C.CString(snapshot.name)
	defer C.free(unsafe.Pointer(c_snapname))

	return GetError(C.rbd_snap_rollback(snapshot.image.image, c_snapname))
}

// int rbd_snap_protect(rbd_image_t image, const char *snap_name);
func (snapshot *Snapshot) Protect() error {
	var c_snapname *C.char = C.CString(snapshot.name)
	defer C.free(unsafe.Pointer(c_snapname))

	return GetError(C.rbd_snap_protect(snapshot.image.image, c_snapname))
}

// int rbd_snap_unprotect(rbd_image_t image, const char *snap_name);
func (snapshot *Snapshot) Unprotect() error {
	var c_snapname *C.char = C.CString(snapshot.name)
	defer C.free(unsafe.Pointer(c_snapname))

	return GetError(C.rbd_snap_unprotect(snapshot.image.image, c_snapname))
}

// int rbd_snap_is_protected(rbd_image_t image, const char *snap_name,
//               int *is_protected);
func (snapshot *Snapshot) IsProtected() (bool, error) {
	var c_is_protected C.int
	var c_snapname *C.char = C.CString(snapshot.name)
	defer C.free(unsafe.Pointer(c_snapname))

	ret := C.rbd_snap_is_protected(snapshot.image.image, c_snapname,
		&c_is_protected)
	if ret < 0 {
		return false, RBDError(int(ret))
	}

	return c_is_protected != 0, nil
}

// int rbd_snap_set(rbd_image_t image, const char *snapname);
func (snapshot *Snapshot) Set() error {
	var c_snapname *C.char = C.CString(snapshot.name)
	defer C.free(unsafe.Pointer(c_snapname))

	return GetError(C.rbd_snap_set(snapshot.image.image, c_snapname))
}
