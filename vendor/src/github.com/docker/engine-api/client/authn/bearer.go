package authn

import (
	"errors"
	"net/http"
)

// BearerAuther is an interface which a caller may provide for obtaining a
// token to use in attempting Bearer authentication with a server.
type BearerAuther interface {
	GetBearerAuth(challenge string) (token string, err error)
}

type bearer struct {
	logger Logger
	token  string
	auther BearerAuther
}

func (b *bearer) scheme() string {
	return "Bearer"
}

func (b *bearer) authRespond(challenge string, req *http.Request) (result bool, err error) {
	token := b.token
	if token != "" {
		b.logger.Debug("using previously-supplied Bearer token")
		req.Header.Add("Authorization", "Bearer "+token)
		return true, nil
	}
	if b.auther == nil {
		b.logger.Debug("failed to obtain token for Bearer auth")
		return false, nil
	}
	token, err = b.auther.GetBearerAuth(challenge)
	if err != nil {
		return false, err
	}
	if token == "" {
		b.logger.Debug("Bearer token not supplied")
		return false, nil
	}
	b.token = token
	req.Header.Add("Authorization", "Bearer "+b.token)
	return true, nil
}

func (b *bearer) authCompleted(challenge string, resp *http.Response) (result bool, err error) {
	if challenge == "" {
		return true, nil
	}
	return false, errors.New("Error: unexpected WWW-Authenticate header in server response")
}

func createBearer(logger Logger, authers []interface{}) authResponder {
	var ba BearerAuther
	for _, auther := range authers {
		if b, ok := auther.(BearerAuther); ok {
			ba = b
			if ba != nil {
				break
			}
		}
	}
	return &bearer{logger: logger, auther: ba}
}

func init() {
	authResponderCreators = append(authResponderCreators, createBearer)
}
