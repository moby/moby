package authorization

import (
	"net/http"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugingetter"
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
func NewMiddleware(names []string, pg plugingetter.PluginGetter) *Middleware {
	SetPluginGetter(pg)
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
		userAuthNMethod := ""

		// Default authorization using existing TLS connection credentials
		// FIXME: Non trivial authorization mechanisms (such as advanced certificate validations, kerberos support
		// and ldap) will be extracted using AuthN feature, which is tracked under:
		// https://github.com/docker/docker/pull/20883
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			user = r.TLS.PeerCertificates[0].Subject.CommonName
			userAuthNMethod = "TLS"
		}

		authCtx := NewCtx(plugins, user, userAuthNMethod, r.Method, r.RequestURI)

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
