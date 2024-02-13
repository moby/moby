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

package types100

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/types"
	types040 "github.com/containernetworking/cni/pkg/types/040"
	convert "github.com/containernetworking/cni/pkg/types/internal"
)

const ImplementedSpecVersion string = "1.0.0"

var supportedVersions = []string{ImplementedSpecVersion}

// Register converters for all versions less than the implemented spec version
func init() {
	// Up-converters
	convert.RegisterConverter("0.1.0", supportedVersions, convertFrom02x)
	convert.RegisterConverter("0.2.0", supportedVersions, convertFrom02x)
	convert.RegisterConverter("0.3.0", supportedVersions, convertFrom04x)
	convert.RegisterConverter("0.3.1", supportedVersions, convertFrom04x)
	convert.RegisterConverter("0.4.0", supportedVersions, convertFrom04x)

	// Down-converters
	convert.RegisterConverter("1.0.0", []string{"0.3.0", "0.3.1", "0.4.0"}, convertTo04x)
	convert.RegisterConverter("1.0.0", []string{"0.1.0", "0.2.0"}, convertTo02x)

	// Creator
	convert.RegisterCreator(supportedVersions, NewResult)
}

func NewResult(data []byte) (types.Result, error) {
	result := &Result{}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, err
	}
	for _, v := range supportedVersions {
		if result.CNIVersion == v {
			return result, nil
		}
	}
	return nil, fmt.Errorf("result type supports %v but unmarshalled CNIVersion is %q",
		supportedVersions, result.CNIVersion)
}

func GetResult(r types.Result) (*Result, error) {
	resultCurrent, err := r.GetAsVersion(ImplementedSpecVersion)
	if err != nil {
		return nil, err
	}
	result, ok := resultCurrent.(*Result)
	if !ok {
		return nil, fmt.Errorf("failed to convert result")
	}
	return result, nil
}

func NewResultFromResult(result types.Result) (*Result, error) {
	newResult, err := convert.Convert(result, ImplementedSpecVersion)
	if err != nil {
		return nil, err
	}
	return newResult.(*Result), nil
}

// Result is what gets returned from the plugin (via stdout) to the caller
type Result struct {
	CNIVersion string         `json:"cniVersion,omitempty"`
	Interfaces []*Interface   `json:"interfaces,omitempty"`
	IPs        []*IPConfig    `json:"ips,omitempty"`
	Routes     []*types.Route `json:"routes,omitempty"`
	DNS        types.DNS      `json:"dns,omitempty"`
}

func convertFrom02x(from types.Result, toVersion string) (types.Result, error) {
	result040, err := convert.Convert(from, "0.4.0")
	if err != nil {
		return nil, err
	}
	result100, err := convertFrom04x(result040, ImplementedSpecVersion)
	if err != nil {
		return nil, err
	}
	return result100, nil
}

func convertIPConfigFrom040(from *types040.IPConfig) *IPConfig {
	to := &IPConfig{
		Address: from.Address,
		Gateway: from.Gateway,
	}
	if from.Interface != nil {
		intf := *from.Interface
		to.Interface = &intf
	}
	return to
}

func convertInterfaceFrom040(from *types040.Interface) *Interface {
	return &Interface{
		Name:    from.Name,
		Mac:     from.Mac,
		Sandbox: from.Sandbox,
	}
}

func convertFrom04x(from types.Result, toVersion string) (types.Result, error) {
	fromResult := from.(*types040.Result)
	toResult := &Result{
		CNIVersion: toVersion,
		DNS:        *fromResult.DNS.Copy(),
		Routes:     []*types.Route{},
	}
	for _, fromIntf := range fromResult.Interfaces {
		toResult.Interfaces = append(toResult.Interfaces, convertInterfaceFrom040(fromIntf))
	}
	for _, fromIPC := range fromResult.IPs {
		toResult.IPs = append(toResult.IPs, convertIPConfigFrom040(fromIPC))
	}
	for _, fromRoute := range fromResult.Routes {
		toResult.Routes = append(toResult.Routes, fromRoute.Copy())
	}
	return toResult, nil
}

func convertIPConfigTo040(from *IPConfig) *types040.IPConfig {
	version := "6"
	if from.Address.IP.To4() != nil {
		version = "4"
	}
	to := &types040.IPConfig{
		Version: version,
		Address: from.Address,
		Gateway: from.Gateway,
	}
	if from.Interface != nil {
		intf := *from.Interface
		to.Interface = &intf
	}
	return to
}

func convertInterfaceTo040(from *Interface) *types040.Interface {
	return &types040.Interface{
		Name:    from.Name,
		Mac:     from.Mac,
		Sandbox: from.Sandbox,
	}
}

func convertTo04x(from types.Result, toVersion string) (types.Result, error) {
	fromResult := from.(*Result)
	toResult := &types040.Result{
		CNIVersion: toVersion,
		DNS:        *fromResult.DNS.Copy(),
		Routes:     []*types.Route{},
	}
	for _, fromIntf := range fromResult.Interfaces {
		toResult.Interfaces = append(toResult.Interfaces, convertInterfaceTo040(fromIntf))
	}
	for _, fromIPC := range fromResult.IPs {
		toResult.IPs = append(toResult.IPs, convertIPConfigTo040(fromIPC))
	}
	for _, fromRoute := range fromResult.Routes {
		toResult.Routes = append(toResult.Routes, fromRoute.Copy())
	}
	return toResult, nil
}

func convertTo02x(from types.Result, toVersion string) (types.Result, error) {
	// First convert to 0.4.0
	result040, err := convertTo04x(from, "0.4.0")
	if err != nil {
		return nil, err
	}
	result02x, err := convert.Convert(result040, toVersion)
	if err != nil {
		return nil, err
	}
	return result02x, nil
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

// Interface contains values about the created interfaces
type Interface struct {
	Name    string `json:"name"`
	Mac     string `json:"mac,omitempty"`
	Sandbox string `json:"sandbox,omitempty"`
}

func (i *Interface) String() string {
	return fmt.Sprintf("%+v", *i)
}

func (i *Interface) Copy() *Interface {
	if i == nil {
		return nil
	}
	newIntf := *i
	return &newIntf
}

// Int returns a pointer to the int value passed in.  Used to
// set the IPConfig.Interface field.
func Int(v int) *int {
	return &v
}

// IPConfig contains values necessary to configure an IP address on an interface
type IPConfig struct {
	// Index into Result structs Interfaces list
	Interface *int
	Address   net.IPNet
	Gateway   net.IP
}

func (i *IPConfig) String() string {
	return fmt.Sprintf("%+v", *i)
}

func (i *IPConfig) Copy() *IPConfig {
	if i == nil {
		return nil
	}

	ipc := &IPConfig{
		Address: i.Address,
		Gateway: i.Gateway,
	}
	if i.Interface != nil {
		intf := *i.Interface
		ipc.Interface = &intf
	}
	return ipc
}

// JSON (un)marshallable types
type ipConfig struct {
	Interface *int        `json:"interface,omitempty"`
	Address   types.IPNet `json:"address"`
	Gateway   net.IP      `json:"gateway,omitempty"`
}

func (c *IPConfig) MarshalJSON() ([]byte, error) {
	ipc := ipConfig{
		Interface: c.Interface,
		Address:   types.IPNet(c.Address),
		Gateway:   c.Gateway,
	}

	return json.Marshal(ipc)
}

func (c *IPConfig) UnmarshalJSON(data []byte) error {
	ipc := ipConfig{}
	if err := json.Unmarshal(data, &ipc); err != nil {
		return err
	}

	c.Interface = ipc.Interface
	c.Address = net.IPNet(ipc.Address)
	c.Gateway = ipc.Gateway
	return nil
}
