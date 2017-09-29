package authorization

import (
	"net/http"
	"sync"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/plugin/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// Middleware uses a list of plugins to
// handle authorization in the API requests.
type Middleware struct {
	mu      sync.Mutex
	plugins []Plugin
	pg      plugingetter.PluginGetter
}

// NewMiddleware creates a new Middleware
// with a slice of plugins names.
func NewMiddleware(names []string, pg plugingetter.PluginGetter) (*Middleware, error) {
	plugins, err := newPlugins(pg, names)
	if err != nil {
		return nil, err
	}
	return &Middleware{
		plugins: plugins,
		pg:      pg,
	}, nil
}

func (m *Middleware) getAuthzPlugins() []Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.plugins
}

// GetPlugins gets the current authorization plugin chain
func (m *Middleware) GetPlugins() []string {
	names := []string{}
	for _, plugin := range m.getAuthzPlugins() {
		names = append(names, plugin.Name())
	}
	return names
}

// SetPlugins sets the plugin used for authorization
func (m *Middleware) SetPlugins(names []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	plugins, err := newPlugins(m.pg, names)
	if err != nil {
		return err
	}
	m.plugins = plugins
	return nil
}

// RemovePlugin removes a single plugin from this authz middleware
// chain; it takes a plugin object rather than a plugin name to ensure
// that the name is valid
func (m *Middleware) RemovePlugin(plugin *v2.Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	plugins := m.plugins[:0]
	name := plugin.Name()
	for _, authPlugin := range m.plugins {
		if authPlugin.Name() != name {
			plugins = append(plugins, authPlugin)
		}
	}
	m.plugins = plugins
}

// PrependUniqueFirst prepends the named plugins to the authz chain
// removing any duplicates after the first time a plugin is named
func (m *Middleware) PrependUniqueFirst(names []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, plugin := range m.plugins {
		names = append(names, plugin.Name())
	}
	plugins, err := newPlugins(m.pg, names)
	if err != nil {
		return err
	}
	m.plugins = plugins
	return nil
}

// AppendPluginIfMissing appends the authorization plugin named to the
// end of the chain if it isn't already in the chain
func (m *Middleware) AppendPluginIfMissing(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	plugin, err := newAuthorizationPlugin(m.pg, name)
	if err != nil {
		return err
	}
	name = plugin.Name()
	for _, p := range m.plugins {
		if p.Name() == name {
			return nil
		}
	}
	m.plugins = append(m.plugins, plugin)
	return nil
}

// WrapHandler returns a new handler function wrapping the previous one in the request chain.
func (m *Middleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		plugins := m.getAuthzPlugins()
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

		var errD error

		if errD = handler(ctx, rw, r, vars); errD != nil {
			logrus.Errorf("Handler for %s %s returned error: %s", r.Method, r.RequestURI, errD)
		}

		// There's a chance that the authCtx.plugins was updated. One of the reasons
		// this can happen is when an authzplugin is disabled.
		plugins = m.getAuthzPlugins()
		if len(plugins) == 0 {
			logrus.Debug("There are no authz plugins in the chain")
			return nil
		}

		authCtx.plugins = plugins

		if err := authCtx.AuthZResponse(rw, r); errD == nil && err != nil {
			logrus.Errorf("AuthZResponse for %s %s returned error: %s", r.Method, r.RequestURI, err)
			return err
		}

		if errD != nil {
			return errD
		}

		return nil
	}
}
