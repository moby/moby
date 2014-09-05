package libvirt

import (
	"testing"
	"time"
)

func buildTestStoragePool(poolName string) (VirStoragePool, VirConnection) {
	conn := buildTestConnection()
	var name string
	if poolName == "" {
		name = "default-pool-test-1"
	} else {
		name = poolName
	}
	pool, err := conn.StoragePoolDefineXML(`<pool type='dir'>
  <name>`+name+`</name>
  <target>
  <path>/default-pool</path>
  </target>
  </pool>`, 0)
	if err != nil {
		panic(err)
	}
	return pool, conn
}

func TestStoragePoolBuild(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if err := pool.Build(VIR_STORAGE_POOL_BUILD_NEW); err != nil {
		t.Fatal(err)
	}
}

func TestUndefineStoragePool(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Free()
		conn.CloseConnection()
	}()
	name, err := pool.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if err := pool.Undefine(); err != nil {
		t.Error(err)
		return
	}
	if _, err := conn.LookupStoragePoolByName(name); err == nil {
		t.Fatal("Shouldn't have been able to find storage pool")
		return
	}
}

func TestGetStoragePoolName(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if _, err := pool.GetName(); err != nil {
		t.Error(err)
	}
}

func TestGetStoragePoolUUID(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if _, err := pool.GetUUID(); err != nil {
		t.Error(err)
	}
}

func TestGetStoragePoolUUIDString(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if _, err := pool.GetUUIDString(); err != nil {
		t.Error(err)
	}
}

func TestGetStoragePoolInfo(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if _, err := pool.GetInfo(); err != nil {
		t.Error(err)
	}
}

func TestGetStoragePoolXMLDesc(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if _, err := pool.GetXMLDesc(0); err != nil {
		t.Error(err)
	}
}

func TestStoragePoolRefresh(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Destroy()
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	if err := pool.Refresh(0); err != nil {
		t.Error(err)
	}
}

func TestCreateDestroyStoragePool(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	state, err := pool.GetInfo()
	if err != nil {
		t.Error(err)
		return
	}
	if state.GetState() != VIR_STORAGE_POOL_RUNNING {
		t.Fatal("Storage pool should be running")
	}
	if err = pool.Destroy(); err != nil {
		t.Error(err)
		return
	}

	state, err = pool.GetInfo()
	if err != nil {
		t.Error(err)
		return
	}
	if state.GetState() != VIR_STORAGE_POOL_INACTIVE {
		t.Fatal("Storage pool should be inactive")
	}
}

func TestStoragePoolAutostart(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	as, err := pool.GetAutostart()
	if err != nil {
		t.Error(err)
		return
	}
	if as {
		t.Fatal("autostart should be false")
	}
	if err := pool.SetAutostart(true); err != nil {
		t.Error(err)
		return
	}
	as, err = pool.GetAutostart()
	if err != nil {
		t.Error(err)
		return
	}
	if !as {
		t.Fatal("autostart should be true")
	}
}

func TestStoragePoolIsActive(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()

	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	active, err := pool.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if !active {
		t.Fatal("Storage pool should be active")
	}
	if err := pool.Destroy(); err != nil {
		t.Error(err)
		return
	}
	active, err = pool.IsActive()
	if err != nil {
		t.Error(err)
		return
	}
	if active {
		t.Fatal("Storage pool should be inactive")
	}
}

func TestStorageVolCreateDelete(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	vol, err := pool.StorageVolCreateXML(testStorageVolXML("", "default-pool"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer vol.Free()
	if err := vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL); err != nil {
		t.Fatal(err)
	}
}

func TestLookupStorageVolByName(t *testing.T) {
	pool, conn := buildTestStoragePool("")
	defer func() {
		pool.Undefine()
		pool.Free()
		conn.CloseConnection()
	}()
	if err := pool.Create(0); err != nil {
		t.Error(err)
		return
	}
	defer pool.Destroy()
	defVolName := time.Now().String()
	vol, err := pool.StorageVolCreateXML(testStorageVolXML(defVolName, "default-pool"), 0)
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		vol.Delete(VIR_STORAGE_VOL_DELETE_NORMAL)
		vol.Free()
	}()
	vol, err = pool.LookupStorageVolByName(defVolName)
	if err != nil {
		t.Error(err)
		return
	}
	name, err := vol.GetName()
	if err != nil {
		t.Error(err)
		return
	}
	if name != defVolName {
		t.Fatalf("expected storage volume name: %s ,got: %s", defVolName, name)
	}
}
