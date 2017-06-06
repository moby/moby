package packngo

import "fmt"

const ipBasePath = "/ips"

// IPService interface defines available IP methods
type IPService interface {
	Assign(deviceID string, assignRequest *IPAddressAssignRequest) (*IPAddress, *Response, error)
	Unassign(ipAddressID string) (*Response, error)
	Get(ipAddressID string) (*IPAddress, *Response, error)
}

// IPAddress represents a ip address
type IPAddress struct {
	ID            string            `json:"id"`
	Address       string            `json:"address"`
	Gateway       string            `json:"gateway"`
	Network       string            `json:"network"`
	AddressFamily int               `json:"address_family"`
	Netmask       string            `json:"netmask"`
	Public        bool              `json:"public"`
	Cidr          int               `json:"cidr"`
	AssignedTo    map[string]string `json:"assigned_to"`
	Created       string            `json:"created_at,omitempty"`
	Updated       string            `json:"updated_at,omitempty"`
	Href          string            `json:"href"`
	Facility      Facility          `json:"facility,omitempty"`
}

// IPAddressAssignRequest represents the body if a ip assign request
type IPAddressAssignRequest struct {
	Address string `json:"address"`
}

func (i IPAddress) String() string {
	return Stringify(i)
}

// IPServiceOp implements IPService
type IPServiceOp struct {
	client *Client
}

// Get returns IpAddress by ID
func (i *IPServiceOp) Get(ipAddressID string) (*IPAddress, *Response, error) {
	path := fmt.Sprintf("%s/%s", ipBasePath, ipAddressID)

	req, err := i.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	ip := new(IPAddress)
	resp, err := i.client.Do(req, ip)
	if err != nil {
		return nil, resp, err
	}

	return ip, resp, err
}

// Unassign unassigns an IP address record. This will remove the relationship between an IP
// and the device and will make the IP address available to be assigned to another device.
func (i *IPServiceOp) Unassign(ipAddressID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", ipBasePath, ipAddressID)

	req, err := i.client.NewRequest("DELETE", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.client.Do(req, nil)
	return resp, err
}

// Assign assigns an IP address to a device. The IP address must be in one of the IP ranges assigned to the device’s project.
func (i *IPServiceOp) Assign(deviceID string, assignRequest *IPAddressAssignRequest) (*IPAddress, *Response, error) {
	path := fmt.Sprintf("%s/%s%s", deviceBasePath, deviceID, ipBasePath)

	req, err := i.client.NewRequest("POST", path, assignRequest)

	ip := new(IPAddress)
	resp, err := i.client.Do(req, ip)
	if err != nil {
		return nil, resp, err
	}

	return ip, resp, err
}

// IP RESERVATIONS API

// IPReservationService interface defines available IPReservation methods
type IPReservationService interface {
	List(projectID string) ([]IPReservation, *Response, error)
	RequestMore(projectID string, ipReservationReq *IPReservationRequest) (*IPReservation, *Response, error)
	Get(ipReservationID string) (*IPReservation, *Response, error)
	Remove(ipReservationID string) (*Response, error)
}

// IPReservationServiceOp implements the IPReservationService interface
type IPReservationServiceOp struct {
	client *Client
}

// IPReservationRequest represents the body of a reservation request
type IPReservationRequest struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
	Comments string `json:"comments"`
}

// IPReservation represent an IP reservation for a single project
type IPReservation struct {
	ID            string              `json:"id"`
	Network       string              `json:"network"`
	Address       string              `json:"address"`
	AddressFamily int                 `json:"address_family"`
	Netmask       string              `json:"netmask"`
	Public        bool                `json:"public"`
	Cidr          int                 `json:"cidr"`
	Management    bool                `json:"management"`
	Manageable    bool                `json:"manageable"`
	Addon         bool                `json:"addon"`
	Bill          bool                `json:"bill"`
	Assignments   []map[string]string `json:"assignments"`
	Created       string              `json:"created_at,omitempty"`
	Updated       string              `json:"updated_at,omitempty"`
	Href          string              `json:"href"`
}

type ipReservationRoot struct {
	IPReservations []IPReservation `json:"ip_addresses"`
}

// List provides a list of IP resevations for a single project.
func (i *IPReservationServiceOp) List(projectID string) ([]IPReservation, *Response, error) {
	path := fmt.Sprintf("%s/%s%s", projectBasePath, projectID, ipBasePath)

	req, err := i.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	reservations := new(ipReservationRoot)
	resp, err := i.client.Do(req, reservations)
	if err != nil {
		return nil, resp, err
	}
	return reservations.IPReservations, resp, err
}

// RequestMore requests more IP space for a project in order to have additional IP addresses to assign to devices
func (i *IPReservationServiceOp) RequestMore(projectID string, ipReservationReq *IPReservationRequest) (*IPReservation, *Response, error) {
	path := fmt.Sprintf("%s/%s%s", projectBasePath, projectID, ipBasePath)

	req, err := i.client.NewRequest("POST", path, &ipReservationReq)
	if err != nil {
		return nil, nil, err
	}

	ip := new(IPReservation)
	resp, err := i.client.Do(req, ip)
	if err != nil {
		return nil, resp, err
	}
	return ip, resp, err
}

// Get returns a single IP reservation object
func (i *IPReservationServiceOp) Get(ipReservationID string) (*IPReservation, *Response, error) {
	path := fmt.Sprintf("%s/%s", ipBasePath, ipReservationID)

	req, err := i.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	reservation := new(IPReservation)
	resp, err := i.client.Do(req, reservation)
	if err != nil {
		return nil, nil, err
	}

	return reservation, resp, err
}

// Remove removes an IP reservation from the project.
func (i *IPReservationServiceOp) Remove(ipReservationID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", ipBasePath, ipReservationID)

	req, err := i.client.NewRequest("DELETE", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.client.Do(req, nil)
	if err != nil {
		return nil, err
	}

	return resp, err
}
