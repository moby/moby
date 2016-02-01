package client

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/registry"
)

// ImageSearch makes the docker host to search by a term in a remote registry.
// The list of results is not sorted in any fashion.
func (cli *Client) ImageSearch(options types.ImageSearchOptions, privilegeFunc RequestPrivilegeFunc) ([]registry.SearchResult, error) {
	var results []registry.SearchResult
	query := url.Values{}
	query.Set("term", options.Term)

	resp, err := cli.tryImageSearch(query, options.RegistryAuth)
	if resp.statusCode == http.StatusUnauthorized {
		newAuthHeader, privilegeErr := privilegeFunc()
		if privilegeErr != nil {
			return results, privilegeErr
		}
		resp, err = cli.tryImageSearch(query, newAuthHeader)
	}
	if err != nil {
		return results, err
	}

	err = json.NewDecoder(resp.body).Decode(&results)
	ensureReaderClosed(resp)
	return results, err
}

func (cli *Client) tryImageSearch(query url.Values, registryAuth string) (*serverResponse, error) {
	headers := map[string][]string{"X-Registry-Auth": {registryAuth}}
	return cli.get("/images/search", query, headers)
}
