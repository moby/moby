package drvregistry

import (
	"runtime"
	"sort"
	"testing"

	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/ipamapi"
	builtinIpam "github.com/docker/docker/libnetwork/ipams/builtin"
	nullIpam "github.com/docker/docker/libnetwork/ipams/null"
	remoteIpam "github.com/docker/docker/libnetwork/ipams/remote"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const mockDriverName = "mock-driver"

type mockDriver struct {
	driverapi.Driver
}

var mockDriverCaps = driverapi.Capability{DataScope: datastore.LocalScope}

var md = mockDriver{}

func mockDriverInit(reg driverapi.DriverCallback, opt map[string]interface{}) error {
	return reg.RegisterDriver(mockDriverName, &md, mockDriverCaps)
}

func (m *mockDriver) Type() string {
	return mockDriverName
}

func (m *mockDriver) IsBuiltIn() bool {
	return true
}

func getNew(t *testing.T) *DrvRegistry {
	reg, err := New(nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = initIPAMDrivers(reg)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func initIPAMDrivers(r *DrvRegistry) error {
	for _, fn := range [](func(ipamapi.Callback, interface{}, interface{}) error){
		builtinIpam.Init, //nolint:staticcheck
		remoteIpam.Init,  //nolint:staticcheck
		nullIpam.Init,    //nolint:staticcheck
	} {
		if err := fn(r, nil, nil); err != nil {
			return err
		}
	}

	return nil
}

func TestNew(t *testing.T) {
	getNew(t)
}

func TestAddDriver(t *testing.T) {
	reg := getNew(t)

	err := reg.AddDriver(mockDriverName, mockDriverInit, nil)
	assert.NilError(t, err)
}

func TestAddDuplicateDriver(t *testing.T) {
	reg := getNew(t)

	err := reg.AddDriver(mockDriverName, mockDriverInit, nil)
	assert.NilError(t, err)

	// Try adding the same driver
	err = reg.AddDriver(mockDriverName, mockDriverInit, nil)
	assert.Check(t, is.ErrorContains(err, ""))
}

func TestIPAMDefaultAddressSpaces(t *testing.T) {
	reg := getNew(t)

	as1, as2, err := reg.IPAMDefaultAddressSpaces("default")
	assert.NilError(t, err)
	assert.Check(t, as1 != "")
	assert.Check(t, as2 != "")
}

func TestDriver(t *testing.T) {
	reg := getNew(t)

	err := reg.AddDriver(mockDriverName, mockDriverInit, nil)
	assert.NilError(t, err)

	d, cap := reg.Driver(mockDriverName)
	assert.Check(t, d != nil)
	assert.Check(t, cap != nil)
}

func TestIPAM(t *testing.T) {
	reg := getNew(t)

	i, cap := reg.IPAM("default")
	assert.Check(t, i != nil)
	assert.Check(t, cap != nil)
}

func TestWalkIPAMs(t *testing.T) {
	reg := getNew(t)

	ipams := make([]string, 0, 2)
	reg.WalkIPAMs(func(name string, driver ipamapi.Ipam, cap *ipamapi.Capability) bool {
		ipams = append(ipams, name)
		return false
	})

	sort.Strings(ipams)
	expected := []string{"default", "null"}
	if runtime.GOOS == "windows" {
		expected = append(expected, "windows")
	}
	assert.Check(t, is.DeepEqual(ipams, expected))
}

func TestWalkDrivers(t *testing.T) {
	reg := getNew(t)

	err := reg.AddDriver(mockDriverName, mockDriverInit, nil)
	assert.NilError(t, err)

	var driverName string
	reg.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		driverName = name
		return false
	})

	assert.Check(t, is.Equal(driverName, mockDriverName))
}
