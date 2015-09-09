package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/plugins"
)

// CertmapPluginRequest is the structure that we pass to an authentication
// plugin.  It contains the incoming request's method, selected portions of the
// request's net.http.URL, the hostname, and all of the request's headers.
type CertmapPluginRequest struct {
	Certificate []byte
	Options     map[string]string
	Method      string
	Host        string
	Header      http.Header
	URL         map[string]string
}

// CertmapPluginResponse is the structure that we get back from an authentication
// plugin.  It contains information about the authenticated user (only
// consulted for CheckResponse()) and a list of header values to add to the
// HTTP response.
type CertmapPluginResponse struct {
	AuthedUser User
	Header     http.Header
}

func getUserFromTLSClientCertificate(w http.ResponseWriter, r *http.Request, options map[string]string) (plugin string, user User) {
	mappers, ok := options["certmap"]
	if !ok || mappers == "" {
		return "", User{}
	}
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 && r.TLS.PeerCertificates[0] != nil {
		req := CertmapPluginRequest{
			Certificate: r.TLS.PeerCertificates[0].Raw,
			Options:     options,
			Method:      r.Method,
			Host:        r.Host,
			Header:      r.Header,
		}
		req.URL = make(map[string]string)
		req.URL["Scheme"] = r.URL.Scheme
		req.URL["Path"] = r.URL.Path
		req.URL["Fragment"] = r.URL.Fragment
		req.URL["RawQuery"] = r.URL.RawQuery
		req.URL["RawPath"] = getRawPath(r.URL)
		for _, pname := range strings.Split(mappers, ",") {
			plugin, err := plugins.Get(pname, "ClientCertificateMapper")
			if err != nil {
				logrus.Errorf("Error looking up ClientCertificateMapper plugin %s: %v", pname, err)
				continue
			}
			resp := CertmapPluginResponse{Header: make(http.Header)}
			err = plugin.Client.Call("ClientCertificateMapper.MapClientCertificateToUser", &req, &resp)
			if err != nil {
				logrus.Errorf("Error in ClientCertificateMapper plugin %s: %v", pname, err)
				continue
			}
			for header, vals := range resp.Header {
				for _, val := range vals {
					w.Header().Add(header, val)
				}
			}
			if resp.AuthedUser.Name != "" || resp.AuthedUser.HaveUID {
				resp.AuthedUser.Scheme = "External"
				return pname, resp.AuthedUser
			}
		}
	}
	return "", User{}
}

func validateCertmapOption(option string) (string, error) {
	if strings.HasPrefix(option, "certmap=") {
		return option, nil
	}
	return "", fmt.Errorf("invalid authentication option: %s", option)
}

func init() {
	opts.RegisterAuthnOptionValidater(validateCertmapOption)
}
