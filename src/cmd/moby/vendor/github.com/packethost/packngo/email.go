package packngo

const emailBasePath = "/emails"

// EmailService interface defines available email methods
type EmailService interface {
	Get(string) (*Email, *Response, error)
}

// Email represents a user's email address
type Email struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Default bool   `json:"default,omitempty"`
	URL     string `json:"href,omitempty"`
}

func (e Email) String() string {
	return Stringify(e)
}

// EmailServiceOp implements EmailService
type EmailServiceOp struct {
	client *Client
}

// Get retrieves an email by id
func (s *EmailServiceOp) Get(emailID string) (*Email, *Response, error) {
	req, err := s.client.NewRequest("GET", emailBasePath, nil)
	if err != nil {
		return nil, nil, err
	}

	email := new(Email)
	resp, err := s.client.Do(req, email)
	if err != nil {
		return nil, resp, err
	}

	return email, resp, err
}
