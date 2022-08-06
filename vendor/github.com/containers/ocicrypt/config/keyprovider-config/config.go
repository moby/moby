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

package config

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
)

// Command describes the structure of command, it consist of path and args, where path defines the location of
// binary executable and args are passed on to the binary executable
type Command struct {
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`
}

// KeyProviderAttrs describes the structure of key provider, it defines the way of invocation to key provider
type KeyProviderAttrs struct {
	Command *Command `json:"cmd,omitempty"`
	Grpc    string   `json:"grpc,omitempty"`
}

// OcicryptConfig represents the format of an ocicrypt_provider.conf config file
type OcicryptConfig struct {
	KeyProviderConfig map[string]KeyProviderAttrs `json:"key-providers"`
}

const ENVVARNAME = "OCICRYPT_KEYPROVIDER_CONFIG"

// parseConfigFile parses a configuration file; it is not an error if the configuration file does
// not exist, so no error is returned.
func parseConfigFile(filename string) (*OcicryptConfig, error) {
	// a non-existent config file is not an error
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	ic := &OcicryptConfig{}
	err = json.Unmarshal(data, ic)
	return ic, err
}

// getConfiguration tries to read the configuration file at the following locations
// ${OCICRYPT_KEYPROVIDER_CONFIG} == "/etc/ocicrypt_keyprovider.yaml"
// If no configuration file could be found or read a null pointer is returned
func GetConfiguration() (*OcicryptConfig, error) {
	var ic *OcicryptConfig
	var err error
	filename := os.Getenv(ENVVARNAME)
	if len(filename) > 0 {
		ic, err = parseConfigFile(filename)
		if err != nil {
			return nil, errors.Wrap(err, "Error while parsing keyprovider config file")
		}
	} else {
		return nil, nil
	}
	return ic, nil
}
