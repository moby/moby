// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package extensions

import "fmt"

// Config is parsed per-extension configuration delivered by id to Init or the
// out-of-process startup handshake.
type Config = map[string]any

// ValidateExtensionConfigKey reports whether key can key extension
// configuration.
// A key is either a full extension id such as org.example.myext.v1, or an
// extension name without a version, such as org.example.myext, which applies
// to every version of that extension.
func ValidateExtensionConfigKey(key ExtensionID) error {
	if ValidateExtensionID(key) != nil && !validExtensionName(string(key)) {
		return fmt.Errorf("invalid extension config key %q: want an extension id like org.example.myext.v1, or a name like org.example.myext for any version", key)
	}

	return nil
}

// LookupConfig returns the configuration for extension id from configs.
// An entry keyed by id takes precedence over one keyed by the extension name,
// which is id without its version.
func LookupConfig(configs map[ExtensionID]Config, id ExtensionID) Config {
	if config, ok := configs[id]; ok {
		return config
	}

	name, ok := cutExtensionVersion(id)
	if !ok {
		return nil
	}

	return configs[ExtensionID(name)]
}
