package authn

import (
	"errors"
	"net/http"
)

// BasicAuther is an interface which a caller may provide for obtaining a user
// name and password to use when attempting Basic authentication with a server.
type BasicAuther interface {
	GetBasicAuth(realm string) (user, password string, err error)
}

type basic struct {
	logger             Logger
	username, password string
	auther             BasicAuther
}

func (b *basic) scheme() string {
	return "Basic"
}

func (b *basic) authRespond(challenge string, req *http.Request) (result bool, err error) {
	if b.username != "" && b.password != "" {
		b.logger.Debug("using previously-supplied Basic username and password")
		req.SetBasicAuth(b.username, b.password)
		return true, nil
	}

	if b.auther == nil {
		b.logger.Debug("failed to obtain user name and password for Basic auth")
		return false, nil
	}

	realm, _ := getParameter(challenge, "realm")
	username, password, err := b.auther.GetBasicAuth(realm)
	if err != nil {
		return false, err
	}
	if username == "" {
		b.logger.Debug("failed to obtain user name for Basic auth")
		return false, nil
	}
	if password == "" {
		b.logger.Debug("failed to obtain password for Basic auth")
		return false, nil
	}

	b.username = username
	b.password = password
	req.SetBasicAuth(b.username, b.password)
	return true, nil
}

func (b *basic) authCompleted(challenge string, resp *http.Response) (result bool, err error) {
	if challenge == "" {
		return true, nil
	}
	return false, errors.New("Error: unexpected WWW-Authenticate header in server response")
}

func createBasic(logger Logger, authers []interface{}) authResponder {
	var ba BasicAuther
	for _, auther := range authers {
		if b, ok := auther.(BasicAuther); ok {
			ba = b
			if ba != nil {
				break
			}
		}
	}
	return &basic{logger: logger, auther: ba}
}

func init() {
	authResponderCreators = append(authResponderCreators, createBasic)
}
