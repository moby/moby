package server

import (
	"errors"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/context"
)

// User represents an authenticated remote user.  We know at least one of the
// user's name (if not "") and UID (if HaveUid is true), and possibly both.
type User struct {
	Name    string
	HaveUid bool
	Uid     uint32
}

type contextKey int

// AuthnUser is the key to use with context.Get() to retrieve an http.Request's
// authenticated user, if the user was authenticated, like so:
//   import "github.com/gorilla/context"
//   user, authed = context.Get(r, server.AuthnUser).(server.User)
const AuthnUser contextKey = iota

// Authenticator is an interface that wraps the GetChallenge and CheckResponse
// methods, which are implemented for each accepted authentication scheme.
//
// At initialization time, an implementation of Authenticator should register
// itself by calling the RegisterAuthenticator function.
//
// If authentication is not required, Authenticator methods will not be called.
type Authenticator interface {
	// If an incoming request includes an authorization header, each
	// authenticator's CheckResponse method will be called to verify it in turn.
	// If any returns an error, then authentication has failed.  If any returns a
	// User, then authentication has succeeded.  If none return a usable User, then
	// authentication has failed.
	GetChallenge(w http.ResponseWriter, r *http.Request) error
	// If there is no authorization header in the request, each authenticator's
	// GetChallenge method will be called to add a suitable challenge header to the
	// not-authorized response.  If any returns an error, then an error will be
	// sent to the client.
	CheckResponse(w http.ResponseWriter, r *http.Request) (User, error)
}

// AuthenticatorCreator either creates a new Authenticator, or returns nil
type AuthenticatorCreater func(options ServerAuthOptions) Authenticator

var authenticatorCreaters = []AuthenticatorCreater{}

// RegisterAuthenticator registers a function which will be called at startup
// to create an Authenticator.
func RegisterAuthenticator(ac AuthenticatorCreater) {
	authenticatorCreaters = append(authenticatorCreaters, ac)
}

// Run through all of the registered authenticator callback creators and build
// a list of authenticating functions.
func createAuthenticators(c *ServerConfig) []Authenticator {
	authenticators := []Authenticator{}
	for _, ac := range authenticatorCreaters {
		authenticator := ac(c.AuthOptions)
		if authenticator != nil {
			authenticators = append(authenticators, authenticator)
		}
	}
	return authenticators
}

func (s *Server) httpAuthenticate(w http.ResponseWriter, r *http.Request, options ServerAuthOptions) (User, error) {
	err401 := errors.New("wrong login/password")
	if len(s.authenticators) == 0 {
		return User{}, errors.New("authentication of clients is required but not supported")
	}
	for _, auther := range s.authenticators {
		if len(r.Header["Authorization"]) == 0 {
			err := auther.GetChallenge(w, r)
			if err != nil {
				return User{}, err
			}
		} else {
			user, err := auther.CheckResponse(w, r)
			if err != nil {
				return User{}, err
			}
			if user.Name != "" || user.HaveUid {
				context.Set(r, AuthnUser, user)
				return user, nil
			}
		}
	}
	if len(r.Header["Authorization"]) == 0 {
		logrus.Info("rejecting unauthenticated request")
	} else {
		logrus.Error("authentication failed for request")
	}
	return User{}, err401
}
