package rados

// #cgo LDFLAGS: -lrados
// #include <errno.h>
// #include <stdlib.h>
// #include <rados/librados.h>
import "C"

import "unsafe"
import "time"

// PoolStat represents Ceph pool statistics.
type PoolStat struct {
	// space used in bytes
	Num_bytes uint64
	// space used in KB
	Num_kb uint64
	// number of objects in the pool
	Num_objects uint64
	// number of clones of objects
	Num_object_clones uint64
	// num_objects * num_replicas
	Num_object_copies              uint64
	Num_objects_missing_on_primary uint64
	// number of objects found on no OSDs
	Num_objects_unfound uint64
	// number of objects replicated fewer times than they should be
	// (but found on at least one OSD)
	Num_objects_degraded uint64
	Num_rd               uint64
	Num_rd_kb            uint64
	Num_wr               uint64
	Num_wr_kb            uint64
}

// ObjectStat represents an object stat information
type ObjectStat struct {
	// current length in bytes
	Size uint64
	// last modification time
	ModTime time.Time
}

// IOContext represents a context for performing I/O within a pool.
type IOContext struct {
	ioctx C.rados_ioctx_t
}

// Pointer returns a uintptr representation of the IOContext.
func (ioctx *IOContext) Pointer() uintptr {
	return uintptr(ioctx.ioctx)
}

// Write writes len(data) bytes to the object with key oid starting at byte
// offset offset. It returns an error, if any.
func (ioctx *IOContext) Write(oid string, data []byte, offset uint64) error {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	ret := C.rados_write(ioctx.ioctx, c_oid,
		(*C.char)(unsafe.Pointer(&data[0])),
		(C.size_t)(len(data)),
		(C.uint64_t)(offset))

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// WriteFull writes len(data) bytes to the object with key oid.
// The object is filled with the provided data. If the object exists,
// it is atomically truncated and then written. It returns an error, if any.
func (ioctx *IOContext) WriteFull(oid string, data []byte) error {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	ret := C.rados_write_full(ioctx.ioctx, c_oid,
		(*C.char)(unsafe.Pointer(&data[0])),
		(C.size_t)(len(data)))

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// Read reads up to len(data) bytes from the object with key oid starting at byte
// offset offset. It returns the number of bytes read and an error, if any.
func (ioctx *IOContext) Read(oid string, data []byte, offset uint64) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	ret := C.rados_read(
		ioctx.ioctx,
		c_oid,
		(*C.char)(unsafe.Pointer(&data[0])),
		(C.size_t)(len(data)),
		(C.uint64_t)(offset))

	if ret >= 0 {
		return int(ret), nil
	} else {
		if ret == -C.ENOENT {
			return 0, RadosErrorNotFound
		}
		return 0, RadosError(int(ret))
	}
}

// Delete deletes the object with key oid. It returns an error, if any.
func (ioctx *IOContext) Delete(oid string) error {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	ret := C.rados_remove(ioctx.ioctx, c_oid)

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// Truncate resizes the object with key oid to size size. If the operation
// enlarges the object, the new area is logically filled with zeroes. If the
// operation shrinks the object, the excess data is removed. It returns an
// error, if any.
func (ioctx *IOContext) Truncate(oid string, size uint64) error {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	ret := C.rados_trunc(ioctx.ioctx, c_oid, (C.uint64_t)(size))

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// Destroy informs librados that the I/O context is no longer in use.
// Resources associated with the context may not be freed immediately, and the
// context should not be used again after calling this method.
func (ioctx *IOContext) Destroy() {
	C.rados_ioctx_destroy(ioctx.ioctx)
}

// Stat returns a set of statistics about the pool associated with this I/O
// context.
func (ioctx *IOContext) GetPoolStats() (stat PoolStat, err error) {
	c_stat := C.struct_rados_pool_stat_t{}
	ret := C.rados_ioctx_pool_stat(ioctx.ioctx, &c_stat)
	if ret < 0 {
		return PoolStat{}, RadosError(int(ret))
	} else {
		return PoolStat{
			Num_bytes:                      uint64(c_stat.num_bytes),
			Num_kb:                         uint64(c_stat.num_kb),
			Num_objects:                    uint64(c_stat.num_objects),
			Num_object_clones:              uint64(c_stat.num_object_clones),
			Num_object_copies:              uint64(c_stat.num_object_copies),
			Num_objects_missing_on_primary: uint64(c_stat.num_objects_missing_on_primary),
			Num_objects_unfound:            uint64(c_stat.num_objects_unfound),
			Num_objects_degraded:           uint64(c_stat.num_objects_degraded),
			Num_rd:                         uint64(c_stat.num_rd),
			Num_rd_kb:                      uint64(c_stat.num_rd_kb),
			Num_wr:                         uint64(c_stat.num_wr),
			Num_wr_kb:                      uint64(c_stat.num_wr_kb),
		}, nil
	}
}

// GetPoolName returns the name of the pool associated with the I/O context.
func (ioctx *IOContext) GetPoolName() (name string, err error) {
	buf := make([]byte, 128)
	for {
		ret := C.rados_ioctx_get_pool_name(ioctx.ioctx,
			(*C.char)(unsafe.Pointer(&buf[0])), C.unsigned(len(buf)))
		if ret == -34 { // FIXME
			buf = make([]byte, len(buf)*2)
			continue
		} else if ret < 0 {
			return "", RadosError(ret)
		}
		name = C.GoStringN((*C.char)(unsafe.Pointer(&buf[0])), ret)
		return name, nil
	}
}

// ObjectListFunc is the type of the function called for each object visited
// by ListObjects.
type ObjectListFunc func(oid string)

// ListObjects lists all of the objects in the pool associated with the I/O
// context, and called the provided listFn function for each object, passing
// to the function the name of the object.
func (ioctx *IOContext) ListObjects(listFn ObjectListFunc) error {
	var ctx C.rados_list_ctx_t
	ret := C.rados_objects_list_open(ioctx.ioctx, &ctx)
	if ret < 0 {
		return RadosError(ret)
	}
	defer func() { C.rados_objects_list_close(ctx) }()

	for {
		var c_entry *C.char
		ret := C.rados_objects_list_next(ctx, &c_entry, nil)
		if ret == -2 { // FIXME
			return nil
		} else if ret < 0 {
			return RadosError(ret)
		}
		listFn(C.GoString(c_entry))
	}

	panic("invalid state")
}

// Stat returns the size of the object and its last modification time
func (ioctx *IOContext) Stat(object string) (stat ObjectStat, err error) {
	var c_psize C.uint64_t
	var c_pmtime C.time_t
	c_object := C.CString(object)
	defer C.free(unsafe.Pointer(c_object))

	ret := C.rados_stat(
		ioctx.ioctx,
		c_object,
		&c_psize,
		&c_pmtime)

	if ret < 0 {
		return ObjectStat{}, RadosError(int(ret))
	} else {
		return ObjectStat{
			Size:    uint64(c_psize),
			ModTime: time.Unix(int64(c_pmtime), 0),
		}, nil
	}
}

// GetXattr gets an xattr with key `name`, it returns the length of
// the key read or an error if not successful
func (ioctx *IOContext) GetXattr(object string, name string, data []byte) (int, error) {
	c_object := C.CString(object)
	c_name := C.CString(name)
	defer C.free(unsafe.Pointer(c_object))
	defer C.free(unsafe.Pointer(c_name))

	ret := C.rados_getxattr(
		ioctx.ioctx,
		c_object,
		c_name,
		(*C.char)(unsafe.Pointer(&data[0])),
		(C.size_t)(len(data)))

	if ret >= 0 {
		return int(ret), nil
	} else {
		return 0, RadosError(int(ret))
	}
}

// Sets an xattr for an object with key `name` with value as `data`
func (ioctx *IOContext) SetXattr(object string, name string, data []byte) error {
	c_object := C.CString(object)
	c_name := C.CString(name)
	defer C.free(unsafe.Pointer(c_object))
	defer C.free(unsafe.Pointer(c_name))

	ret := C.rados_setxattr(
		ioctx.ioctx,
		c_object,
		c_name,
		(*C.char)(unsafe.Pointer(&data[0])),
		(C.size_t)(len(data)))

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// function that lists all the xattrs for an object, since xattrs are
// a k-v pair, this function returns a map of k-v pairs on
// success, error code on failure
func (ioctx *IOContext) ListXattrs(oid string) (map[string][]byte, error) {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	var it C.rados_xattrs_iter_t

	ret := C.rados_getxattrs(ioctx.ioctx, c_oid, &it)
	if ret < 0 {
		return nil, RadosError(ret)
	}
	defer func() { C.rados_getxattrs_end(it) }()
	m := make(map[string][]byte)
	for {
		var c_name, c_val *C.char
		var c_len C.size_t
		defer C.free(unsafe.Pointer(c_name))
		defer C.free(unsafe.Pointer(c_val))

		ret := C.rados_getxattrs_next(it, &c_name, &c_val, &c_len)
		if ret < 0 {
			return nil, RadosError(int(ret))
		}
		// rados api returns a null name,val & 0-length upon
		// end of iteration
		if c_name == nil {
			return m, nil // stop iteration
		}
		m[C.GoString(c_name)] = C.GoBytes(unsafe.Pointer(c_val), (C.int)(c_len))
	}
}

// Remove an xattr with key `name` from object `oid`
func (ioctx *IOContext) RmXattr(oid string, name string) error {
	c_oid := C.CString(oid)
	c_name := C.CString(name)
	defer C.free(unsafe.Pointer(c_oid))
	defer C.free(unsafe.Pointer(c_name))

	ret := C.rados_rmxattr(
		ioctx.ioctx,
		c_oid,
		c_name)

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// Append the map `pairs` to the omap `oid`
func (ioctx *IOContext) SetOmap(oid string, pairs map[string][]byte) error {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	var s C.size_t
	var c *C.char
	ptrSize := unsafe.Sizeof(c)

	c_keys := C.malloc(C.size_t(len(pairs)) * C.size_t(ptrSize))
	c_values := C.malloc(C.size_t(len(pairs)) * C.size_t(ptrSize))
	c_lengths := C.malloc(C.size_t(len(pairs)) * C.size_t(unsafe.Sizeof(s)))

	defer C.free(unsafe.Pointer(c_keys))
	defer C.free(unsafe.Pointer(c_values))
	defer C.free(unsafe.Pointer(c_lengths))

	i := 0
	for key, value := range pairs {
		// key
		c_key_ptr := (**C.char)(unsafe.Pointer(uintptr(c_keys) + uintptr(i)*ptrSize))
		*c_key_ptr = C.CString(key)
		defer C.free(unsafe.Pointer(*c_key_ptr))

		// value and its length
		c_value_ptr := (**C.char)(unsafe.Pointer(uintptr(c_values) + uintptr(i)*ptrSize))

		var c_length C.size_t
		if len(value) > 0 {
			*c_value_ptr = (*C.char)(unsafe.Pointer(&value[0]))
			c_length = C.size_t(len(value))
		} else {
			*c_value_ptr = nil
			c_length = C.size_t(0)
		}

		c_length_ptr := (*C.size_t)(unsafe.Pointer(uintptr(c_lengths) + uintptr(i)*ptrSize))
		*c_length_ptr = c_length

		i++
	}

	op := C.rados_create_write_op()
	C.rados_write_op_omap_set(
		op,
		(**C.char)(c_keys),
		(**C.char)(c_values),
		(*C.size_t)(c_lengths),
		C.size_t(len(pairs)))

	ret := C.rados_write_op_operate(op, ioctx.ioctx, c_oid, nil, 0)
	C.rados_release_write_op(op)

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// OmapListFunc is the type of the function called for each omap key
// visited by ListOmapValues
type OmapListFunc func(key string, value []byte)

// Iterate on a set of keys and their values from an omap
// `startAfter`: iterate only on the keys after this specified one
// `filterPrefix`: iterate only on the keys beginning with this prefix
// `maxReturn`: iterate no more than `maxReturn` key/value pairs
// `listFn`: the function called at each iteration
func (ioctx *IOContext) ListOmapValues(oid string, startAfter string, filterPrefix string, maxReturn int64, listFn OmapListFunc) error {
	c_oid := C.CString(oid)
	c_start_after := C.CString(startAfter)
	c_filter_prefix := C.CString(filterPrefix)
	c_max_return := C.uint64_t(maxReturn)

	defer C.free(unsafe.Pointer(c_oid))
	defer C.free(unsafe.Pointer(c_start_after))
	defer C.free(unsafe.Pointer(c_filter_prefix))

	op := C.rados_create_read_op()

	var c_iter C.rados_omap_iter_t
	var c_prval C.int
	C.rados_read_op_omap_get_vals(
		op,
		c_start_after,
		c_filter_prefix,
		c_max_return,
		&c_iter,
		&c_prval,
	)

	ret := C.rados_read_op_operate(op, ioctx.ioctx, c_oid, 0)

	if int(c_prval) != 0 {
		return RadosError(int(c_prval))
	} else if int(ret) != 0 {
		return RadosError(int(ret))
	}

	for {
		var c_key *C.char
		var c_val *C.char
		var c_len C.size_t

		ret = C.rados_omap_get_next(c_iter, &c_key, &c_val, &c_len)

		if int(ret) != 0 {
			return RadosError(int(ret))
		}

		if c_key == nil {
			break
		}

		listFn(C.GoString(c_key), C.GoBytes(unsafe.Pointer(c_val), C.int(c_len)))
	}

	C.rados_omap_get_end(c_iter)
	C.rados_release_read_op(op)

	return nil
}

// Fetch a set of keys and their values from an omap and returns then as a map
// `startAfter`: retrieve only the keys after this specified one
// `filterPrefix`: retrieve only the keys beginning with this prefix
// `maxReturn`: retrieve no more than `maxReturn` key/value pairs
func (ioctx *IOContext) GetOmapValues(oid string, startAfter string, filterPrefix string, maxReturn int64) (map[string][]byte, error) {
	omap := map[string][]byte{}

	err := ioctx.ListOmapValues(
		oid, startAfter, filterPrefix, maxReturn,
		func(key string, value []byte) {
			omap[key] = value
		},
	)

	return omap, err
}

// Fetch all the keys and their values from an omap and returns then as a map
// `startAfter`: retrieve only the keys after this specified one
// `filterPrefix`: retrieve only the keys beginning with this prefix
// `iteratorSize`: internal number of keys to fetch during a read operation
func (ioctx *IOContext) GetAllOmapValues(oid string, startAfter string, filterPrefix string, iteratorSize int64) (map[string][]byte, error) {
	omap := map[string][]byte{}
	omapSize := 0

	for {
		err := ioctx.ListOmapValues(
			oid, startAfter, filterPrefix, iteratorSize,
			func(key string, value []byte) {
				omap[key] = value
				startAfter = key
			},
		)

		if err != nil {
			return omap, err
		}

		// End of omap
		if len(omap) == omapSize {
			break
		}

		omapSize = len(omap)
	}

	return omap, nil
}

// Remove the specified `keys` from the omap `oid`
func (ioctx *IOContext) RmOmapKeys(oid string, keys []string) error {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	var c *C.char
	ptrSize := unsafe.Sizeof(c)

	c_keys := C.malloc(C.size_t(len(keys)) * C.size_t(ptrSize))
	defer C.free(unsafe.Pointer(c_keys))

	i := 0
	for _, key := range keys {
		c_key_ptr := (**C.char)(unsafe.Pointer(uintptr(c_keys) + uintptr(i)*ptrSize))
		*c_key_ptr = C.CString(key)
		defer C.free(unsafe.Pointer(*c_key_ptr))
		i++
	}

	op := C.rados_create_write_op()
	C.rados_write_op_omap_rm_keys(
		op,
		(**C.char)(c_keys),
		C.size_t(len(keys)))

	ret := C.rados_write_op_operate(op, ioctx.ioctx, c_oid, nil, 0)
	C.rados_release_write_op(op)

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// Clear the omap `oid`
func (ioctx *IOContext) CleanOmap(oid string) error {
	c_oid := C.CString(oid)
	defer C.free(unsafe.Pointer(c_oid))

	op := C.rados_create_write_op()
	C.rados_write_op_omap_clear(op)

	ret := C.rados_write_op_operate(op, ioctx.ioctx, c_oid, nil, 0)
	C.rados_release_write_op(op)

	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}
