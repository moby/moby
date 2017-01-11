package api

import (
	"fmt"
)

// AgentCheck represents a check known to the agent
type AgentCheck struct {
	Node        string
	CheckID     string
	Name        string
	Status      string
	Notes       string
	Output      string
	ServiceID   string
	ServiceName string
}

// AgentService represents a service known to the agent
type AgentService struct {
	ID      string
	Service string
	Tags    []string
	Port    int
	Address string
}

// AgentMember represents a cluster member known to the agent
type AgentMember struct {
	Name        string
	Addr        string
	Port        uint16
	Tags        map[string]string
	Status      int
	ProtocolMin uint8
	ProtocolMax uint8
	ProtocolCur uint8
	DelegateMin uint8
	DelegateMax uint8
	DelegateCur uint8
}

// AgentServiceRegistration is used to register a new service
type AgentServiceRegistration struct {
	ID      string   `json:",omitempty"`
	Name    string   `json:",omitempty"`
	Tags    []string `json:",omitempty"`
	Port    int      `json:",omitempty"`
	Address string   `json:",omitempty"`
	Check   *AgentServiceCheck
	Checks  AgentServiceChecks
}

// AgentCheckRegistration is used to register a new check
type AgentCheckRegistration struct {
	ID        string `json:",omitempty"`
	Name      string `json:",omitempty"`
	Notes     string `json:",omitempty"`
	ServiceID string `json:",omitempty"`
	AgentServiceCheck
}

// AgentServiceCheck is used to create an associated
// check for a service
type AgentServiceCheck struct {
	Script   string `json:",omitempty"`
	Interval string `json:",omitempty"`
	TTL      string `json:",omitempty"`
}
type AgentServiceChecks []*AgentServiceCheck

// Agent can be used to query the Agent endpoints
type Agent struct {
	c *Client

	// cache the node name
	nodeName string
}

// Agent returns a handle to the agent endpoints
func (c *Client) Agent() *Agent {
	return &Agent{c: c}
}

// Self is used to query the agent we are speaking to for
// information about itself
func (a *Agent) Self() (map[string]map[string]interface{}, error) {
	r := a.c.newRequest("GET", "/v1/agent/self")
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out map[string]map[string]interface{}
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// NodeName is used to get the node name of the agent
func (a *Agent) NodeName() (string, error) {
	if a.nodeName != "" {
		return a.nodeName, nil
	}
	info, err := a.Self()
	if err != nil {
		return "", err
	}
	name := info["Config"]["NodeName"].(string)
	a.nodeName = name
	return name, nil
}

// Checks returns the locally registered checks
func (a *Agent) Checks() (map[string]*AgentCheck, error) {
	r := a.c.newRequest("GET", "/v1/agent/checks")
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out map[string]*AgentCheck
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Services returns the locally registered services
func (a *Agent) Services() (map[string]*AgentService, error) {
	r := a.c.newRequest("GET", "/v1/agent/services")
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out map[string]*AgentService
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Members returns the known gossip members. The WAN
// flag can be used to query a server for WAN members.
func (a *Agent) Members(wan bool) ([]*AgentMember, error) {
	r := a.c.newRequest("GET", "/v1/agent/members")
	if wan {
		r.params.Set("wan", "1")
	}
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []*AgentMember
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ServiceRegister is used to register a new service with
// the local agent
func (a *Agent) ServiceRegister(service *AgentServiceRegistration) error {
	r := a.c.newRequest("PUT", "/v1/agent/service/register")
	r.obj = service
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ServiceDeregister is used to deregister a service with
// the local agent
func (a *Agent) ServiceDeregister(serviceID string) error {
	r := a.c.newRequest("PUT", "/v1/agent/service/deregister/"+serviceID)
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// PassTTL is used to set a TTL check to the passing state
func (a *Agent) PassTTL(checkID, note string) error {
	return a.UpdateTTL(checkID, note, "pass")
}

// WarnTTL is used to set a TTL check to the warning state
func (a *Agent) WarnTTL(checkID, note string) error {
	return a.UpdateTTL(checkID, note, "warn")
}

// FailTTL is used to set a TTL check to the failing state
func (a *Agent) FailTTL(checkID, note string) error {
	return a.UpdateTTL(checkID, note, "fail")
}

// UpdateTTL is used to update the TTL of a check
func (a *Agent) UpdateTTL(checkID, note, status string) error {
	switch status {
	case "pass":
	case "warn":
	case "fail":
	default:
		return fmt.Errorf("Invalid status: %s", status)
	}
	endpoint := fmt.Sprintf("/v1/agent/check/%s/%s", status, checkID)
	r := a.c.newRequest("PUT", endpoint)
	r.params.Set("note", note)
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// CheckRegister is used to register a new check with
// the local agent
func (a *Agent) CheckRegister(check *AgentCheckRegistration) error {
	r := a.c.newRequest("PUT", "/v1/agent/check/register")
	r.obj = check
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// CheckDeregister is used to deregister a check with
// the local agent
func (a *Agent) CheckDeregister(checkID string) error {
	r := a.c.newRequest("PUT", "/v1/agent/check/deregister/"+checkID)
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Join is used to instruct the agent to attempt a join to
// another cluster member
func (a *Agent) Join(addr string, wan bool) error {
	r := a.c.newRequest("PUT", "/v1/agent/join/"+addr)
	if wan {
		r.params.Set("wan", "1")
	}
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ForceLeave is used to have the agent eject a failed node
func (a *Agent) ForceLeave(node string) error {
	r := a.c.newRequest("PUT", "/v1/agent/force-leave/"+node)
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// EnableServiceMaintenance toggles service maintenance mode on
// for the given service ID.
func (a *Agent) EnableServiceMaintenance(serviceID, reason string) error {
	r := a.c.newRequest("PUT", "/v1/agent/service/maintenance/"+serviceID)
	r.params.Set("enable", "true")
	r.params.Set("reason", reason)
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// DisableServiceMaintenance toggles service maintenance mode off
// for the given service ID.
func (a *Agent) DisableServiceMaintenance(serviceID string) error {
	r := a.c.newRequest("PUT", "/v1/agent/service/maintenance/"+serviceID)
	r.params.Set("enable", "false")
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// EnableNodeMaintenance toggles node maintenance mode on for the
// agent we are connected to.
func (a *Agent) EnableNodeMaintenance(reason string) error {
	r := a.c.newRequest("PUT", "/v1/agent/maintenance")
	r.params.Set("enable", "true")
	r.params.Set("reason", reason)
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// DisableNodeMaintenance toggles node maintenance mode off for the
// agent we are connected to.
func (a *Agent) DisableNodeMaintenance() error {
	r := a.c.newRequest("PUT", "/v1/agent/maintenance")
	r.params.Set("enable", "false")
	_, resp, err := requireOK(a.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
