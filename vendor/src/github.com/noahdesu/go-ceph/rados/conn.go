package rados

// #cgo LDFLAGS: -lrados
// #include <stdlib.h>
// #include <rados/librados.h>
import "C"

import "unsafe"
import "bytes"

// ClusterStat represents Ceph cluster statistics.
type ClusterStat struct {
	Kb          uint64
	Kb_used     uint64
	Kb_avail    uint64
	Num_objects uint64
}

// Conn is a connection handle to a Ceph cluster.
type Conn struct {
	cluster C.rados_t
}

// PingMonitor sends a ping to a monitor and returns the reply.
func (c *Conn) PingMonitor(id string) (string, error) {
	c_id := C.CString(id)
	defer C.free(unsafe.Pointer(c_id))

	var strlen C.size_t
	var strout *C.char

	ret := C.rados_ping_monitor(c.cluster, c_id, &strout, &strlen)
	defer C.rados_buffer_free(strout)

	if ret == 0 {
		reply := C.GoStringN(strout, (C.int)(strlen))
		return reply, nil
	} else {
		return "", RadosError(int(ret))
	}
}

// Connect establishes a connection to a RADOS cluster. It returns an error,
// if any.
func (c *Conn) Connect() error {
	ret := C.rados_connect(c.cluster)
	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// Shutdown disconnects from the cluster.
func (c *Conn) Shutdown() {
	C.rados_shutdown(c.cluster)
}

// ReadConfigFile configures the connection using a Ceph configuration file.
func (c *Conn) ReadConfigFile(path string) error {
	c_path := C.CString(path)
	defer C.free(unsafe.Pointer(c_path))
	ret := C.rados_conf_read_file(c.cluster, c_path)
	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// ReadDefaultConfigFile configures the connection using a Ceph configuration
// file located at default locations.
func (c *Conn) ReadDefaultConfigFile() error {
	ret := C.rados_conf_read_file(c.cluster, nil)
	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

func (c *Conn) OpenIOContext(pool string) (*IOContext, error) {
	c_pool := C.CString(pool)
	defer C.free(unsafe.Pointer(c_pool))
	ioctx := &IOContext{}
	ret := C.rados_ioctx_create(c.cluster, c_pool, &ioctx.ioctx)
	if ret == 0 {
		return ioctx, nil
	} else {
		return nil, RadosError(int(ret))
	}
}

// ListPools returns the names of all existing pools.
func (c *Conn) ListPools() (names []string, err error) {
	buf := make([]byte, 4096)
	for {
		ret := int(C.rados_pool_list(c.cluster,
			(*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))))
		if ret < 0 {
			return nil, RadosError(int(ret))
		}

		if ret > len(buf) {
			buf = make([]byte, ret)
			continue
		}

		tmp := bytes.SplitAfter(buf[:ret-1], []byte{0})
		for _, s := range tmp {
			if len(s) > 0 {
				name := C.GoString((*C.char)(unsafe.Pointer(&s[0])))
				names = append(names, name)
			}
		}

		return names, nil
	}
}

// SetConfigOption sets the value of the configuration option identified by
// the given name.
func (c *Conn) SetConfigOption(option, value string) error {
	c_opt, c_val := C.CString(option), C.CString(value)
	defer C.free(unsafe.Pointer(c_opt))
	defer C.free(unsafe.Pointer(c_val))
	ret := C.rados_conf_set(c.cluster, c_opt, c_val)
	if ret < 0 {
		return RadosError(int(ret))
	} else {
		return nil
	}
}

// GetConfigOption returns the value of the Ceph configuration option
// identified by the given name.
func (c *Conn) GetConfigOption(name string) (value string, err error) {
	buf := make([]byte, 4096)
	c_name := C.CString(name)
	defer C.free(unsafe.Pointer(c_name))
	ret := int(C.rados_conf_get(c.cluster, c_name,
		(*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))))
	// FIXME: ret may be -ENAMETOOLONG if the buffer is not large enough. We
	// can handle this case, but we need a reliable way to test for
	// -ENAMETOOLONG constant. Will the syscall/Errno stuff in Go help?
	if ret == 0 {
		value = C.GoString((*C.char)(unsafe.Pointer(&buf[0])))
		return value, nil
	} else {
		return "", RadosError(ret)
	}
}

// WaitForLatestOSDMap blocks the caller until the latest OSD map has been
// retrieved.
func (c *Conn) WaitForLatestOSDMap() error {
	ret := C.rados_wait_for_latest_osdmap(c.cluster)
	if ret < 0 {
		return RadosError(int(ret))
	} else {
		return nil
	}
}

// GetClusterStat returns statistics about the cluster associated with the
// connection.
func (c *Conn) GetClusterStats() (stat ClusterStat, err error) {
	c_stat := C.struct_rados_cluster_stat_t{}
	ret := C.rados_cluster_stat(c.cluster, &c_stat)
	if ret < 0 {
		return ClusterStat{}, RadosError(int(ret))
	} else {
		return ClusterStat{
			Kb:          uint64(c_stat.kb),
			Kb_used:     uint64(c_stat.kb_used),
			Kb_avail:    uint64(c_stat.kb_avail),
			Num_objects: uint64(c_stat.num_objects),
		}, nil
	}
}

// ParseCmdLineArgs configures the connection from command line arguments.
func (c *Conn) ParseCmdLineArgs(args []string) error {
	// add an empty element 0 -- Ceph treats the array as the actual contents
	// of argv and skips the first element (the executable name)
	argc := C.int(len(args) + 1)
	argv := make([]*C.char, argc)

	// make the first element a string just in case it is ever examined
	argv[0] = C.CString("placeholder")
	defer C.free(unsafe.Pointer(argv[0]))

	for i, arg := range args {
		argv[i+1] = C.CString(arg)
		defer C.free(unsafe.Pointer(argv[i+1]))
	}

	ret := C.rados_conf_parse_argv(c.cluster, argc, &argv[0])
	if ret < 0 {
		return RadosError(int(ret))
	} else {
		return nil
	}
}

// ParseDefaultConfigEnv configures the connection from the default Ceph
// environment variable(s).
func (c *Conn) ParseDefaultConfigEnv() error {
	ret := C.rados_conf_parse_env(c.cluster, nil)
	if ret == 0 {
		return nil
	} else {
		return RadosError(int(ret))
	}
}

// GetFSID returns the fsid of the cluster as a hexadecimal string. The fsid
// is a unique identifier of an entire Ceph cluster.
func (c *Conn) GetFSID() (fsid string, err error) {
	buf := make([]byte, 37)
	ret := int(C.rados_cluster_fsid(c.cluster,
		(*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))))
	// FIXME: the success case isn't documented correctly in librados.h
	if ret == 36 {
		fsid = C.GoString((*C.char)(unsafe.Pointer(&buf[0])))
		return fsid, nil
	} else {
		return "", RadosError(int(ret))
	}
}

// GetInstanceID returns a globally unique identifier for the cluster
// connection instance.
func (c *Conn) GetInstanceID() uint64 {
	// FIXME: are there any error cases for this?
	return uint64(C.rados_get_instance_id(c.cluster))
}

// MakePool creates a new pool with default settings.
func (c *Conn) MakePool(name string) error {
	c_name := C.CString(name)
	defer C.free(unsafe.Pointer(c_name))
	ret := int(C.rados_pool_create(c.cluster, c_name))
	if ret == 0 {
		return nil
	} else {
		return RadosError(ret)
	}
}

// DeletePool deletes a pool and all the data inside the pool.
func (c *Conn) DeletePool(name string) error {
	c_name := C.CString(name)
	defer C.free(unsafe.Pointer(c_name))
	ret := int(C.rados_pool_delete(c.cluster, c_name))
	if ret == 0 {
		return nil
	} else {
		return RadosError(ret)
	}
}

// MonCommand sends a command to one of the monitors
func (c *Conn) MonCommand(args []byte) (buffer []byte, info string, err error) {
	argv := make([]*C.char, len(args))
	for i, _ := range args {
		argv[i] = (*C.char)(unsafe.Pointer(&args[i]))
	}

	var (
		outs, outbuf       *C.char
		outslen, outbuflen C.size_t
	)
	inbuf := C.CString("")
	defer C.free(unsafe.Pointer(inbuf))

	ret := C.rados_mon_command(c.cluster,
		&argv[0], C.size_t(len(args)),
		inbuf,       // bulk input (e.g. crush map)
		C.size_t(0), // length inbuf
		&outbuf,     // buffer
		&outbuflen,  // buffer length
		&outs,       // status string
		&outslen)

	if outslen > 0 {
		info = C.GoStringN(outs, C.int(outslen))
		C.free(unsafe.Pointer(outs))
	}
	if outbuflen > 0 {
		buffer = C.GoBytes(unsafe.Pointer(outbuf), C.int(outbuflen))
		C.free(unsafe.Pointer(outbuf))
	}
	if ret != 0 {
		err = RadosError(int(ret))
		return nil, info, err
	}

	return
}
