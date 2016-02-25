package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/plugins"
)

type authnPlugins struct {
	plugins []string
	options map[string]string
}

// AuthnPluginRequest is the structure that we pass to an authentication
// plugin.  It contains the incoming request's method, the Scheme, Path,
// Fragment, RawQuery, and RawPath fields of the request's net.http.URL, stored
// in a map, the hostname, all of the request's headers, and the authentication
// options which were passed to the daemon at startup.
type AuthnPluginRequest struct {
	Method  string
	URL     map[string]string
	Host    string
	Header  http.Header
	Options map[string]string
}

// AuthnPluginResponse is the structure that we get back from an authentication
// plugin.  It contains information about the authenticated user (only
// consulted for CheckResponse()) and a list of header values to add to the
// HTTP response.
type AuthnPluginResponse struct {
	AuthedUser User
	Header     http.Header
}

func (a *authnPlugins) GetChallenge(w http.ResponseWriter, r *http.Request) error {
	req := AuthnPluginRequest{
		Method:  r.Method,
		Host:    r.Host,
		Header:  r.Header,
		Options: a.options,
	}
	req.URL = make(map[string]string)
	req.URL["Scheme"] = r.URL.Scheme
	req.URL["Path"] = r.URL.Path
	req.URL["Fragment"] = r.URL.Fragment
	req.URL["RawQuery"] = r.URL.RawQuery
	req.URL["RawPath"] = getRawPath(r.URL)
	for _, pname := range a.plugins {
		plugin, err := plugins.Get(pname, "Authentication")
		if err != nil {
			logrus.Errorf("Error looking up Authentication plugin %s: %v", pname, err)
			continue
		}
		resp := AuthnPluginResponse{Header: make(http.Header)}
		err = plugin.Client.Call("Authentication.GetChallenge", &req, &resp)
		if err != nil {
			logrus.Errorf("Error in Authentication plugin %s: %v", pname, err)
			continue
		}
		for header, vals := range resp.Header {
			for _, val := range vals {
				w.Header().Add(header, val)
			}
		}
	}
	return nil
}

func (a *authnPlugins) CheckResponse(w http.ResponseWriter, r *http.Request) (User, error) {
	req := AuthnPluginRequest{
		Method:  r.Method,
		Host:    r.Host,
		Header:  r.Header,
		Options: a.options,
	}
	req.URL = make(map[string]string)
	req.URL["Scheme"] = r.URL.Scheme
	req.URL["Path"] = r.URL.Path
	req.URL["Fragment"] = r.URL.Fragment
	req.URL["RawQuery"] = r.URL.RawQuery
	req.URL["RawPath"] = getRawPath(r.URL)
	for _, pname := range a.plugins {
		plugin, err := plugins.Get(pname, "Authentication")
		if err != nil {
			logrus.Errorf("Error looking up Authentication plugin %s: %v", pname, err)
			continue
		}
		resp := AuthnPluginResponse{Header: make(http.Header)}
		err = plugin.Client.Call("Authentication.CheckResponse", &req, &resp)
		if err != nil {
			logrus.Errorf("Error in Authentication plugin %s: %v", pname, err)
			continue
		}
		for header, vals := range resp.Header {
			for _, val := range vals {
				w.Header().Add(header, val)
			}
		}
		if resp.AuthedUser.Name != "" || resp.AuthedUser.HaveUID {
			if resp.AuthedUser.Scheme == "" {
				resp.AuthedUser.Scheme = pname
			}
			return resp.AuthedUser, nil
		}
	}
	return User{}, nil
}

func createAuthnPlugins(options map[string]string) Authenticator {
	plugins, ok := options["plugins"]
	if ok && plugins != "" {
		return &authnPlugins{plugins: strings.Split(plugins, ","), options: options}
	}
	return nil
}

func validatePluginsOption(option string) (string, error) {
	if strings.HasPrefix(option, "plugins=") {
		return option, nil
	}
	return "", fmt.Errorf("invalid authentication option: %s", option)
}

func init() {
	RegisterAuthenticator(createAuthnPlugins)
	opts.RegisterAuthnOptionValidater(validatePluginsOption)
}
