/*
 * ZLint Copyright 2023 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package lint

// Global is what one would intuitive think of as being the global context of the configuration file.
// That is, given the following configuration...
//
// some_flag = true
// some_string = "the greatest song in the world"
//
// [e_some_lint]
// some_other_flag = false
//
// The fields `some_flag` and `some_string` will be targeted to land into this struct.
type Global struct{}

func (g Global) namespace() string {
	return "Global"
}

// RFC5280Config is the higher scoped configuration which services as the deserialization target for...
//
// [RFC5280Config]
// ...
// ...
type RFC5280Config struct{}

func (r RFC5280Config) namespace() string {
	return "RFC5280Config"
}

// RFC5480Config is the higher scoped configuration which services as the deserialization target for...
//
// [RFC5480Config]
// ...
// ...
type RFC5480Config struct{}

func (r RFC5480Config) namespace() string {
	return "RFC5480Config"
}

// RFC5891Config is the higher scoped configuration which services as the deserialization target for...
//
// [RFC5891Config]
// ...
// ...
type RFC5891Config struct{}

func (r RFC5891Config) namespace() string {
	return "RFC5891Config"
}

// CABFBaselineRequirementsConfig is the higher scoped configuration which services as the deserialization target for...
//
// [CABFBaselineRequirementsConfig]
// ...
// ...
type CABFBaselineRequirementsConfig struct{}

func (c CABFBaselineRequirementsConfig) namespace() string {
	return "CABFBaselineRequirementsConfig"
}

// CABFEVGuidelinesConfig is the higher scoped configuration which services as the deserialization target for...
//
// [CABFEVGuidelinesConfig]
// ...
// ...
type CABFEVGuidelinesConfig struct{}

func (c CABFEVGuidelinesConfig) namespace() string {
	return "CABFEVGuidelinesConfig"
}

// MozillaRootStorePolicyConfig is the higher scoped configuration which services as the deserialization target for...
//
// [MozillaRootStorePolicyConfig]
// ...
// ...
type MozillaRootStorePolicyConfig struct{}

func (m MozillaRootStorePolicyConfig) namespace() string {
	return "MozillaRootStorePolicyConfig"
}

// AppleRootStorePolicyConfig is the higher scoped configuration which services as the deserialization target for...
//
// [AppleRootStorePolicyConfig]
// ...
// ...
type AppleRootStorePolicyConfig struct{}

func (a AppleRootStorePolicyConfig) namespace() string {
	return "AppleRootStorePolicyConfig"
}

// CommunityConfig is the higher scoped configuration which services as the deserialization target for...
//
// [CommunityConfig]
// ...
// ...
type CommunityConfig struct{}

func (c CommunityConfig) namespace() string {
	return "CommunityConfig"
}

// EtsiEsiConfig is the higher scoped configuration which services as the deserialization target for...
//
// [EtsiEsiConfig]
// ...
// ...
type EtsiEsiConfig struct{}

func (e EtsiEsiConfig) namespace() string {
	return "EtsiEsiConfig"
}

// GlobalConfiguration acts both as an interface that can be used to obtain the TOML namespace of configuration
// as well as a way to mark a fielf in a struct as one of our own, higher scoped, configurations.
//
// the interface itself is public, however the singular `namespace` method is package private, meaning that
// normal lint struct cannot accidentally implement this.
type GlobalConfiguration interface {
	namespace() string
}

// defaultGlobals are used by other locations in the codebase that may want to iterate over all currently know
// global configuration types. Most notably, Registry.DefaultConfiguration uses it because it wants to print
// out a TOML document that is the full default configuration for ZLint.
var defaultGlobals = []GlobalConfiguration{
	&Global{},
	&CABFBaselineRequirementsConfig{},
	&RFC5280Config{},
	&RFC5480Config{},
	&RFC5891Config{},
	&CABFBaselineRequirementsConfig{},
	&CABFEVGuidelinesConfig{},
	&MozillaRootStorePolicyConfig{},
	&AppleRootStorePolicyConfig{},
	&CommunityConfig{},
}
