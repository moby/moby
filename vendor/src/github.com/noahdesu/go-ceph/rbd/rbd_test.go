package rbd_test

import (
	"bytes"
	"encoding/json"
	"github.com/noahdesu/go-ceph/rados"
	"github.com/noahdesu/go-ceph/rbd"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"sort"
	"testing"
)

func GetUUID() string {
	out, _ := exec.Command("uuidgen").Output()
	return string(out[:36])
}

func TestVersion(t *testing.T) {
	var major, minor, patch = rbd.Version()
	assert.False(t, major < 0 || major > 1000, "invalid major")
	assert.False(t, minor < 0 || minor > 1000, "invalid minor")
	assert.False(t, patch < 0 || patch > 1000, "invalid patch")
}

func TestGetImageNames(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	ioctx, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	createdList := []string{}
	for i := 0; i < 10; i++ {
		name := GetUUID()
		_, err := rbd.Create(ioctx, name, 1<<22)
		assert.NoError(t, err)
		createdList = append(createdList, name)
	}

	imageNames, err := rbd.GetImageNames(ioctx)
	assert.NoError(t, err)

	sort.Strings(createdList)
	sort.Strings(imageNames)
	assert.Equal(t, createdList, imageNames)

	for _, name := range createdList {
		img := rbd.GetImage(ioctx, name)
		err := img.Remove()
		assert.NoError(t, err)
	}

	ioctx.Destroy()
	conn.DeletePool(poolname)
	conn.Shutdown()
}

func TestIOReaderWriter(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	ioctx, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	name := GetUUID()
	img, err := rbd.Create(ioctx, name, 1<<22)
	assert.NoError(t, err)

	err = img.Open()
	assert.NoError(t, err)

	stats, err := img.Stat()
	assert.NoError(t, err)

	encoder := json.NewEncoder(img)
	encoder.Encode(stats)

	err = img.Flush()
	assert.NoError(t, err)

	_, err = img.Seek(0, 0)
	assert.NoError(t, err)

	var stats2 *rbd.ImageInfo
	decoder := json.NewDecoder(img)
	decoder.Decode(&stats2)

	assert.Equal(t, &stats, &stats2)

	_, err = img.Seek(0, 0)
	bytes_in := []byte("input data")
	_, err = img.Write(bytes_in)
	assert.NoError(t, err)

	_, err = img.Seek(0, 0)
	assert.NoError(t, err)

	bytes_out := make([]byte, len(bytes_in))
	n_out, err := img.Read(bytes_out)

	assert.Equal(t, n_out, len(bytes_in))
	assert.Equal(t, bytes_in, bytes_out)

	err = img.Close()
	assert.NoError(t, err)

	img.Remove()
	assert.NoError(t, err)

	ioctx.Destroy()
	conn.DeletePool(poolname)
	conn.Shutdown()
}

func TestCreateSnapshot(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	ioctx, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	name := GetUUID()
	img, err := rbd.Create(ioctx, name, 1<<22)
	assert.NoError(t, err)

	err = img.Open()
	assert.NoError(t, err)

	snapshot, err := img.CreateSnapshot("mysnap")
	assert.NoError(t, err)

	err = img.Close()
	err = img.Open("mysnap")
	assert.NoError(t, err)

	snapshot.Remove()
	assert.NoError(t, err)

	err = img.Close()
	assert.NoError(t, err)

	img.Remove()
	assert.NoError(t, err)

	ioctx.Destroy()
	conn.DeletePool(poolname)
	conn.Shutdown()
}

func TestParentInfo(t *testing.T) {
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	conn.Connect()

	poolname := GetUUID()
	err := conn.MakePool(poolname)
	assert.NoError(t, err)

	ioctx, err := conn.OpenIOContext(poolname)
	assert.NoError(t, err)

	name := "parent"
	img, err := rbd.Create(ioctx, name, 1<<22, 1)
	assert.NoError(t, err)

	err = img.Open()
	assert.NoError(t, err)

	snapshot, err := img.CreateSnapshot("mysnap")
	assert.NoError(t, err)

	err = snapshot.Protect()
	assert.NoError(t, err)

	imgNew, err := img.Clone("mysnap", ioctx, "child", 1)
	assert.NoError(t, err)

	err = imgNew.Open()
	assert.NoError(t, err)
	parentPool := make([]byte, 128)
	parentName := make([]byte, 128)
	parentSnapname := make([]byte, 128)

	err = imgNew.GetParentInfo(parentPool, parentName, parentSnapname)
	assert.NoError(t, err)

	n := bytes.Index(parentName, []byte{0})
	pName := string(parentName[:n])

	n = bytes.Index(parentSnapname, []byte{0})
	pSnapname := string(parentSnapname[:n])
	assert.Equal(t, pName, "parent", "they should be equal")
	assert.Equal(t, pSnapname, "mysnap", "they should be equal")

	err = imgNew.Close()
	assert.NoError(t, err)

	err = imgNew.Remove()
	assert.NoError(t, err)

	err = snapshot.Unprotect()
	assert.NoError(t, err)

	err = snapshot.Remove()
	assert.NoError(t, err)

	err = img.Close()
	assert.NoError(t, err)

	err = img.Remove()
	assert.NoError(t, err)

	ioctx.Destroy()
	conn.DeletePool(poolname)
	conn.Shutdown()
}
