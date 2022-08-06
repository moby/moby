/*
   Copyright The ocicrypt Authors.
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

package pkcs11

import (
	"fmt"
	"github.com/pkg/errors"
	pkcs11uri "github.com/stefanberger/go-pkcs11uri"
	"gopkg.in/yaml.v2"
)

// Pkcs11KeyFile describes the format of the pkcs11 (private) key file.
// It also carries pkcs11 module related environment variables that are transferred to the
// Pkcs11URI object and activated when the pkcs11 module is used.
type Pkcs11KeyFile struct {
	Pkcs11 struct {
		Uri string `yaml:"uri"`
	} `yaml:"pkcs11"`
	Module struct {
		Env map[string]string `yaml:"env,omitempty"`
	} `yaml:"module"`
}

// Pkcs11KeyFileObject is a representation of the Pkcs11KeyFile with the pkcs11 URI as an object
type Pkcs11KeyFileObject struct {
	Uri *pkcs11uri.Pkcs11URI
}

// ParsePkcs11Uri parses a pkcs11 URI
func ParsePkcs11Uri(uri string) (*pkcs11uri.Pkcs11URI, error) {
	p11uri := pkcs11uri.New()
	err := p11uri.Parse(uri)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not parse Pkcs11URI from file")
	}
	return p11uri, err
}

// ParsePkcs11KeyFile parses a pkcs11 key file holding a pkcs11 URI describing a private key.
// The file has the following yaml format:
// pkcs11:
//  - uri : <pkcs11 uri>
// An error is returned if the pkcs11 URI is malformed
func ParsePkcs11KeyFile(yamlstr []byte) (*Pkcs11KeyFileObject, error) {
	p11keyfile := Pkcs11KeyFile{}

	err := yaml.Unmarshal([]byte(yamlstr), &p11keyfile)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not unmarshal pkcs11 keyfile")
	}

	p11uri, err := ParsePkcs11Uri(p11keyfile.Pkcs11.Uri)
	if err != nil {
		return nil, err
	}
	p11uri.SetEnvMap(p11keyfile.Module.Env)

	return &Pkcs11KeyFileObject{Uri: p11uri}, err
}

// IsPkcs11PrivateKey checks whether the given YAML represents a Pkcs11 private key
func IsPkcs11PrivateKey(yamlstr []byte) bool {
	_, err := ParsePkcs11KeyFile(yamlstr)
	return err == nil
}

// IsPkcs11PublicKey checks whether the given YAML represents a Pkcs11 public key
func IsPkcs11PublicKey(yamlstr []byte) bool {
	_, err := ParsePkcs11KeyFile(yamlstr)
	return err == nil
}

// Pkcs11Config describes the layout of a pkcs11 config file
// The file has the following yaml format:
// module-directories:
// - /usr/lib64/pkcs11/
// allowd-module-paths
// - /usr/lib64/pkcs11/libsofthsm2.so
type Pkcs11Config struct {
	ModuleDirectories  []string `yaml:"module-directories"`
	AllowedModulePaths []string `yaml:"allowed-module-paths"`
}

// GetDefaultModuleDirectories returns module directories covering
// a variety of Linux distros
func GetDefaultModuleDirectories() []string {
	dirs := []string{
		"/usr/lib64/pkcs11/", // Fedora,RHEL,openSUSE
		"/usr/lib/pkcs11/",   // Fedora,ArchLinux
		"/usr/local/lib/pkcs11/",
		"/usr/lib/softhsm/", // Debian,Ubuntu
	}

	// Debian directory: /usr/lib/(x86_64|aarch64|arm|powerpc64le|s390x)-linux-gnu/
	hosttype, ostype, q := getHostAndOsType()
	if len(hosttype) > 0 {
		dir := fmt.Sprintf("/usr/lib/%s-%s-%s/", hosttype, ostype, q)
		dirs = append(dirs, dir)
	}
	return dirs
}

// GetDefaultModuleDirectoresFormatted returns the default module directories formatted for YAML
func GetDefaultModuleDirectoriesYaml(indent string) string {
	res := ""

	for _, dir := range GetDefaultModuleDirectories() {
		res += indent + "- " + dir + "\n"
	}
	return res
}

// ParsePkcs11ConfigFile parses a pkcs11 config file hat influences the module search behavior
// as well as the set of modules that users are allowed to use
func ParsePkcs11ConfigFile(yamlstr []byte) (*Pkcs11Config, error) {
	p11conf := Pkcs11Config{}

	err := yaml.Unmarshal([]byte(yamlstr), &p11conf)
	if err != nil {
		return &p11conf, errors.Wrapf(err, "Could not parse Pkcs11Config")
	}
	return &p11conf, nil
}
