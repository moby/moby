package packngo

import "fmt"

const sshKeyBasePath = "/ssh-keys"

// SSHKeyService interface defines available device methods
type SSHKeyService interface {
	List() ([]SSHKey, *Response, error)
	Get(string) (*SSHKey, *Response, error)
	Create(*SSHKeyCreateRequest) (*SSHKey, *Response, error)
	Update(*SSHKeyUpdateRequest) (*SSHKey, *Response, error)
	Delete(string) (*Response, error)
}

type sshKeyRoot struct {
	SSHKeys []SSHKey `json:"ssh_keys"`
}

// SSHKey represents a user's ssh key
type SSHKey struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Key         string `json:"key"`
	FingerPrint string `json:"fingerprint"`
	Created     string `json:"created_at"`
	Updated     string `json:"updated_at"`
	User        User   `json:"user,omitempty"`
	URL         string `json:"href,omitempty"`
}

func (s SSHKey) String() string {
	return Stringify(s)
}

// SSHKeyCreateRequest type used to create an ssh key
type SSHKeyCreateRequest struct {
	Label     string `json:"label"`
	Key       string `json:"key"`
	ProjectID string `json:"-"`
}

func (s SSHKeyCreateRequest) String() string {
	return Stringify(s)
}

// SSHKeyUpdateRequest type used to update an ssh key
type SSHKeyUpdateRequest struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Key   string `json:"key"`
}

func (s SSHKeyUpdateRequest) String() string {
	return Stringify(s)
}

// SSHKeyServiceOp implements SSHKeyService
type SSHKeyServiceOp struct {
	client *Client
}

// List returns a user's ssh keys
func (s *SSHKeyServiceOp) List() ([]SSHKey, *Response, error) {
	req, err := s.client.NewRequest("GET", sshKeyBasePath, nil)
	if err != nil {
		return nil, nil, err
	}

	root := new(sshKeyRoot)
	resp, err := s.client.Do(req, root)
	if err != nil {
		return nil, resp, err
	}

	return root.SSHKeys, resp, err
}

// Get returns an ssh key by id
func (s *SSHKeyServiceOp) Get(sshKeyID string) (*SSHKey, *Response, error) {
	path := fmt.Sprintf("%s/%s", sshKeyBasePath, sshKeyID)

	req, err := s.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}

	sshKey := new(SSHKey)
	resp, err := s.client.Do(req, sshKey)
	if err != nil {
		return nil, resp, err
	}

	return sshKey, resp, err
}

// Create creates a new ssh key
func (s *SSHKeyServiceOp) Create(createRequest *SSHKeyCreateRequest) (*SSHKey, *Response, error) {
	path := sshKeyBasePath
	if createRequest.ProjectID != "" {
		path = "/projects/" + createRequest.ProjectID + sshKeyBasePath
	}
	req, err := s.client.NewRequest("POST", path, createRequest)
	if err != nil {
		return nil, nil, err
	}

	sshKey := new(SSHKey)
	resp, err := s.client.Do(req, sshKey)
	if err != nil {
		return nil, resp, err
	}

	return sshKey, resp, err
}

// Update updates an ssh key
func (s *SSHKeyServiceOp) Update(updateRequest *SSHKeyUpdateRequest) (*SSHKey, *Response, error) {
	path := fmt.Sprintf("%s/%s", sshKeyBasePath, updateRequest.ID)
	req, err := s.client.NewRequest("PATCH", path, updateRequest)
	if err != nil {
		return nil, nil, err
	}

	sshKey := new(SSHKey)
	resp, err := s.client.Do(req, sshKey)
	if err != nil {
		return nil, resp, err
	}

	return sshKey, resp, err
}

// Delete deletes an ssh key
func (s *SSHKeyServiceOp) Delete(sshKeyID string) (*Response, error) {
	path := fmt.Sprintf("%s/%s", sshKeyBasePath, sshKeyID)

	req, err := s.client.NewRequest("DELETE", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)

	return resp, err
}
