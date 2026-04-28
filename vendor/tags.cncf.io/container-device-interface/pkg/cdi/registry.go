/*
   Copyright © 2021 The CDI Authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cdi

import (
	"sync"

	oci "github.com/opencontainers/runtime-spec/specs-go"
	cdi "tags.cncf.io/container-device-interface/specs-go"
)

// Registry keeps a cache of all CDI Specs installed or generated on
// the host. Registry is the primary interface clients should use to
// interact with CDI.
//
// The most commonly used Registry functions are for refreshing the
// registry and injecting CDI devices into an OCI Spec.
//
// Deprecated: Registry is deprecated and will be removed in a future
// version. Please update your code to use the corresponding package-
// level functions Configure(), Refresh(), InjectDevices(), GetErrors(),
// and GetDefaultCache().
type Registry interface {
	RegistryResolver
	RegistryRefresher
	DeviceDB() RegistryDeviceDB
	SpecDB() RegistrySpecDB
}

// RegistryRefresher is the registry interface for refreshing the
// cache of CDI Specs and devices.
//
// Configure reconfigures the registry with the given options.
//
// Refresh rescans all CDI Spec directories and updates the
// state of the cache to reflect any changes. It returns any
// errors encountered during the refresh.
//
// GetErrors returns all errors encountered for any of the scanned
// Spec files during the last cache refresh.
//
// GetSpecDirectories returns the set up CDI Spec directories
// currently in use. The directories are returned in the scan
// order of Refresh().
//
// GetSpecDirErrors returns any errors related to the configured
// Spec directories.
//
// Deprecated: RegistryRefresher is deprecated and will be removed
// in a future version. Please use the default cache and its related
// package-level functions instead.
type RegistryRefresher interface {
	Configure(...Option) error
	Refresh() error
	GetErrors() map[string][]error
	GetSpecDirectories() []string
	GetSpecDirErrors() map[string]error
}

// RegistryResolver is the registry interface for injecting CDI
// devices into an OCI Spec.
//
// InjectDevices takes an OCI Spec and injects into it a set of
// CDI devices given by qualified name. It returns the names of
// any unresolved devices and an error if injection fails.
//
// Deprecated: RegistryRefresher is deprecated and will be removed
// in a future version. Please use the default cache and its related
// package-level functions instead.
type RegistryResolver interface {
	InjectDevices(spec *oci.Spec, device ...string) (unresolved []string, err error)
}

// RegistryDeviceDB is the registry interface for querying devices.
//
// GetDevice returns the CDI device for the given qualified name. If
// the device is not GetDevice returns nil.
//
// ListDevices returns a slice with the names of qualified device
// known. The returned slice is sorted.
//
// Deprecated: RegistryDeviceDB is deprecated and will be removed
// in a future version. Please use the default cache and its related
// package-level functions instead.
// and will be removed in a future version. Please use the default
// cache and its related package-level functions instead.
type RegistryDeviceDB interface {
	GetDevice(device string) *Device
	ListDevices() []string
}

// RegistrySpecDB is the registry interface for querying CDI Specs.
//
// ListVendors returns a slice with all vendors known. The returned
// slice is sorted.
//
// ListClasses returns a slice with all classes known. The returned
// slice is sorted.
//
// GetVendorSpecs returns a slice of all Specs for the vendor.
//
// GetSpecErrors returns any errors for the Spec encountered during
// the last cache refresh.
//
// WriteSpec writes the Spec with the given content and name to the
// last Spec directory.
//
// Deprecated: RegistrySpecDB is deprecated and will be removed
// in a future version. Please use the default cache and its related
// package-level functions instead.
type RegistrySpecDB interface {
	ListVendors() []string
	ListClasses() []string
	GetVendorSpecs(vendor string) []*Spec
	GetSpecErrors(*Spec) []error
	WriteSpec(raw *cdi.Spec, name string) error
	RemoveSpec(name string) error
}

type registry struct {
	*Cache
}

var _ Registry = &registry{}

var (
	reg      *registry
	initOnce sync.Once
)

// GetRegistry returns the CDI registry. If any options are given, those
// are applied to the registry.
//
// Deprecated: GetRegistry is deprecated and will be removed in a future
// version. Please use the default cache and its related package-level
// functions instead.
func GetRegistry(options ...Option) Registry {
	initOnce.Do(func() {
		reg = &registry{GetDefaultCache()}
	})
	if len(options) > 0 {
		// We don't care about errors here
		_ = reg.Configure(options...)
	}
	return reg
}

// DeviceDB returns the registry interface for querying devices.
//
// Deprecated: DeviceDB is deprecated and will be removed in a future
// version. Please use the default cache and its related package-level
// functions instead.
func (r *registry) DeviceDB() RegistryDeviceDB {
	return r
}

// SpecDB returns the registry interface for querying Specs.
//
// Deprecated: SpecDB is deprecated and will be removed in a future
// version. Please use the default cache and its related package-level
// functions instead.
func (r *registry) SpecDB() RegistrySpecDB {
	return r
}
