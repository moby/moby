// +build experimental

package plugin

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

func (pr *pluginRouter) pullPlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	metaHeaders := map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}

	// Get X-Registry-Auth
	authEncoded := r.Header.Get("X-Registry-Auth")
	authConfig := &types.AuthConfig{}
	if authEncoded != "" {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
			authConfig = &types.AuthConfig{}
		}
	}

	privileges, err := pr.backend.Pull(r.FormValue("name"), metaHeaders, authConfig)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, privileges)
}

func (pr *pluginRouter) enablePlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return pr.backend.Enable(vars["name"])
}

func (pr *pluginRouter) disablePlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return pr.backend.Disable(vars["name"])
}

func (pr *pluginRouter) removePlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	config := &types.PluginRmConfig{
		ForceRemove: httputils.BoolValue(r, "force"),
	}
	return pr.backend.Remove(name, config)
}

func (pr *pluginRouter) pushPlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	metaHeaders := map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}

	// Get X-Registry-Auth
	authEncoded := r.Header.Get("X-Registry-Auth")
	authConfig := &types.AuthConfig{}
	if authEncoded != "" {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
			authConfig = &types.AuthConfig{}
		}
	}
	return pr.backend.Push(vars["name"], metaHeaders, authConfig)
}

func (pr *pluginRouter) setPlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var args []string
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		return err
	}
	return pr.backend.Set(vars["name"], args)
}

func (pr *pluginRouter) listPlugins(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	l, err := pr.backend.List()
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, l)
}

func (pr *pluginRouter) inspectPlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	result, err := pr.backend.Inspect(vars["name"])
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, result)
}
