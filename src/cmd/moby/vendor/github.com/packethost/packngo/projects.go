package packngo

import "fmt"

const projectBasePath = "/projects"

// ProjectService interface defines available project methods
type ProjectService interface {
	List() ([]Project, *Response, error)
	Get(string) (*Project, *Response, error)
	Create(*ProjectCreateRequest) (*Project, *Response, error)
	Update(*ProjectUpdateRequest) (*Project, *Response, error)
	Delete(string) (*Response, error)
	ListIPAddresses(string) ([]IPAddress, *Response, error)
	ListVolumes(string) ([]Volume, *Response, error)
}

type ipsRoot struct {
	IPAddresses []IPAddress `json:"ip_addresses"`
}

type volumesRoot struct {
	Volumes []Volume `json:"volumes"`
}

type projectsRoot struct {
	Projects []Project `json:"projects"`
}

// Project represents a Packet project
type Project struct {
	ID      string   `json:"id"`
	Name    string   `json:"name,omitempty"`
	Created string   `json:"created_at,omitempty"`
	Updated string   `json:"updated_at,omitempty"`
	Users   []User   `json:"members,omitempty"`
	Devices []Device `json:"devices,omitempty"`
	SSHKeys []SSHKey `json:"ssh_keys,omitempty"`
	URL     string   `json:"href,omitempty"`
}

func (p Project) String() string {
	return Stringify(p)
}

// ProjectCreateRequest type used to create a Packet project
type ProjectCreateRequest struct {
	Name          string `json:"name"`
	PaymentMethod string `json:"payment_method,omitempty"`
}

func (p ProjectCreateRequest) String() string {
	return Stringify(p)
}

// ProjectUpdateRequest type used to update a Packet project
type ProjectUpdateRequest struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	PaymentMethod string `json:"payment_method,omitempty"`
}

func (p ProjectUpdateRequest) String() string {
	return Stringify(p)
}

// ProjectServiceOp implements ProjectService
type ProjectServiceOp struct {
	client *Client
}

func (s *ProjectServiceOp) ListIPAddresses(projectID string) ([]IPAddress, *Response, error) {
	url := fmt.Sprintf("%s/%s/ips", projectBasePath, projectID)
	req, err := s.client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	root := new(ipsRoot)
	resp, err := s.client.Do(req, root)
	if err != nil {
		return nil, resp, err
	}

	return root.IPAddresses, resp, err
}

// List returns the user's projects
func (s *ProjectServiceOp) List() ([]Project, *Response, error) {
	req, err := s.client.NewRequest("GET", projectBasePath, nil)
	if err != nil {
		return nil, nil, err
	}

	root := new(projectsRoot)
	resp, err := s.client.Do(req, root)
	if err != nil {
		return nil, resp, err
	}

	return root.Projects, resp, err
}

// Get returns a project by id
func (s *ProjectServiceOp) Get(projectID string) (*Project, *Response, error) {
	path := fmt.Sprintf("%s/%s", projectBasePath, projectID)
	req, err := s.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	project := new(Project)
	resp, err := s.client.Do(req, project)
	if err != nil {
		return nil, resp, err
	}

	return project, resp, err
}

// Create creates a new project
func (s *ProjectServiceOp) Create(createRequest *ProjectCreateRequest) (*Project, *Response, error) {
	req, err := s.client.NewRequest("POST", projectBasePath, createRequest)
	if err != nil {
		return nil, nil, err
	}

	project := new(Project)
	resp, err := s.client.Do(req, project)
	if err != nil {
		return nil, resp, err
	}

	return project, resp, err
}

// Update updates a project
func (s *ProjectServiceOp) Update(updateRequest *ProjectUpdateRequest) (*Project, *Response, error) {
	path := fmt.Sprintf("%s/%s", projectBasePath, updateRequest.ID)
	req, err := s.client.NewRequest("PATCH", path, updateRequest)
	if err != nil {
		return nil, nil, err
	}

	project := new(Project)
	resp, err := s.client.Do(req, project)
	if err != nil {
		return nil, resp, err
	}

	return project, resp, err
}

// Delete deletes a project
func (s *ProjectServiceOp) Delete(projectID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", projectBasePath, projectID)

	req, err := s.client.NewRequest("DELETE", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)

	return resp, err
}

// List returns Volumes for a project
func (s *ProjectServiceOp) ListVolumes(projectID string) ([]Volume, *Response, error) {
	url := fmt.Sprintf("%s/%s%s", projectBasePath, projectID, volumeBasePath)
	req, err := s.client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	root := new(volumesRoot)
	resp, err := s.client.Do(req, root)
	if err != nil {
		return nil, resp, err
	}

	return root.Volumes, resp, err
}
