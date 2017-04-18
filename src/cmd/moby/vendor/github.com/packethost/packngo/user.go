package packngo

const userBasePath = "/users"

// UserService interface defines available user methods
type UserService interface {
	Get(string) (*User, *Response, error)
}

// User represents a Packet user
type User struct {
	ID          string  `json:"id"`
	FirstName   string  `json:"first_name,omitempty"`
	LastName    string  `json:"last_name,omitempty"`
	FullName    string  `json:"full_name,omitempty"`
	Email       string  `json:"email,omitempty"`
	TwoFactor   string  `json:"two_factor_auth,omitempty"`
	AvatarURL   string  `json:"avatar_url,omitempty"`
	Facebook    string  `json:"twitter,omitempty"`
	Twitter     string  `json:"facebook,omitempty"`
	LinkedIn    string  `json:"linkedin,omitempty"`
	Created     string  `json:"created_at,omitempty"`
	Updated     string  `json:"updated_at,omitempty"`
	TimeZone    string  `json:"timezone,omitempty"`
	Emails      []Email `json:"email,omitempty"`
	PhoneNumber string  `json:"phone_number,omitempty"`
	URL         string  `json:"href,omitempty"`
}

func (u User) String() string {
	return Stringify(u)
}

// UserServiceOp implements UserService
type UserServiceOp struct {
	client *Client
}

// Get method gets a user by userID
func (s *UserServiceOp) Get(userID string) (*User, *Response, error) {
	req, err := s.client.NewRequest("GET", userBasePath, nil)
	if err != nil {
		return nil, nil, err
	}

	user := new(User)
	resp, err := s.client.Do(req, user)
	if err != nil {
		return nil, resp, err
	}

	return user, resp, err
}
