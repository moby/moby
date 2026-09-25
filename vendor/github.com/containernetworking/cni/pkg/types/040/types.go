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

package types040

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/types"
	types020 "github.com/containernetworking/cni/pkg/types/020"
	convert "github.com/containernetworking/cni/pkg/types/internal"
)

const ImplementedSpecVersion string = "0.4.0"

var supportedVersions = []string{"0.3.0", "0.3.1", ImplementedSpecVersion}

// Register converters for all versions less than the implemented spec version
func init() {
	// Up-converters
	convert.RegisterConverter("0.1.0", supportedVersions, convertFrom02x)
	convert.RegisterConverter("0.2.0", supportedVersions, convertFrom02x)
	convert.RegisterConverter("0.3.0", supportedVersions, convertInternal)
	convert.RegisterConverter("0.3.1", supportedVersions, convertInternal)

	// Down-converters
	convert.RegisterConverter("0.4.0", []string{"0.3.0", "0.3.1"}, convertInternal)
	convert.RegisterConverter("0.4.0", []string{"0.1.0", "0.2.0"}, convertTo02x)
	convert.RegisterConverter("0.3.1", []string{"0.1.0", "0.2.0"}, convertTo02x)
	convert.RegisterConverter("0.3.0", []string{"0.1.0", "0.2.0"}, convertTo02x)

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

func convert020IPConfig(from *types020.IPConfig, ipVersion string) *IPConfig {
	return &IPConfig{
		Version: ipVersion,
		Address: from.IP,
		Gateway: from.Gateway,
	}
}

func convertFrom02x(from types.Result, toVersion string) (types.Result, error) {
	fromResult := from.(*types020.Result)
	toResult := &Result{
		CNIVersion: toVersion,
		DNS:        *fromResult.DNS.Copy(),
		Routes:     []*types.Route{},
	}
	if fromResult.IP4 != nil {
		toResult.IPs = append(toResult.IPs, convert020IPConfig(fromResult.IP4, "4"))
		for _, fromRoute := range fromResult.IP4.Routes {
			toResult.Routes = append(toResult.Routes, fromRoute.Copy())
		}
	}

	if fromResult.IP6 != nil {
		toResult.IPs = append(toResult.IPs, convert020IPConfig(fromResult.IP6, "6"))
		for _, fromRoute := range fromResult.IP6.Routes {
			toResult.Routes = append(toResult.Routes, fromRoute.Copy())
		}
	}

	return toResult, nil
}

func convertInternal(from types.Result, toVersion string) (types.Result, error) {
	fromResult := from.(*Result)
	toResult := &Result{
		CNIVersion: toVersion,
		DNS:        *fromResult.DNS.Copy(),
		Routes:     []*types.Route{},
	}
	for _, fromIntf := range fromResult.Interfaces {
		toResult.Interfaces = append(toResult.Interfaces, fromIntf.Copy())
	}
	for _, fromIPC := range fromResult.IPs {
		toResult.IPs = append(toResult.IPs, fromIPC.Copy())
	}
	for _, fromRoute := range fromResult.Routes {
		toResult.Routes = append(toResult.Routes, fromRoute.Copy())
	}
	return toResult, nil
}

func convertTo02x(from types.Result, toVersion string) (types.Result, error) {
	fromResult := from.(*Result)
	toResult := &types020.Result{
		CNIVersion: toVersion,
		DNS:        *fromResult.DNS.Copy(),
	}

	for _, fromIP := range fromResult.IPs {
		// Only convert the first IP address of each version as 0.2.0
		// and earlier cannot handle multiple IP addresses
		if fromIP.Version == "4" && toResult.IP4 == nil {
			toResult.IP4 = &types020.IPConfig{
				IP:      fromIP.Address,
				Gateway: fromIP.Gateway,
			}
		} else if fromIP.Version == "6" && toResult.IP6 == nil {
			toResult.IP6 = &types020.IPConfig{
				IP:      fromIP.Address,
				Gateway: fromIP.Gateway,
			}
		}
		if toResult.IP4 != nil && toResult.IP6 != nil {
			break
		}
	}

	for _, fromRoute := range fromResult.Routes {
		is4 := fromRoute.Dst.IP.To4() != nil
		if is4 && toResult.IP4 != nil {
			toResult.IP4.Routes = append(toResult.IP4.Routes, types.Route{
				Dst: fromRoute.Dst,
				GW:  fromRoute.GW,
			})
		} else if !is4 && toResult.IP6 != nil {
			toResult.IP6.Routes = append(toResult.IP6.Routes, types.Route{
				Dst: fromRoute.Dst,
				GW:  fromRoute.GW,
			})
		}
	}

	// 0.2.0 and earlier require at least one IP address in the Result
	if toResult.IP4 == nil && toResult.IP6 == nil {
		return nil, fmt.Errorf("cannot convert: no valid IP addresses")
	}

	return toResult, nil
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
	// IP version, either "4" or "6"
	Version string
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
		Version: i.Version,
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
	Version   string      `json:"version"`
	Interface *int        `json:"interface,omitempty"`
	Address   types.IPNet `json:"address"`
	Gateway   net.IP      `json:"gateway,omitempty"`
}

func (c *IPConfig) MarshalJSON() ([]byte, error) {
	ipc := ipConfig{
		Version:   c.Version,
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

	c.Version = ipc.Version
	c.Interface = ipc.Interface
	c.Address = net.IPNet(ipc.Address)
	c.Gateway = ipc.Gateway
	return nil
}
