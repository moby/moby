package hcsshim

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
)

type NatPolicy struct {
	Type         string
	Protocol     string
	InternalPort uint16
	ExternalPort uint16
}

type QosPolicy struct {
	Type                            string
	MaximumOutgoingBandwidthInBytes uint64
}

type VlanPolicy struct {
	Type string
	VLAN uint
}

type VsidPolicy struct {
	Type string
	VSID uint
}

type PaPolicy struct {
	Type string
	PA   string
}

// Subnet is assoicated with a network and represents a list
// of subnets available to the network
type Subnet struct {
	AddressPrefix  string            `json:",omitempty"`
	GatewayAddress string            `json:",omitempty"`
	Policies       []json.RawMessage `json:",omitempty"`
}

// MacPool is assoicated with a network and represents a list
// of macaddresses available to the network
type MacPool struct {
	StartMacAddress string `json:",omitempty"`
	EndMacAddress   string `json:",omitempty"`
}

// HNSNetwork represents a network in HNS
type HNSNetwork struct {
	Id                   string            `json:"ID,omitempty"`
	Name                 string            `json:",omitempty"`
	Type                 string            `json:",omitempty"`
	NetworkAdapterName   string            `json:",omitempty"`
	SourceMac            string            `json:",omitempty"`
	Policies             []json.RawMessage `json:",omitempty"`
	MacPools             []MacPool         `json:",omitempty"`
	Subnets              []Subnet          `json:",omitempty"`
	DNSSuffix            string            `json:",omitempty"`
	DNSServerList        string            `json:",omitempty"`
	DNSServerCompartment uint32            `json:",omitempty"`
	ManagementIP         string            `json:",omitempty"`
}

// HNSEndpoint represents a network endpoint in HNS
type HNSEndpoint struct {
	Id                 string            `json:"ID,omitempty"`
	Name               string            `json:",omitempty"`
	VirtualNetwork     string            `json:",omitempty"`
	VirtualNetworkName string            `json:",omitempty"`
	Policies           []json.RawMessage `json:",omitempty"`
	MacAddress         string            `json:",omitempty"`
	IPAddress          net.IP            `json:",omitempty"`
	DNSSuffix          string            `json:",omitempty"`
	DNSServerList      string            `json:",omitempty"`
	GatewayAddress     string            `json:",omitempty"`
	EnableInternalDNS  bool              `json:",omitempty"`
	DisableICC         bool              `json:",omitempty"`
	PrefixLength       uint8             `json:",omitempty"`
	IsRemoteEndpoint   bool              `json:",omitempty"`
}

type hnsNetworkResponse struct {
	Success bool
	Error   string
	Output  HNSNetwork
}

type hnsResponse struct {
	Success bool
	Error   string
	Output  json.RawMessage
}

func hnsCall(method, path, request string, returnResponse interface{}) error {
	var responseBuffer *uint16
	logrus.Debugf("[%s]=>[%s] Request : %s", method, path, request)

	err := _hnsCall(method, path, request, &responseBuffer)
	if err != nil {
		return makeError(err, "hnsCall ", "")
	}
	response := convertAndFreeCoTaskMemString(responseBuffer)

	hnsresponse := &hnsResponse{}
	if err = json.Unmarshal([]byte(response), &hnsresponse); err != nil {
		return err
	}

	if !hnsresponse.Success {
		return fmt.Errorf("HNS failed with error : %s", hnsresponse.Error)
	}

	if len(hnsresponse.Output) == 0 {
		return nil
	}

	logrus.Debugf("Network Response : %s", hnsresponse.Output)
	err = json.Unmarshal(hnsresponse.Output, returnResponse)
	if err != nil {
		return err
	}

	return nil
}

// HNSNetworkRequest makes a call into HNS to update/query a single network
func HNSNetworkRequest(method, path, request string) (*HNSNetwork, error) {
	var network HNSNetwork
	err := hnsCall(method, "/networks/"+path, request, &network)
	if err != nil {
		return nil, err
	}

	return &network, nil
}

// HNSListNetworkRequest makes a HNS call to query the list of available networks
func HNSListNetworkRequest(method, path, request string) ([]HNSNetwork, error) {
	var network []HNSNetwork
	err := hnsCall(method, "/networks/"+path, request, &network)
	if err != nil {
		return nil, err
	}

	return network, nil
}

// HNSEndpointRequest makes a HNS call to modify/query a network endpoint
func HNSEndpointRequest(method, path, request string) (*HNSEndpoint, error) {
	endpoint := &HNSEndpoint{}
	err := hnsCall(method, "/endpoints/"+path, request, &endpoint)
	if err != nil {
		return nil, err
	}

	return endpoint, nil
}

// HNSListEndpointRequest makes a HNS call to query the list of available endpoints
func HNSListEndpointRequest() ([]HNSEndpoint, error) {
	var endpoint []HNSEndpoint
	err := hnsCall("GET", "/endpoints/", "", &endpoint)
	if err != nil {
		return nil, err
	}

	return endpoint, nil
}

// HotAttachEndpoint makes a HCS Call to attach the endpoint to the container
func HotAttachEndpoint(containerID string, endpointID string) error {
	return modifyNetworkEndpoint(containerID, endpointID, Add)
}

// HotDetachEndpoint makes a HCS Call to detach the endpoint from the container
func HotDetachEndpoint(containerID string, endpointID string) error {
	return modifyNetworkEndpoint(containerID, endpointID, Remove)
}

// ModifyContainer corresponding to the container id, by sending a request
func modifyContainer(id string, request *ResourceModificationRequestResponse) error {
	container, err := OpenContainer(id)
	if err != nil {
		if IsNotExist(err) {
			return ErrComputeSystemDoesNotExist
		}
		return getInnerError(err)
	}
	defer container.Close()
	err = container.Modify(request)
	if err != nil {
		if IsNotSupported(err) {
			return ErrPlatformNotSupported
		}
		return getInnerError(err)
	}

	return nil
}

func modifyNetworkEndpoint(containerID string, endpointID string, request RequestType) error {
	requestMessage := &ResourceModificationRequestResponse{
		Resource: Network,
		Request:  request,
		Data:     endpointID,
	}
	err := modifyContainer(containerID, requestMessage)

	if err != nil {
		return err
	}

	return nil
}

// GetHNSNetworkByID
func GetHNSNetworkByID(networkID string) (*HNSNetwork, error) {
	return HNSNetworkRequest("GET", networkID, "")
}

// GetHNSNetworkName filtered by Name
func GetHNSNetworkByName(networkName string) (*HNSNetwork, error) {
	hsnnetworks, err := HNSListNetworkRequest("GET", "", "")
	if err != nil {
		return nil, err
	}
	for _, hnsnetwork := range hsnnetworks {
		if hnsnetwork.Name == networkName {
			return &hnsnetwork, nil
		}
	}
	return nil, fmt.Errorf("Network %v not found", networkName)
}

// Create Endpoint by sending EndpointRequest to HNS. TODO: Create a separate HNS interface to place all these methods
func (endpoint *HNSEndpoint) Create() (*HNSEndpoint, error) {
	jsonString, err := json.Marshal(endpoint)
	if err != nil {
		return nil, err
	}
	return HNSEndpointRequest("POST", "", string(jsonString))
}

// Create Endpoint by sending EndpointRequest to HNS
func (endpoint *HNSEndpoint) Delete() (*HNSEndpoint, error) {
	return HNSEndpointRequest("DELETE", endpoint.Id, "")
}

// GetHNSEndpointByID
func GetHNSEndpointByID(endpointID string) (*HNSEndpoint, error) {
	return HNSEndpointRequest("GET", endpointID, "")
}

// GetHNSNetworkName filtered by Name
func GetHNSEndpointByName(endpointName string) (*HNSEndpoint, error) {
	hnsResponse, err := HNSListEndpointRequest()
	if err != nil {
		return nil, err
	}
	for _, hnsEndpoint := range hnsResponse {
		if hnsEndpoint.Name == endpointName {
			return &hnsEndpoint, nil
		}
	}
	return nil, fmt.Errorf("Endpoint %v not found", endpointName)
}
