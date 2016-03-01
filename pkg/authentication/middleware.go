package authentication

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
)

// NewMiddleware creates a new Authentication middleware, which provides a
// method for wrapping its passed-in handler function with an authentication
// check.
func NewMiddleware(required bool, options map[string]string, makeError func(error) error) *Authentication {
	return NewAuthentication(required, options, makeError)
}

// WrapHandler returns a method that wraps an authentication check around
// handlers that are passed to it.
func (a *Authentication) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		user, err := a.Authenticate(w, r)
		if a.required && err == nil && user.Name == "" && !user.HaveUID {
			err = fmt.Errorf("authentication failed")
		}
		if err != nil {
			return a.makeAuthError(err)
		}
		if user.Name != "" && user.HaveUID {
			logrus.Debugf("authentication succeeded for %s(uid=%d)", user.Name, user.UID)
		} else if user.Name != "" {
			logrus.Debugf("authentication succeeded for %s", user.Name)
		} else {
			logrus.Debugf("authentication succeeded for (uid=%d)", user.UID)
		}
		return handler(ctx, w, r, vars)
	}
}
