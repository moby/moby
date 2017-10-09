package authorization

import (
	"fmt"
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
	mu        sync.Mutex
	plugins   []Plugin
	pg        plugingetter.PluginGetter
	saveChain func([]string) error
}

// NewMiddleware creates a new Middleware
// with a slice of plugins names.
func NewMiddleware(pg plugingetter.PluginGetter) *Middleware {
	return &Middleware{
		pg:        pg,
		saveChain: func(names []string) error { return nil },
	}
}

// SetSaveChain stores the saveChain function so that it can later be
// used to persist the chain to disk
func (m *Middleware) SetSaveChain(fn func([]string) error) {
	m.mu.Lock()
	m.saveChain = fn
	m.mu.Unlock()
}

// Type informs users of the plugin type of this chain
func (m *Middleware) Type() string {
	return AuthZApiImplements
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
	if err := m.save(plugins); err != nil {
		return err
	}
	m.plugins = plugins
	return nil
}

// RemovePlugin removes a single plugin from this authz middleware
// chain; it takes a plugin object rather than a plugin name to ensure
// that the name is valid
func (m *Middleware) RemovePlugin(plugin *v2.Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	plugins := m.plugins[:0]
	name := plugin.Name()
	for _, authPlugin := range m.plugins {
		if authPlugin.Name() == name && authPlugin.IsV1() == plugin.IsV1() {
			continue
		}
		plugins = append(plugins, authPlugin)
	}
	if err := m.save(plugins); err != nil {
		return err
	}
	m.plugins = plugins
	return nil
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
	if err := m.save(plugins); err != nil {
		return err
	}
	m.plugins = plugins
	return nil
}

// AppendPluginIfMissing appends the authorization plugin named to the
// end of the chain if it isn't already in the chain
func (m *Middleware) AppendPluginIfMissing(pluginV2 *v2.Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := pluginV2.Name()
	plugin := &authorizationPlugin{
		name:   name,
		plugin: pluginV2.Client(),
	}
	for _, p := range m.plugins {
		if p.Name() == name {
			if p.IsV1() == plugin.IsV1() {
				return nil
			}
			if p.IsV1() {
				return errV1V2Collision{"v1", "v2", name}
			}
			return errV1V2Collision{"v2", "v1", name}
		}
	}
	plugins := append(m.plugins, plugin)
	if err := m.save(plugins); err != nil {
		return err
	}
	m.plugins = plugins
	return nil
}

func (m *Middleware) save(plugins []Plugin) error {
	names := []string{}
	for _, plugin := range plugins {
		if !plugin.IsV1() {
			names = append(names, plugin.Name())
		}
	}

	return m.saveChain(names)
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

type errV1V2Collision struct {
	exists string
	other  string
	name   string
}

func (err errV1V2Collision) Error() string {
	return fmt.Sprintf("%s plugin %s already exists in authz chain, cannot add %s plugin", err.exists, err.name, err.other)
}

func (err errV1V2Collision) Conflict() {}
