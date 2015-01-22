package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/utils"
)

type tokenResponse struct {
	Token string `json:"token"`
}

func getToken(username, password string, params map[string]string, registryEndpoint *Endpoint, factory *utils.HTTPRequestFactory) (token string, err error) {
	realm, ok := params["realm"]
	if !ok {
		return "", errors.New("no realm specified for token auth challenge")
	}

	realmURL, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("invalid token auth challenge realm: %s", err)
	}

	if realmURL.Scheme == "" {
		realmURL.Scheme = registryEndpoint.URL.Scheme
	}

	if realmURL.Scheme != "https" {
		return "", fmt.Errorf("cannot send token credentials over insecure channel: %s", realmURL)
	}

	req, err := factory.NewRequest("GET", realmURL.String(), nil)
	if err != nil {
		return "", err
	}

	reqParams := req.URL.Query()
	service := params["service"]
	scope := params["scope"]

	if service != "" {
		reqParams.Add("service", service)
	}

	for _, scopeField := range strings.Fields(scope) {
		reqParams.Add("scope", scopeField)
	}

	reqParams.Add("account", username)

	req.URL.RawQuery = reqParams.Encode()
	req.SetBasicAuth(username, password)

	client := newClient(nil, ConnectTimeout, true)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token auth attempt for registry %s: %s request failed with status: %d %s", registryEndpoint, req.URL, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	decoder := json.NewDecoder(resp.Body)

	tr := new(tokenResponse)
	if err = decoder.Decode(tr); err != nil {
		return "", fmt.Errorf("unable to decode token response: %s", err)
	}

	if tr.Token == "" {
		return "", errors.New("authorization server did not include a token in the response")
	}

	return tr.Token, nil
}
