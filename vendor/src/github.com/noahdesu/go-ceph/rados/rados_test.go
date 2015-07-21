package rados_test

import "testing"

//import "bytes"
import "github.com/noahdesu/go-ceph/rados"
import "github.com/stretchr/testify/assert"
import "os"
import "os/exec"
import "io"
import "io/ioutil"
import "time"
import "net"
import "fmt"
import "sort"
import "encoding/json"

func GetUUID() string {
	out, _ := exec.Command("uuidgen").Output()
	return string(out[:36])
}

func TestVersion(t *testing.T) {
	var major, minor, patch = rados.Version()
	assert.False(t, major < 0 || major > 1000, "invalid major")
	assert.False(t, minor < 0 || minor > 1000, "invalid minor")
	assert.False(t, patch < 0 || patch > 1000, "invalid patch")
}

func TestGetSetConfigOption(t *testing.T) {
	conn, _ := rados.NewConn()

	// rejects invalid options
	err := conn.SetConfigOption("wefoijweojfiw", "welfkwjelkfj")
	assert.Error(t, err, "Invalid option")

	// verify SetConfigOption changes a values
	log_file_val, err := conn.GetConfigOption("log_file")
	assert.NotEqual(t, log_file_val, "/dev/null")

	err = conn.SetConfigOption("log_file", "/dev/null")
	assert.NoError(t, err, "Invalid option")

	log_file_val, err = conn.GetConfigOption("log_file")
	assert.Equal(t, log_file_val, "/dev/null")
}

func TestParseDefaultConfigEnv(t *testing.T) {
	conn, _ := rados.NewConn()

	log_file_val, _ := conn.GetConfigOption("log_file")
	assert.NotEqual(t, log_file_val, "/dev/null")

	err := os.Setenv("CEPH_ARGS", "--log-file /dev/null")
	assert.NoError(t, err)

	err = conn.ParseDefaultConfigEnv()
	assert.NoError(t, err)

	log_file_val, _ = conn.GetConfigOption("log_file")
	assert.Equal(t, log_file_val, "/dev/null")
}

func TestParseCmdLineArgs(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()

	mon_host_val, _ := conn.GetConfigOption("mon_host")
	assert.NotEqual(t, mon_host_val, "1.1.1.1")

	args := []string{"--mon-host", "1.1.1.1"}
	err := conn.ParseCmdLineArgs(args)
	assert.NoError(t, err)

	mon_host_val, _ = conn.GetConfigOption("mon_host")
	assert.Equal(t, mon_host_val, "1.1.1.1")
}

func TestGetClusterStats(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	// grab current stats
	prev_stat, err := conn.GetClusterStats()
	fmt.Printf("prev_stat: %+v\n", prev_stat)
	assert.NoError(t, err)

	// make some changes to the cluster
	buf := make([]byte, 1<<20)
	for i := 0; i < 10; i++ {
		objname := GetUUID()
		pool.Write(objname, buf, 0)
	}

	// wait a while for the stats to change
	for i := 0; i < 30; i++ {
		stat, err := conn.GetClusterStats()
		assert.NoError(t, err)

		// wait for something to change
		if stat == prev_stat {
			fmt.Printf("curr_stat: %+v (trying again...)\n", stat)
			time.Sleep(time.Second)
		} else {
			// success
			fmt.Printf("curr_stat: %+v (change detected)\n", stat)
			conn.Shutdown()
			return
		}
	}

	pool.Destroy()
	conn.Shutdown()
	t.Error("Cluster stats aren't changing")
}

func TestGetFSID(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	fsid, err := conn.GetFSID()
	assert.NoError(t, err)
	assert.NotEqual(t, fsid, "")

	conn.Shutdown()
}

func TestGetInstanceID(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	id := conn.GetInstanceID()
	assert.NotEqual(t, id, 0)

	conn.Shutdown()
}

func TestMakeDeletePool(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	// get current list of pool
	pools, err := conn.ListPools()
	assert.NoError(t, err)

	// check that new pool name is unique
	new_name := GetUUID()
	for _, poolname := range pools {
		if new_name == poolname {
			t.Error("Random pool name exists!")
			return
		}
	}

	// create pool
	err = conn.MakePool(new_name)
	assert.NoError(t, err)

	// get updated list of pools
	pools, err = conn.ListPools()
	assert.NoError(t, err)

	// verify that the new pool name exists
	found := false
	for _, poolname := range pools {
		if new_name == poolname {
			found = true
		}
	}

	if !found {
		t.Error("Cannot find newly created pool")
	}

	// delete the pool
	err = conn.DeletePool(new_name)
	assert.NoError(t, err)

	// verify that it is gone

	// get updated list of pools
	pools, err = conn.ListPools()
	assert.NoError(t, err)

	// verify that the new pool name exists
	found = false
	for _, poolname := range pools {
		if new_name == poolname {
			found = true
		}
	}

	if found {
		t.Error("Deleted pool still exists")
	}

	conn.Shutdown()
}

func TestPingMonitor(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	// mon id that should work with vstart.sh
	reply, err := conn.PingMonitor("a")
	if err == nil {
		assert.NotEqual(t, reply, "")
		return
	}

	// mon id that should work with micro-osd.sh
	reply, err = conn.PingMonitor("0")
	if err == nil {
		assert.NotEqual(t, reply, "")
		return
	}

	// try to use a hostname as the monitor id
	mon_addr, _ := conn.GetConfigOption("mon_host")
	hosts, _ := net.LookupAddr(mon_addr)
	for _, host := range hosts {
		reply, err := conn.PingMonitor(host)
		if err == nil {
			assert.NotEqual(t, reply, "")
			return
		}
	}

	t.Error("Could not find a valid monitor id")

	conn.Shutdown()
}

func TestReadConfigFile(t *testing.T) {
	conn, _ := rados.NewConn()

	// check current log_file value
	log_file_val, err := conn.GetConfigOption("log_file")
	assert.NoError(t, err)
	assert.NotEqual(t, log_file_val, "/dev/null")

	// create a temporary ceph.conf file that changes the log_file conf
	// option.
	file, err := ioutil.TempFile("/tmp", "go-rados")
	assert.NoError(t, err)

	_, err = io.WriteString(file, "[global]\nlog_file = /dev/null\n")
	assert.NoError(t, err)

	// parse the config file
	err = conn.ReadConfigFile(file.Name())
	assert.NoError(t, err)

	// check current log_file value
	log_file_val, err = conn.GetConfigOption("log_file")
	assert.NoError(t, err)
	assert.Equal(t, log_file_val, "/dev/null")

	// cleanup
	file.Close()
	os.Remove(file.Name())
}

func TestWaitForLatestOSDMap(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	err := conn.WaitForLatestOSDMap()
	assert.NoError(t, err)

	conn.Shutdown()
}

func TestReadWrite(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	// make pool
	pool_name := GetUUID()
	err := conn.MakePool(pool_name)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(pool_name)
	assert.NoError(t, err)

	bytes_in := []byte("input data")
	err = pool.Write("obj", bytes_in, 0)
	assert.NoError(t, err)

	bytes_out := make([]byte, len(bytes_in))
	n_out, err := pool.Read("obj", bytes_out, 0)

	assert.Equal(t, n_out, len(bytes_in))
	assert.Equal(t, bytes_in, bytes_out)

	bytes_in = []byte("input another data")
	err = pool.WriteFull("obj", bytes_in)
	assert.NoError(t, err)

	bytes_out = make([]byte, len(bytes_in))
	n_out, err = pool.Read("obj", bytes_out, 0)

	assert.Equal(t, n_out, len(bytes_in))
	assert.Equal(t, bytes_in, bytes_out)

	pool.Destroy()
}

func TestObjectStat(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	pool_name := GetUUID()
	err := conn.MakePool(pool_name)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(pool_name)
	assert.NoError(t, err)

	bytes_in := []byte("input data")
	err = pool.Write("obj", bytes_in, 0)
	assert.NoError(t, err)

	stat, err := pool.Stat("obj")
	assert.Equal(t, uint64(len(bytes_in)), stat.Size)
	assert.NotNil(t, stat.ModTime)

	pool.Destroy()
	conn.Shutdown()
}

func TestGetPoolStats(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	// grab current stats
	prev_stat, err := pool.GetPoolStats()
	fmt.Printf("prev_stat: %+v\n", prev_stat)
	assert.NoError(t, err)

	// make some changes to the cluster
	buf := make([]byte, 1<<20)
	for i := 0; i < 10; i++ {
		objname := GetUUID()
		pool.Write(objname, buf, 0)
	}

	// wait a while for the stats to change
	for i := 0; i < 30; i++ {
		stat, err := pool.GetPoolStats()
		assert.NoError(t, err)

		// wait for something to change
		if stat == prev_stat {
			fmt.Printf("curr_stat: %+v (trying again...)\n", stat)
			time.Sleep(time.Second)
		} else {
			// success
			fmt.Printf("curr_stat: %+v (change detected)\n", stat)
			conn.Shutdown()
			return
		}
	}

	pool.Destroy()
	conn.Shutdown()
	t.Error("Pool stats aren't changing")
}

func TestGetPoolName(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	ioctx, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	poolname_ret, err := ioctx.GetPoolName()
	assert.NoError(t, err)

	assert.Equal(t, poolname, poolname_ret)

	ioctx.Destroy()
	conn.Shutdown()
}

func TestMonCommand(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	command, err := json.Marshal(map[string]string{"prefix": "df", "format": "json"})
	assert.NoError(t, err)

	buf, info, err := conn.MonCommand(command)
	assert.NoError(t, err)
	assert.Equal(t, info, "")

	var message map[string]interface{}
	err = json.Unmarshal(buf, &message)
	assert.NoError(t, err)

	conn.Shutdown()
}

func TestObjectIterator(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	ioctx, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	objectList := []string{}
	err = ioctx.ListObjects(func(oid string) {
		objectList = append(objectList, oid)
	})
	assert.NoError(t, err)
	assert.True(t, len(objectList) == 0)

	createdList := []string{}
	for i := 0; i < 200; i++ {
		oid := GetUUID()
		bytes_in := []byte("input data")
		err = ioctx.Write(oid, bytes_in, 0)
		assert.NoError(t, err)
		createdList = append(createdList, oid)
	}
	assert.True(t, len(createdList) == 200)

	err = ioctx.ListObjects(func(oid string) {
		objectList = append(objectList, oid)
	})
	assert.NoError(t, err)
	assert.Equal(t, len(objectList), len(createdList))

	sort.Strings(objectList)
	sort.Strings(createdList)

	assert.Equal(t, objectList, createdList)
}

func TestNewConnWithUser(t *testing.T) {
	_, err := rados.NewConnWithUser("admin")
	assert.Equal(t, err, nil)
}

func TestNewConnWithClusterAndUser(t *testing.T) {
	_, err := rados.NewConnWithClusterAndUser("ceph", "client.admin")
	assert.Equal(t, err, nil)
}

func TestReadWriteXattr(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	// make pool
	pool_name := GetUUID()
	err := conn.MakePool(pool_name)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(pool_name)
	assert.NoError(t, err)

	bytes_in := []byte("input data")
	err = pool.Write("obj", bytes_in, 0)
	assert.NoError(t, err)

	my_xattr_in := []byte("my_value")
	err = pool.SetXattr("obj", "my_key", my_xattr_in)
	assert.NoError(t, err)

	my_xattr_out := make([]byte, len(my_xattr_in))
	n_out, err := pool.GetXattr("obj", "my_key", my_xattr_out)

	assert.Equal(t, n_out, len(my_xattr_in))
	assert.Equal(t, my_xattr_in, my_xattr_out)

	pool.Destroy()
}

func TestListXattrs(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	// make pool
	pool_name := GetUUID()
	err := conn.MakePool(pool_name)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(pool_name)
	assert.NoError(t, err)

	bytes_in := []byte("input data")
	err = pool.Write("obj", bytes_in, 0)
	assert.NoError(t, err)

	input_xattrs := make(map[string][]byte)
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("key_%d", i)
		data := []byte(GetUUID())
		err = pool.SetXattr("obj", name, data)
		assert.NoError(t, err)
		input_xattrs[name] = data
	}

	output_xattrs := make(map[string][]byte)
	output_xattrs, err = pool.ListXattrs("obj")
	assert.NoError(t, err)
	assert.Equal(t, len(input_xattrs), len(output_xattrs))
	assert.Equal(t, input_xattrs, output_xattrs)

	pool.Destroy()
}

func TestRmXattr(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	pool_name := GetUUID()
	err := conn.MakePool(pool_name)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(pool_name)
	assert.NoError(t, err)

	bytes_in := []byte("input data")
	err = pool.Write("obj", bytes_in, 0)
	assert.NoError(t, err)

	key := "key1"
	val := []byte("val1")
	err = pool.SetXattr("obj", key, val)
	assert.NoError(t, err)

	key = "key2"
	val = []byte("val2")
	err = pool.SetXattr("obj", key, val)
	assert.NoError(t, err)

	xattr_list := make(map[string][]byte)
	xattr_list, err = pool.ListXattrs("obj")
	assert.NoError(t, err)
	assert.Equal(t, len(xattr_list), 2)

	pool.RmXattr("obj", "key2")
	xattr_list, err = pool.ListXattrs("obj")
	assert.NoError(t, err)
	assert.Equal(t, len(xattr_list), 1)

	found := false
	for key, _ = range xattr_list {
		if key == "key2" {
			found = true
		}

	}

	if found {
		t.Error("Deleted pool still exists")
	}

	pool.Destroy()
}

func TestReadWriteOmap(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	pool_name := GetUUID()
	err := conn.MakePool(pool_name)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(pool_name)
	assert.NoError(t, err)

	// Set
	orig := map[string][]byte{
		"key1":          []byte("value1"),
		"key2":          []byte("value2"),
		"prefixed-key3": []byte("value3"),
		"empty":         []byte(""),
	}

	err = pool.SetOmap("obj", orig)
	assert.NoError(t, err)

	// List
	remaining := map[string][]byte{}
	for k, v := range orig {
		remaining[k] = v
	}

	err = pool.ListOmapValues("obj", "", "", 4, func(key string, value []byte) {
		assert.Equal(t, remaining[key], value)
		delete(remaining, key)
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(remaining))

	// Get (with a fixed number of keys)
	fetched, err := pool.GetOmapValues("obj", "", "", 4)
	assert.NoError(t, err)
	assert.Equal(t, orig, fetched)

	// Get All (with an iterator size bigger than the map size)
	fetched, err = pool.GetAllOmapValues("obj", "", "", 100)
	assert.NoError(t, err)
	assert.Equal(t, orig, fetched)

	// Get All (with an iterator size smaller than the map size)
	fetched, err = pool.GetAllOmapValues("obj", "", "", 1)
	assert.NoError(t, err)
	assert.Equal(t, orig, fetched)

	// Remove
	err = pool.RmOmapKeys("obj", []string{"key1", "prefixed-key3"})
	assert.NoError(t, err)

	fetched, err = pool.GetOmapValues("obj", "", "", 4)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]byte{
		"key2":  []byte("value2"),
		"empty": []byte(""),
	}, fetched)

	// Clear
	err = pool.CleanOmap("obj")
	assert.NoError(t, err)

	fetched, err = pool.GetOmapValues("obj", "", "", 4)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]byte{}, fetched)

	pool.Destroy()
}

func TestReadFilterOmap(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	pool_name := GetUUID()
	err := conn.MakePool(pool_name)
	assert.NoError(t, err)

	pool, err := conn.OpenIOContext(pool_name)
	assert.NoError(t, err)

	orig := map[string][]byte{
		"key1":          []byte("value1"),
		"prefixed-key3": []byte("value3"),
		"key2":          []byte("value2"),
	}

	err = pool.SetOmap("obj", orig)
	assert.NoError(t, err)

	// filter by prefix
	fetched, err := pool.GetOmapValues("obj", "", "prefixed", 4)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]byte{
		"prefixed-key3": []byte("value3"),
	}, fetched)

	// "start_after" a key
	fetched, err = pool.GetOmapValues("obj", "key1", "", 4)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]byte{
		"prefixed-key3": []byte("value3"),
		"key2":          []byte("value2"),
	}, fetched)

	// maxReturn
	fetched, err = pool.GetOmapValues("obj", "", "key", 1)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]byte{
		"key1": []byte("value1"),
	}, fetched)

	pool.Destroy()
}
