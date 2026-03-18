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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	oci "github.com/opencontainers/runtime-spec/specs-go"
	orderedyaml "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"

	"tags.cncf.io/container-device-interface/internal/validation"
	"tags.cncf.io/container-device-interface/pkg/parser"
	cdi "tags.cncf.io/container-device-interface/specs-go"
)

const (
	// defaultSpecExt is the file extension for the default encoding.
	defaultSpecExt = ".yaml"
)

type validator interface {
	Validate(*cdi.Spec) error
}

var (
	// Externally set CDI Spec validation function.
	specValidator validator
	validatorLock sync.RWMutex
)

// Spec represents a single CDI Spec. It is usually loaded from a
// file and stored in a cache. The Spec has an associated priority.
// This priority is inherited from the associated priority of the
// CDI Spec directory that contains the CDI Spec file and is used
// to resolve conflicts if multiple CDI Spec files contain entries
// for the same fully qualified device.
type Spec struct {
	*cdi.Spec
	vendor   string
	class    string
	path     string
	priority int
	devices  map[string]*Device
}

// ReadSpec reads the given CDI Spec file. The resulting Spec is
// assigned the given priority. If reading or parsing the Spec
// data fails ReadSpec returns a nil Spec and an error.
func ReadSpec(path string, priority int) (*Spec, error) {
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return nil, err
	case err != nil:
		return nil, fmt.Errorf("failed to read CDI Spec %q: %w", path, err)
	}

	raw, err := ParseSpec(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CDI Spec %q: %w", path, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("failed to parse CDI Spec %q, no Spec data", path)
	}

	spec, err := newSpec(raw, path, priority)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

// newSpec creates a new Spec from the given CDI Spec data. The
// Spec is marked as loaded from the given path with the given
// priority. If Spec data validation fails newSpec returns a nil
// Spec and an error.
func newSpec(raw *cdi.Spec, path string, priority int) (*Spec, error) {
	err := validateSpec(raw)
	if err != nil {
		return nil, err
	}

	spec := &Spec{
		Spec:     raw,
		path:     filepath.Clean(path),
		priority: priority,
	}

	if ext := filepath.Ext(spec.path); ext != ".yaml" && ext != ".json" {
		spec.path += defaultSpecExt
	}

	spec.vendor, spec.class = parser.ParseQualifier(spec.Kind)

	if spec.devices, err = spec.validate(); err != nil {
		return nil, fmt.Errorf("invalid CDI Spec: %w", err)
	}

	return spec, nil
}

// Write the CDI Spec to the file associated with it during instantiation
// by newSpec() or ReadSpec().
func (s *Spec) write(overwrite bool) error {
	var (
		data []byte
		dir  string
		tmp  *os.File
		err  error
	)

	err = validateSpec(s.Spec)
	if err != nil {
		return err
	}

	if filepath.Ext(s.path) == ".yaml" {
		data, err = orderedyaml.Marshal(s.Spec)
		data = append([]byte("---\n"), data...)
	} else {
		data, err = json.Marshal(s.Spec)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal Spec file: %w", err)
	}

	dir = filepath.Dir(s.path)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create Spec dir: %w", err)
	}

	tmp, err = os.CreateTemp(dir, "spec.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create Spec file: %w", err)
	}
	_, err = tmp.Write(data)
	_ = tmp.Close()
	if err != nil {
		return fmt.Errorf("failed to write Spec file: %w", err)
	}

	err = renameIn(dir, filepath.Base(tmp.Name()), filepath.Base(s.path), overwrite)

	if err != nil {
		_ = os.Remove(tmp.Name())
		err = fmt.Errorf("failed to write Spec file: %w", err)
	}

	return err
}

// GetVendor returns the vendor of this Spec.
func (s *Spec) GetVendor() string {
	return s.vendor
}

// GetClass returns the device class of this Spec.
func (s *Spec) GetClass() string {
	return s.class
}

// GetDevice returns the device for the given unqualified name.
func (s *Spec) GetDevice(name string) *Device {
	return s.devices[name]
}

// GetPath returns the filesystem path of this Spec.
func (s *Spec) GetPath() string {
	return s.path
}

// GetPriority returns the priority of this Spec.
func (s *Spec) GetPriority() int {
	return s.priority
}

// ApplyEdits applies the Spec's global-scope container edits to an OCI Spec.
func (s *Spec) ApplyEdits(ociSpec *oci.Spec) error {
	return s.edits().Apply(ociSpec)
}

// edits returns the applicable global container edits for this spec.
func (s *Spec) edits() *ContainerEdits {
	return &ContainerEdits{&s.ContainerEdits}
}

// MinimumRequiredVersion determines the minimum spec version for the input spec.
// Deprecated: use cdi.MinimumRequiredVersion instead
func MinimumRequiredVersion(spec *cdi.Spec) (string, error) {
	return cdi.MinimumRequiredVersion(spec)
}

// Validate the Spec.
func (s *Spec) validate() (map[string]*Device, error) {
	if err := cdi.ValidateVersion(s.Spec); err != nil {
		return nil, err
	}
	if err := parser.ValidateVendorName(s.vendor); err != nil {
		return nil, err
	}
	if err := parser.ValidateClassName(s.class); err != nil {
		return nil, err
	}
	if err := validation.ValidateSpecAnnotations(s.Kind, s.Annotations); err != nil {
		return nil, err
	}
	if err := s.edits().Validate(); err != nil {
		return nil, err
	}

	devices := make(map[string]*Device)
	for _, d := range s.Devices {
		dev, err := newDevice(s, d)
		if err != nil {
			return nil, fmt.Errorf("failed add device %q: %w", d.Name, err)
		}
		if _, conflict := devices[d.Name]; conflict {
			return nil, fmt.Errorf("invalid spec, multiple device %q", d.Name)
		}
		devices[d.Name] = dev
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("invalid spec, no devices")
	}

	return devices, nil
}

// ParseSpec parses CDI Spec data into a raw CDI Spec.
func ParseSpec(data []byte) (*cdi.Spec, error) {
	var raw *cdi.Spec
	err := yaml.UnmarshalStrict(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal CDI Spec: %w", err)
	}
	return raw, nil
}

// SetSpecValidator sets a CDI Spec validator function. This function
// is used for extra CDI Spec content validation whenever a Spec file
// loaded (using ReadSpec() or written (using WriteSpec()).
func SetSpecValidator(v validator) {
	validatorLock.Lock()
	defer validatorLock.Unlock()
	specValidator = v
}

// validateSpec validates the Spec using the external validator.
func validateSpec(raw *cdi.Spec) error {
	validatorLock.RLock()
	defer validatorLock.RUnlock()

	if specValidator == nil {
		return nil
	}
	err := specValidator.Validate(raw)
	if err != nil {
		return fmt.Errorf("Spec validation failed: %w", err)
	}
	return nil
}

// GenerateSpecName generates a vendor+class scoped Spec file name. The
// name can be passed to WriteSpec() to write a Spec file to the file
// system.
//
// vendor and class should match the vendor and class of the CDI Spec.
// The file name is generated without a ".json" or ".yaml" extension.
// The caller can append the desired extension to choose a particular
// encoding. Otherwise WriteSpec() will use its default encoding.
//
// This function always returns the same name for the same vendor/class
// combination. Therefore it cannot be used as such to generate multiple
// Spec file names for a single vendor and class.
func GenerateSpecName(vendor, class string) string {
	return vendor + "-" + class
}

// GenerateTransientSpecName generates a vendor+class scoped transient
// Spec file name. The name can be passed to WriteSpec() to write a Spec
// file to the file system.
//
// Transient Specs are those whose lifecycle is tied to that of some
// external entity, for instance a container. vendor and class should
// match the vendor and class of the CDI Spec. transientID should be
// unique among all CDI users on the same host that might generate
// transient Spec files using the same vendor/class combination. If
// the external entity to which the lifecycle of the transient Spec
// is tied to has a unique ID of its own, then this is usually a
// good choice for transientID.
//
// The file name is generated without a ".json" or ".yaml" extension.
// The caller can append the desired extension to choose a particular
// encoding. Otherwise WriteSpec() will use its default encoding.
func GenerateTransientSpecName(vendor, class, transientID string) string {
	transientID = strings.ReplaceAll(transientID, "/", "_")
	return GenerateSpecName(vendor, class) + "_" + transientID
}

// GenerateNameForSpec generates a name for the given Spec using
// GenerateSpecName with the vendor and class taken from the Spec.
// On success it returns the generated name and a nil error. If
// the Spec does not contain a valid vendor or class, it returns
// an empty name and a non-nil error.
func GenerateNameForSpec(raw *cdi.Spec) (string, error) {
	vendor, class := parser.ParseQualifier(raw.Kind)
	if vendor == "" {
		return "", fmt.Errorf("invalid vendor/class %q in Spec", raw.Kind)
	}

	return GenerateSpecName(vendor, class), nil
}

// GenerateNameForTransientSpec generates a name for the given transient
// Spec using GenerateTransientSpecName with the vendor and class taken
// from the Spec. On success it returns the generated name and a nil error.
// If the Spec does not contain a valid vendor or class, it returns an
// an empty name and a non-nil error.
func GenerateNameForTransientSpec(raw *cdi.Spec, transientID string) (string, error) {
	vendor, class := parser.ParseQualifier(raw.Kind)
	if vendor == "" {
		return "", fmt.Errorf("invalid vendor/class %q in Spec", raw.Kind)
	}

	return GenerateTransientSpecName(vendor, class, transientID), nil
}
