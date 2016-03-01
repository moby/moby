package authorization

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/authentication"
	"golang.org/x/net/context"
)

// Middleware uses a list of plugins to
// handle authorization in the API requests.
type Middleware struct {
	mu      sync.Mutex
	plugins []Plugin
}

// NewMiddleware creates a new Middleware
// with a slice of plugins names.
func NewMiddleware(names []string) *Middleware {
	return &Middleware{
		plugins: newPlugins(names),
	}
}

// SetPlugins sets the plugin used for authorization
func (m *Middleware) SetPlugins(names []string) {
	m.mu.Lock()
	m.plugins = newPlugins(names)
	m.mu.Unlock()
}

// WrapHandler returns a new handler function wrapping the previous one in the request chain.
func (m *Middleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {

		m.mu.Lock()
		plugins := m.plugins
		m.mu.Unlock()
		if len(plugins) == 0 {
			return handler(ctx, w, r, vars)
		}

		user := ""
		uid := ""
		userAuthNMethod := ""

		if authedUser, authenticated := authentication.GetUser(r); authenticated {
			user = authedUser.Name
			if authedUser.HaveUID {
				uid = fmt.Sprint(authedUser.UID)
			}
			userAuthNMethod = authedUser.Scheme
		}

		authCtx := NewCtx(plugins, user, uid, userAuthNMethod, r.Method, r.RequestURI)

		if err := authCtx.AuthZRequest(w, r); err != nil {
			logrus.Errorf("AuthZRequest for %s %s returned error: %s", r.Method, r.RequestURI, err)
			return err
		}

		rw := NewResponseModifier(w)

		if err := handler(ctx, rw, r, vars); err != nil {
			logrus.Errorf("Handler for %s %s returned error: %s", r.Method, r.RequestURI, err)
			return err
		}

		if err := authCtx.AuthZResponse(rw, r); err != nil {
			logrus.Errorf("AuthZResponse for %s %s returned error: %s", r.Method, r.RequestURI, err)
			return err
		}
		return nil
	}
}
