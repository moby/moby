package drvregistry

import (
	"testing"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/scope"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const mockDriverName = "mock-driver"

type mockDriver struct {
	driverapi.Driver
}

var mockDriverCaps = driverapi.Capability{DataScope: scope.Local}

var md = mockDriver{}

func (m *mockDriver) Type() string {
	return mockDriverName
}

func (m *mockDriver) IsBuiltIn() bool {
	return true
}

func TestNetworks(t *testing.T) {
	t.Run("RegisterDriver", func(t *testing.T) {
		var reg Networks
		err := reg.RegisterDriver(mockDriverName, &md, mockDriverCaps)
		assert.NilError(t, err)
	})

	t.Run("RegisterDuplicateDriver", func(t *testing.T) {
		var reg Networks
		err := reg.RegisterDriver(mockDriverName, &md, mockDriverCaps)
		assert.NilError(t, err)

		// Try adding the same driver
		err = reg.RegisterDriver(mockDriverName, &md, mockDriverCaps)
		assert.Check(t, is.ErrorContains(err, ""))
	})

	t.Run("Driver", func(t *testing.T) {
		var reg Networks
		err := reg.RegisterDriver(mockDriverName, &md, mockDriverCaps)
		assert.NilError(t, err)

		d, cap := reg.Driver(mockDriverName)
		assert.Check(t, d != nil)
		assert.Check(t, is.DeepEqual(cap, mockDriverCaps))
	})

	t.Run("WalkDrivers", func(t *testing.T) {
		var reg Networks
		err := reg.RegisterDriver(mockDriverName, &md, mockDriverCaps)
		assert.NilError(t, err)

		var driverName string
		reg.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
			driverName = name
			return false
		})

		assert.Check(t, is.Equal(driverName, mockDriverName))
	})
}
