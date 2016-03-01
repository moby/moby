package authentication

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/gorilla/context"
)

type contextKey int

// authnUser is the key to use with context.Get() to retrieve an http.Request's
// authenticated user, if the user was authenticated.  The name and value don't
// matter, but the type does.
const authnUser contextKey = iota

// An Authentication object wraps handles to the various authentication plugins
// that we're using.
type Authentication struct {
	pluginNames   []string
	plugins       map[string]*plugins.Plugin
	required      bool
	makeAuthError func(error) error
}

// Pull selected fields out of a request's URL structure and populate a request.
func (a *Authentication) makeAuthnReq(r *http.Request) *AuthnPluginAuthenticateRequest {
	req := &AuthnPluginAuthenticateRequest{}
	if r != nil {
		req.Method = r.Method
		req.Host = r.Host
		req.Header = r.Header
		req.URL = r.URL.String()
	}
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 && r.TLS.PeerCertificates[0] != nil {
		req.Certificate = r.TLS.PeerCertificates[0].Raw
	}
	return req
}

// Build a properly initialized response structure.
func makeAuthnResp() AuthnPluginAuthenticateResponse {
	return AuthnPluginAuthenticateResponse{Header: make(http.Header)}
}

// callPlugins asks the plugins what they think about the client's request.
func (a *Authentication) callPlugins(w http.ResponseWriter, r *http.Request) (User, error) {
	req := a.makeAuthnReq(r)
	headers := make(http.Header)
	for _, pname := range a.pluginNames {
		plugin, ok := a.plugins[pname]
		if !ok || plugin == nil {
			logrus.Errorf("Error looking up Authentication plugin %s", pname)
			continue
		}
		client := plugin.Client()
		if client == nil {
			logrus.Errorf("Error obtaining client connection to Authentication plugin %s", pname)
			continue
		}
		resp := makeAuthnResp()
		if err := client.Call(AuthenticationRequestName, &req, &resp); err != nil {
			logrus.Errorf("Error in Authentication plugin %s: %v", pname, err)
			continue
		}
		if resp.AuthedUser.Name != "" || resp.AuthedUser.HaveUID {
			// This plugin succeeded.  Make sure the scheme is
			// populated, and add the headers returned by this
			// plugin (and only this plugin) to the response.
			if resp.AuthedUser.Scheme == "" {
				resp.AuthedUser.Scheme = pname
			}
			for header, vals := range resp.Header {
				for _, val := range vals {
					w.Header().Add(header, val)
				}
			}
			return resp.AuthedUser, nil
		}
		// This plugin didn't succeed.  Hang on to any headers it
		// returned, which may be challenge headers.
		for header, vals := range resp.Header {
			for _, val := range vals {
				headers.Add(header, val)
			}
		}
	}
	// None of the plugins succeeded.  Return all of the headers that they
	// returned, any or all of which may be challenge headers.
	for header, vals := range headers {
		for _, val := range vals {
			w.Header().Add(header, val)
		}
	}
	return User{}, nil
}

// NewAuthentication runs through the configured list of authentication plugins
// and open connections to them.  Each plugin should implement the
// "Authentication" interface, which consists of a two entry points:
//    /Authentication.Authenticate
// It receives an AuthnPluginAuthenticateRequest and returns an
// AuthnPluginAuthenticateResponse.
//
// If the request does not include any "Authorization" headers, the plugin can
// attempt to derive the user's identity from other information in and about
// the request, and if successful, return information about the user.  If it
// fails, it can provide "WWW-Authenticate" headers to be returned to the
// client as part of a 401 response.
//
// If the request contains "Authorization" headers for a scheme which the
// plugin can check, and if the checks are successful, the plugin can return
// information about the user.  If the request contains "Authorization" headers
// for a different scheme, the plugin should do nothing.
//
//    /Authentication.SetOptions
// It receives an AuthnPluginSetOptionsRequest and returns an
// AuthnPluginSetOptionsResponse.
func NewAuthentication(required bool, options map[string]string, makeAuthError func(error) error) *Authentication {
	pnames := options["plugins"]
	names := []string{}
	handles := make(map[string]*plugins.Plugin)
	for _, name := range strings.Split(pnames, ",") {
		if name == "" || handles[name] != nil {
			continue
		}
		// Connect to the plugin.
		plugin, err := plugins.Get(name, PluginImplements)
		if err != nil {
			logrus.Errorf("Error looking up authentication plugin %s: %v", name, err)
			continue
		}
		client := plugin.Client()
		if client == nil {
			logrus.Errorf("Error obtaining client connection to Authentication plugin %s", name)
			continue
		}
		// Send it the authentication options, once.
		req := AuthnPluginSetOptionsRequest{Options: options}
		resp := AuthnPluginSetOptionsResponse{}
		if err := client.Call(SetOptionsRequestName, &req, &resp); err != nil {
			logrus.Errorf("Error in Authentication plugin %s: %v", name, err)
			continue
		}
		names = append(names, name)
		handles[name] = plugin
	}
	return &Authentication{pluginNames: names, plugins: handles, required: required, makeAuthError: makeAuthError}
}

// Authenticate checks the request for an "Authorization" header, and depending
// on whether or not it finds one, it either asks the various configured
// plugins either for the set of challenges to send so that the client will
// attempt to authenticate, or to check the contents of the headers supplied by
// a client which is attempting to authenticate.
func (a *Authentication) Authenticate(w http.ResponseWriter, r *http.Request) (User, error) {
	// Default to authenticating using part of the subject name in the
	// client's certificate, if one was supplied while doing the TLS
	// handshake.
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 && r.TLS.PeerCertificates[0] != nil {
		user := User{Name: r.TLS.PeerCertificates[0].Subject.CommonName, Scheme: "TLS"}
		if user.Name != "" {
			logrus.Infof("client is \"%s\"", user.Name)
			context.Set(r, authnUser, user)
			return user, nil
		}
	}
	// If we don't need to be authenticating, stop now, so that we don't
	// add any challenge headers to our response
	if !a.required {
		return User{}, nil
	}
	// Check if we have any configured plugins
	if len(a.pluginNames) == 0 {
		err := fmt.Errorf("no authentication plugins configured")
		return User{}, err
	}
	user, err := a.callPlugins(w, r)
	if err != nil {
		return User{}, err
	}
	if user.Name != "" || user.HaveUID {
		if user.Name != "" && user.HaveUID {
			logrus.Infof("client is \"%s\"(UID %d)", user.Name, user.UID)
		} else if user.Name != "" {
			logrus.Infof("client is \"%s\"", user.Name)
		} else {
			logrus.Infof("client is UID %d", user.UID)
		}
		context.Set(r, authnUser, user)
		return user, nil
	}
	if len(r.Header["Authorization"]) == 0 {
		err = fmt.Errorf("unauthenticated request rejected")
		logrus.Debug(err.Error())
	} else {
		err = fmt.Errorf("authentication failed for request")
		logrus.Error(err.Error())
	}
	return User{}, err
}

// GetUser reads the name of the authenticated client user, if we managed to
// authenticate one.
func GetUser(r *http.Request) (user User, ok bool) {
	user, ok = context.Get(r, authnUser).(User)
	return user, ok && (user.Name != "" || user.HaveUID)
}
