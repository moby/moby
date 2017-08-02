package hcsshim

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
)

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

// Create Endpoint by sending EndpointRequest to HNS. TODO: Create a separate HNS interface to place all these methods
func (endpoint *HNSEndpoint) Create() (*HNSEndpoint, error) {
	operation := "Create"
	title := "HCSShim::HNSEndpoint::" + operation
	logrus.Debugf(title+" id=%s", endpoint.Id)

	jsonString, err := json.Marshal(endpoint)
	if err != nil {
		return nil, err
	}
	return HNSEndpointRequest("POST", "", string(jsonString))
}

// Delete Endpoint by sending EndpointRequest to HNS
func (endpoint *HNSEndpoint) Delete() (*HNSEndpoint, error) {
	operation := "Delete"
	title := "HCSShim::HNSEndpoint::" + operation
	logrus.Debugf(title+" id=%s", endpoint.Id)

	return HNSEndpointRequest("DELETE", endpoint.Id, "")
}

// Delete Endpoint by sending EndpointRequest to HNS
func (endpoint *HNSEndpoint) Update() (*HNSEndpoint, error) {
	operation := "Update"
	title := "HCSShim::HNSEndpoint::" + operation
	logrus.Debugf(title+" id=%s", endpoint.Id)
	jsonString, err := json.Marshal(endpoint)
	if err != nil {
		return nil, err
	}
	err = hnsCall("POST", "/endpoints/"+endpoint.Id+"/update", string(jsonString), &endpoint)

	return endpoint, err
}

// Hot Attach an endpoint to a container
func (endpoint *HNSEndpoint) HotAttach(containerID string) error {
	operation := "HotAttach"
	title := "HCSShim::HNSEndpoint::" + operation
	logrus.Debugf(title+" id=%s, containerId=%s", endpoint.Id, containerID)

	return modifyNetworkEndpoint(containerID, endpoint.Id, Add)
}

// Hot Detach an endpoint from a container
func (endpoint *HNSEndpoint) HotDetach(containerID string) error {
	operation := "HotDetach"
	title := "HCSShim::HNSEndpoint::" + operation
	logrus.Debugf(title+" id=%s, containerId=%s", endpoint.Id, containerID)

	return modifyNetworkEndpoint(containerID, endpoint.Id, Remove)
}

// Apply Acl Policy on the Endpoint
func (endpoint *HNSEndpoint) ApplyACLPolicy(policy *ACLPolicy) error {
	operation := "ApplyACLPolicy"
	title := "HCSShim::HNSEndpoint::" + operation
	logrus.Debugf(title+" id=%s", endpoint.Id)

	jsonString, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	endpoint.Policies[0] = jsonString
	_, err = endpoint.Update()
	return err
}
