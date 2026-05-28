// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types020

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/types"
	convert "github.com/containernetworking/cni/pkg/types/internal"
)

const ImplementedSpecVersion string = "0.2.0"

var supportedVersions = []string{"", "0.1.0", ImplementedSpecVersion}

// Register converters for all versions less than the implemented spec version
func init() {
	convert.RegisterConverter("0.1.0", []string{ImplementedSpecVersion}, convertFrom010)
	convert.RegisterConverter(ImplementedSpecVersion, []string{"0.1.0"}, convertTo010)

	// Creator
	convert.RegisterCreator(supportedVersions, NewResult)
}

// Compatibility types for CNI version 0.1.0 and 0.2.0

// NewResult creates a new Result object from JSON data. The JSON data
// must be compatible with the CNI versions implemented by this type.
func NewResult(data []byte) (types.Result, error) {
	result := &Result{}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, err
	}
	for _, v := range supportedVersions {
		if result.CNIVersion == v {
			if result.CNIVersion == "" {
				result.CNIVersion = "0.1.0"
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("result type supports %v but unmarshalled CNIVersion is %q",
		supportedVersions, result.CNIVersion)
}

// GetResult converts the given Result object to the ImplementedSpecVersion
// and returns the concrete type or an error
func GetResult(r types.Result) (*Result, error) {
	result020, err := convert.Convert(r, ImplementedSpecVersion)
	if err != nil {
		return nil, err
	}
	result, ok := result020.(*Result)
	if !ok {
		return nil, fmt.Errorf("failed to convert result")
	}
	return result, nil
}

func convertFrom010(from types.Result, toVersion string) (types.Result, error) {
	if toVersion != "0.2.0" {
		panic("only converts to version 0.2.0")
	}
	fromResult := from.(*Result)
	return &Result{
		CNIVersion: ImplementedSpecVersion,
		IP4:        fromResult.IP4.Copy(),
		IP6:        fromResult.IP6.Copy(),
		DNS:        *fromResult.DNS.Copy(),
	}, nil
}

func convertTo010(from types.Result, toVersion string) (types.Result, error) {
	if toVersion != "0.1.0" {
		panic("only converts to version 0.1.0")
	}
	fromResult := from.(*Result)
	return &Result{
		CNIVersion: "0.1.0",
		IP4:        fromResult.IP4.Copy(),
		IP6:        fromResult.IP6.Copy(),
		DNS:        *fromResult.DNS.Copy(),
	}, nil
}

// Result is what gets returned from the plugin (via stdout) to the caller
type Result struct {
	CNIVersion string    `json:"cniVersion,omitempty"`
	IP4        *IPConfig `json:"ip4,omitempty"`
	IP6        *IPConfig `json:"ip6,omitempty"`
	DNS        types.DNS `json:"dns,omitempty"`
}

func (r *Result) Version() string {
	return r.CNIVersion
}

func (r *Result) GetAsVersion(version string) (types.Result, error) {
	// If the creator of the result did not set the CNIVersion, assume it
	// should be the highest spec version implemented by this Result
	if r.CNIVersion == "" {
		r.CNIVersion = ImplementedSpecVersion
	}
	return convert.Convert(r, version)
}

func (r *Result) Print() error {
	return r.PrintTo(os.Stdout)
}

func (r *Result) PrintTo(writer io.Writer) error {
	data, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

// IPConfig contains values necessary to configure an interface
type IPConfig struct {
	IP      net.IPNet
	Gateway net.IP
	Routes  []types.Route
}

func (i *IPConfig) Copy() *IPConfig {
	if i == nil {
		return nil
	}

	var routes []types.Route
	for _, fromRoute := range i.Routes {
		routes = append(routes, *fromRoute.Copy())
	}
	return &IPConfig{
		IP:      i.IP,
		Gateway: i.Gateway,
		Routes:  routes,
	}
}

// net.IPNet is not JSON (un)marshallable so this duality is needed
// for our custom IPNet type

// JSON (un)marshallable types
type ipConfig struct {
	IP      types.IPNet   `json:"ip"`
	Gateway net.IP        `json:"gateway,omitempty"`
	Routes  []types.Route `json:"routes,omitempty"`
}

func (c *IPConfig) MarshalJSON() ([]byte, error) {
	ipc := ipConfig{
		IP:      types.IPNet(c.IP),
		Gateway: c.Gateway,
		Routes:  c.Routes,
	}

	return json.Marshal(ipc)
}

func (c *IPConfig) UnmarshalJSON(data []byte) error {
	ipc := ipConfig{}
	if err := json.Unmarshal(data, &ipc); err != nil {
		return err
	}

	c.IP = net.IPNet(ipc.IP)
	c.Gateway = ipc.Gateway
	c.Routes = ipc.Routes
	return nil
}
