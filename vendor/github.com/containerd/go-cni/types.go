/*
   Copyright The containerd Authors.

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

package cni

const (
	CNIPluginName        = "cni"
	DefaultNetDir        = "/etc/cni/net.d"
	DefaultCNIDir        = "/opt/cni/bin"
	DefaultMaxConfNum    = 1
	VendorCNIDirTemplate = "%s/opt/%s/bin"
	DefaultPrefix        = "eth"
)

type config struct {
	pluginDirs       []string
	pluginConfDir    string
	pluginMaxConfNum int
	prefix           string
}

type PortMapping struct {
	HostPort      int32
	ContainerPort int32
	Protocol      string
	HostIP        string
}

type IPRanges struct {
	Subnet     string
	RangeStart string
	RangeEnd   string
	Gateway    string
}

// BandWidth defines the ingress/egress rate and burst limits
type BandWidth struct {
	IngressRate  uint64
	IngressBurst uint64
	EgressRate   uint64
	EgressBurst  uint64
}

// DNS defines the dns config
type DNS struct {
	// List of DNS servers of the cluster.
	Servers []string
	// List of DNS search domains of the cluster.
	Searches []string
	// List of DNS options.
	Options []string
}
