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
	"github.com/container-orchestrated-devices/container-device-interface/pkg/parser"
)

// QualifiedName returns the qualified name for a device.
// The syntax for a qualified device names is
//
//	"<vendor>/<class>=<name>".
//
// A valid vendor and class name may contain the following runes:
//
//	'A'-'Z', 'a'-'z', '0'-'9', '.', '-', '_'.
//
// A valid device name may contain the following runes:
//
//	'A'-'Z', 'a'-'z', '0'-'9', '-', '_', '.', ':'
//
// Deprecated: use parser.QualifiedName instead
func QualifiedName(vendor, class, name string) string {
	return parser.QualifiedName(vendor, class, name)
}

// IsQualifiedName tests if a device name is qualified.
//
// Deprecated: use parser.IsQualifiedName instead
func IsQualifiedName(device string) bool {
	return parser.IsQualifiedName(device)
}

// ParseQualifiedName splits a qualified name into device vendor, class,
// and name. If the device fails to parse as a qualified name, or if any
// of the split components fail to pass syntax validation, vendor and
// class are returned as empty, together with the verbatim input as the
// name and an error describing the reason for failure.
//
// Deprecated: use parser.ParseQualifiedName instead
func ParseQualifiedName(device string) (string, string, string, error) {
	return parser.ParseQualifiedName(device)
}

// ParseDevice tries to split a device name into vendor, class, and name.
// If this fails, for instance in the case of unqualified device names,
// ParseDevice returns an empty vendor and class together with name set
// to the verbatim input.
//
// Deprecated: use parser.ParseDevice instead
func ParseDevice(device string) (string, string, string) {
	return parser.ParseDevice(device)
}

// ParseQualifier splits a device qualifier into vendor and class.
// The syntax for a device qualifier is
//
//	"<vendor>/<class>"
//
// If parsing fails, an empty vendor and the class set to the
// verbatim input is returned.
//
// Deprecated: use parser.ParseQualifier instead
func ParseQualifier(kind string) (string, string) {
	return parser.ParseQualifier(kind)
}

// ValidateVendorName checks the validity of a vendor name.
// A vendor name may contain the following ASCII characters:
//   - upper- and lowercase letters ('A'-'Z', 'a'-'z')
//   - digits ('0'-'9')
//   - underscore, dash, and dot ('_', '-', and '.')
//
// Deprecated: use parser.ValidateVendorName instead
func ValidateVendorName(vendor string) error {
	return parser.ValidateVendorName(vendor)
}

// ValidateClassName checks the validity of class name.
// A class name may contain the following ASCII characters:
//   - upper- and lowercase letters ('A'-'Z', 'a'-'z')
//   - digits ('0'-'9')
//   - underscore, dash, and dot ('_', '-', and '.')
//
// Deprecated: use parser.ValidateClassName instead
func ValidateClassName(class string) error {
	return parser.ValidateClassName(class)
}

// ValidateDeviceName checks the validity of a device name.
// A device name may contain the following ASCII characters:
//   - upper- and lowercase letters ('A'-'Z', 'a'-'z')
//   - digits ('0'-'9')
//   - underscore, dash, dot, colon ('_', '-', '.', ':')
//
// Deprecated: use parser.ValidateDeviceName instead
func ValidateDeviceName(name string) error {
	return parser.ValidateDeviceName(name)
}
