// Package cdi has the primary purpose of providing an API for
// interacting with CDI and consuming CDI devices.
//
// For more information about Container Device Interface, please refer to
// https://tags.cncf.io/container-device-interface
//
// # Container Device Interface
//
// Container Device Interface, or CDI for short, provides comprehensive
// third party device support for container runtimes. CDI uses vendor
// provided specification files, CDI Specs for short, to describe how a
// container's runtime environment should be modified when one or more
// of the vendor-specific devices is injected into the container. Beyond
// describing the low level platform-specific details of how to gain
// basic access to a device, CDI Specs allow more fine-grained device
// initialization, and the automatic injection of any necessary vendor-
// or device-specific software that might be required for a container
// to use a device or take full advantage of it.
//
// In the CDI device model containers request access to a device using
// fully qualified device names, qualified names for short, consisting of
// a vendor identifier, a device class and a device name or identifier.
// These pieces of information together uniquely identify a device among
// all device vendors, classes and device instances.
//
// This package implements an API for easy consumption of CDI. The API
// implements discovery, loading and caching of CDI Specs and injection
// of CDI devices into containers. This is the most common functionality
// the vast majority of CDI consumers need. The API should be usable both
// by OCI runtime clients and runtime implementations.
//
// # Default CDI Cache
//
// There is a default CDI cache instance which is always implicitly
// available and instantiated the first time it is referenced directly
// or indirectly. The most frequently used cache functions are available
// as identically named package level functions which operate on the
// default cache instance. Moreover, the registry also operates on the
// same default cache. We plan to deprecate the registry and eventually
// remove it in a future release.
//
// # CDI Registry
//
// Note: the Registry and its related interfaces are deprecated and will
// be removed in a future version. Please use the default cache and its
// related package-level function instead.
//
// The primary interface to interact with CDI devices is the Registry. It
// is essentially a cache of all Specs and devices discovered in standard
// CDI directories on the host. The registry has two main functionality,
// injecting devices into an OCI Spec and refreshing the cache of CDI
// Specs and devices.
//
// # Device Injection
//
// Using the Registry one can inject CDI devices into a container with code
// similar to the following snippet:
//
//	import (
//	    "fmt"
//	    "strings"
//
//	    log "github.com/sirupsen/logrus"
//
//	    "tags.cncf.io/container-device-interface/pkg/cdi"
//	    oci "github.com/opencontainers/runtime-spec/specs-go"
//	)
//
//	func injectCDIDevices(spec *oci.Spec, devices []string) error {
//	    log.Debug("pristine OCI Spec: %s", dumpSpec(spec))
//
//	    unresolved, err := cdi.GetRegistry().InjectDevices(spec, devices)
//	    if err != nil {
//	        return fmt.Errorf("CDI device injection failed: %w", err)
//	    }
//
//	    log.Debug("CDI-updated OCI Spec: %s", dumpSpec(spec))
//	    return nil
//	}
//
// # Cache Refresh
//
// By default the CDI Spec cache monitors the configured Spec directories
// and automatically refreshes itself when necessary. This behavior can be
// disabled using the WithAutoRefresh(false) option.
//
// Failure to set up monitoring for a Spec directory causes the directory to
// get ignored and an error to be recorded among the Spec directory errors.
// These errors can be queried using the GetSpecDirErrors() function. If the
// error condition is transient, for instance a missing directory which later
// gets created, the corresponding error will be removed once the condition
// is over.
//
// With auto-refresh enabled injecting any CDI devices can be done without
// an explicit call to Refresh(), using a code snippet similar to the
// following:
//
// In a runtime implementation one typically wants to make sure the
// CDI Spec cache is up to date before performing device injection.
// A code snippet similar to the following accmplishes that:
//
//	import (
//	    "fmt"
//	    "strings"
//
//	    log "github.com/sirupsen/logrus"
//
//	    "tags.cncf.io/container-device-interface/pkg/cdi"
//	    oci "github.com/opencontainers/runtime-spec/specs-go"
//	)
//
//	func injectCDIDevices(spec *oci.Spec, devices []string) error {
//	    registry := cdi.GetRegistry()
//
//	    if err := registry.Refresh(); err != nil {
//	        // Note:
//	        //   It is up to the implementation to decide whether
//	        //   to abort injection on errors. A failed Refresh()
//	        //   does not necessarily render the registry unusable.
//	        //   For instance, a parse error in a Spec file for
//	        //   vendor A does not have any effect on devices of
//	        //   vendor B...
//	        log.Warnf("pre-injection Refresh() failed: %v", err)
//	    }
//
//	    log.Debug("pristine OCI Spec: %s", dumpSpec(spec))
//
//	    unresolved, err := registry.InjectDevices(spec, devices)
//	    if err != nil {
//	        return fmt.Errorf("CDI device injection failed: %w", err)
//	    }
//
//	    log.Debug("CDI-updated OCI Spec: %s", dumpSpec(spec))
//	    return nil
//	}
//
// # Generated Spec Files, Multiple Directories, Device Precedence
//
// It is often necessary to generate Spec files dynamically. On some
// systems the available or usable set of CDI devices might change
// dynamically which then needs to be reflected in CDI Specs. For
// some device classes it makes sense to enumerate the available
// devices at every boot and generate Spec file entries for each
// device found. Some CDI devices might need special client- or
// request-specific configuration which can only be fulfilled by
// dynamically generated client-specific entries in transient Spec
// files.
//
// CDI can collect Spec files from multiple directories. Spec files are
// automatically assigned priorities according to which directory they
// were loaded from. The later a directory occurs in the list of CDI
// directories to scan, the higher priority Spec files loaded from that
// directory are assigned to. When two or more Spec files define the
// same device, conflict is resolved by choosing the definition from the
// Spec file with the highest priority.
//
// The default CDI directory configuration is chosen to encourage
// separating dynamically generated CDI Spec files from static ones.
// The default directories are '/etc/cdi' and '/var/run/cdi'. By putting
// dynamically generated Spec files under '/var/run/cdi', those take
// precedence over static ones in '/etc/cdi'. With this scheme, static
// Spec files, typically installed by distro-specific packages, go into
// '/etc/cdi' while all the dynamically generated Spec files, transient
// or other, go into '/var/run/cdi'.
//
// # Spec File Generation
//
// CDI offers two functions for writing and removing dynamically generated
// Specs from CDI Spec directories. These functions, WriteSpec() and
// RemoveSpec() implicitly follow the principle of separating dynamic Specs
// from the rest and therefore always write to and remove Specs from the
// last configured directory.
//
// Corresponding functions are also provided for generating names for Spec
// files. These functions follow a simple naming convention to ensure that
// multiple entities generating Spec files simultaneously on the same host
// do not end up using conflicting Spec file names. GenerateSpecName(),
// GenerateNameForSpec(), GenerateTransientSpecName(), and
// GenerateTransientNameForSpec() all generate names which can be passed
// as such to WriteSpec() and subsequently to RemoveSpec().
//
// Generating a Spec file for a vendor/device class can be done with a
// code snippet similar to the following:
//
// import (
//
//	"fmt"
//	...
//	"tags.cncf.io/container-device-interface/specs-go"
//	"tags.cncf.io/container-device-interface/pkg/cdi"
//
// )
//
//	func generateDeviceSpecs() error {
//	    registry := cdi.GetRegistry()
//	    spec := &specs.Spec{
//	        Version: specs.CurrentVersion,
//	        Kind:    vendor+"/"+class,
//	    }
//
//	    for _, dev := range enumerateDevices() {
//	        spec.Devices = append(spec.Devices, specs.Device{
//	            Name: dev.Name,
//	            ContainerEdits: getContainerEditsForDevice(dev),
//	        })
//	    }
//
//	    specName, err := cdi.GenerateNameForSpec(spec)
//	    if err != nil {
//	        return fmt.Errorf("failed to generate Spec name: %w", err)
//	    }
//
//	    return registry.SpecDB().WriteSpec(spec, specName)
//	}
//
// Similarly, generating and later cleaning up transient Spec files can be
// done with code fragments similar to the following. These transient Spec
// files are temporary Spec files with container-specific parametrization.
// They are typically created before the associated container is created
// and removed once that container is removed.
//
// import (
//
//	"fmt"
//	...
//	"tags.cncf.io/container-device-interface/specs-go"
//	"tags.cncf.io/container-device-interface/pkg/cdi"
//
// )
//
//	func generateTransientSpec(ctr Container) error {
//	    registry := cdi.GetRegistry()
//	    devices := getContainerDevs(ctr, vendor, class)
//	    spec := &specs.Spec{
//	        Version: specs.CurrentVersion,
//	        Kind:    vendor+"/"+class,
//	    }
//
//	    for _, dev := range devices {
//	        spec.Devices = append(spec.Devices, specs.Device{
//	            // the generated name needs to be unique within the
//	            // vendor/class domain on the host/node.
//	            Name: generateUniqueDevName(dev, ctr),
//	            ContainerEdits: getEditsForContainer(dev),
//	        })
//	    }
//
//	    // transientID is expected to guarantee that the Spec file name
//	    // generated using <vendor, class, transientID> is unique within
//	    // the host/node. If more than one device is allocated with the
//	    // same vendor/class domain, either all generated Spec entries
//	    // should go to a single Spec file (like in this sample snippet),
//	    // or transientID should be unique for each generated Spec file.
//	    transientID := getSomeSufficientlyUniqueIDForContainer(ctr)
//	    specName, err := cdi.GenerateNameForTransientSpec(vendor, class, transientID)
//	    if err != nil {
//	        return fmt.Errorf("failed to generate Spec name: %w", err)
//	    }
//
//	    return registry.SpecDB().WriteSpec(spec, specName)
//	}
//
//	func removeTransientSpec(ctr Container) error {
//	    registry := cdi.GetRegistry()
//	    transientID := getSomeSufficientlyUniqueIDForContainer(ctr)
//	    specName := cdi.GenerateNameForTransientSpec(vendor, class, transientID)
//
//	    return registry.SpecDB().RemoveSpec(specName)
//	}
//
// # CDI Spec Validation
//
// This package performs both syntactic and semantic validation of CDI
// Spec file data when a Spec file is loaded via the registry or using
// the ReadSpec API function. As part of the semantic verification, the
// Spec file is verified against the CDI Spec JSON validation schema.
//
// If a valid externally provided JSON validation schema is found in
// the filesystem at /etc/cdi/schema/schema.json it is loaded and used
// as the default validation schema. If such a file is not found or
// fails to load, an embedded no-op schema is used.
//
// The used validation schema can also be changed programmatically using
// the SetSchema API convenience function. This function also accepts
// the special "builtin" (BuiltinSchemaName) and "none" (NoneSchemaName)
// schema names which switch the used schema to the in-repo validation
// schema embedded into the binary or the now default no-op schema
// correspondingly. Other names are interpreted as the path to the actual
// validation schema to load and use.
package cdi
