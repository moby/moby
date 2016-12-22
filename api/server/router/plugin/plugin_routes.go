package plugin

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

func parseHeaders(headers http.Header) (map[string][]string, *types.AuthConfig) {

	metaHeaders := map[string][]string{}
	for k, v := range headers {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}

	// Get X-Registry-Auth
	authEncoded := headers.Get("X-Registry-Auth")
	authConfig := &types.AuthConfig{}
	if authEncoded != "" {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
			authConfig = &types.AuthConfig{}
		}
	}

	return metaHeaders, authConfig
}

func (pr *pluginRouter) getPrivileges(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	metaHeaders, authConfig := parseHeaders(r.Header)

	privileges, err := pr.backend.Privileges(r.FormValue("name"), metaHeaders, authConfig)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, privileges)
}

func (pr *pluginRouter) pullPlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var privileges types.PluginPrivileges
	if err := json.NewDecoder(r.Body).Decode(&privileges); err != nil {
		return err
	}

	metaHeaders, authConfig := parseHeaders(r.Header)

	if err := pr.backend.Pull(r.FormValue("name"), metaHeaders, authConfig, privileges); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (pr *pluginRouter) createPlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	options := &types.PluginCreateOptions{
		RepoName: r.FormValue("name")}

	if err := pr.backend.CreateFromContext(ctx, r.Body, options); err != nil {
		return err
	}
	//TODO: send progress bar
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (pr *pluginRouter) enablePlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	timeout, err := strconv.Atoi(r.Form.Get("timeout"))
	if err != nil {
		return err
	}
	config := &types.PluginEnableConfig{Timeout: timeout}

	return pr.backend.Enable(name, config)
}

func (pr *pluginRouter) disablePlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	name := vars["name"]
	config := &types.PluginDisableConfig{
		ForceDisable: httputils.BoolValue(r, "force"),
	}

	return pr.backend.Disable(name, config)
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

	metaHeaders, authConfig := parseHeaders(r.Header)

	return pr.backend.Push(vars["name"], metaHeaders, authConfig)
}

func (pr *pluginRouter) setPlugin(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var args []string
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		return err
	}
	if err := pr.backend.Set(vars["name"], args); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
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
