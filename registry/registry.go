package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/utils"
	"github.com/shin-/cookiejar"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

var ErrAlreadyExists error = errors.New("Image already exists")

func doWithCookies(c *http.Client, req *http.Request) (*http.Response, error) {
	for _, cookie := range c.Jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}
	return c.Do(req)
}

// Retrieve the history of a given image from the Registry.
// Return a list of the parent's json (requested image included)
func (r *Registry) GetRemoteHistory(imgId, registry string, token []string) ([]string, error) {
	req, err := http.NewRequest("GET", registry+"/images/"+imgId+"/ancestry", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+strings.Join(token, ", "))
	res, err := r.client.Do(req)
	if err != nil || res.StatusCode != 200 {
		if res != nil {
			return nil, fmt.Errorf("Internal server error: %d trying to fetch remote history for %s", res.StatusCode, imgId)
		}
		return nil, err
	}
	defer res.Body.Close()

	jsonString, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Error while reading the http response: %s", err)
	}

	utils.Debugf("Ancestry: %s", jsonString)
	history := new([]string)
	if err := json.Unmarshal(jsonString, history); err != nil {
		return nil, err
	}
	return *history, nil
}

// Check if an image exists in the Registry
func (r *Registry) LookupRemoteImage(imgId, registry string, authConfig *auth.AuthConfig) bool {
	rt := &http.Transport{Proxy: http.ProxyFromEnvironment}

	req, err := http.NewRequest("GET", registry+"/images/"+imgId+"/json", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(authConfig.Username, authConfig.Password)
	res, err := rt.RoundTrip(req)
	return err == nil && res.StatusCode == 307
}

func (r *Registry) getImagesInRepository(repository string, authConfig *auth.AuthConfig) ([]map[string]string, error) {
	u := auth.IndexServerAddress() + "/repositories/" + repository + "/images"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	if authConfig != nil && len(authConfig.Username) > 0 {
		req.SetBasicAuth(authConfig.Username, authConfig.Password)
	}
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Repository doesn't exist yet
	if res.StatusCode == 404 {
		return nil, nil
	}

	jsonData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	imageList := []map[string]string{}
	if err := json.Unmarshal(jsonData, &imageList); err != nil {
		utils.Debugf("Body: %s (%s)\n", res.Body, u)
		return nil, err
	}

	return imageList, nil
}

// Retrieve an image from the Registry.
// Returns the Image object as well as the layer as an Archive (io.Reader)
func (r *Registry) GetRemoteImageJson(imgId, registry string, token []string) ([]byte, error) {
	// Get the Json
	req, err := http.NewRequest("GET", registry+"/images/"+imgId+"/json", nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to download json: %s", err)
	}
	req.Header.Set("Authorization", "Token "+strings.Join(token, ", "))
	res, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to download json: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP code %d", res.StatusCode)
	}
	jsonString, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse downloaded json: %s (%s)", err, jsonString)
	}
	return jsonString, nil
}

func (r *Registry) GetRemoteImageLayer(imgId, registry string, token []string) (io.ReadCloser, int, error) {
	req, err := http.NewRequest("GET", registry+"/images/"+imgId+"/layer", nil)
	if err != nil {
		return nil, -1, fmt.Errorf("Error while getting from the server: %s\n", err)
	}
	req.Header.Set("Authorization", "Token "+strings.Join(token, ", "))
	res, err := r.client.Do(req)
	if err != nil {
		return nil, -1, err
	}
	return res.Body, int(res.ContentLength), nil
}

func (r *Registry) GetRemoteTags(registries []string, repository string, token []string) (map[string]string, error) {
	if strings.Count(repository, "/") == 0 {
		// This will be removed once the Registry supports auto-resolution on
		// the "library" namespace
		repository = "library/" + repository
	}
	for _, host := range registries {
		endpoint := fmt.Sprintf("https://%s/v1/repositories/%s/tags", host, repository)
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Token "+strings.Join(token, ", "))
		res, err := r.client.Do(req)
		defer res.Body.Close()
		utils.Debugf("Got status code %d from %s", res.StatusCode, endpoint)
		if err != nil || (res.StatusCode != 200 && res.StatusCode != 404) {
			continue
		} else if res.StatusCode == 404 {
			return nil, fmt.Errorf("Repository not found")
		}

		result := make(map[string]string)

		rawJson, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rawJson, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, fmt.Errorf("Could not reach any registry endpoint")
}

func (r *Registry) GetRepositoryData(remote string) (*RepositoryData, error) {
	repositoryTarget := auth.IndexServerAddress() + "/repositories/" + remote + "/images"

	req, err := http.NewRequest("GET", repositoryTarget, nil)
	if err != nil {
		return nil, err
	}
	if r.authConfig != nil && len(r.authConfig.Username) > 0 {
		req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
	}
	req.Header.Set("X-Docker-Token", "true")

	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return nil, fmt.Errorf("Please login first (HTTP code %d)", res.StatusCode)
	}
	// TODO: Right now we're ignoring checksums in the response body.
	// In the future, we need to use them to check image validity.
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP code: %d", res.StatusCode)
	}

	var tokens []string
	if res.Header.Get("X-Docker-Token") != "" {
		tokens = res.Header["X-Docker-Token"]
	}

	var endpoints []string
	if res.Header.Get("X-Docker-Endpoints") != "" {
		endpoints = res.Header["X-Docker-Endpoints"]
	} else {
		return nil, fmt.Errorf("Index response didn't contain any endpoints")
	}

	checksumsJson, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	remoteChecksums := []*ImgData{}
	if err := json.Unmarshal(checksumsJson, &remoteChecksums); err != nil {
		return nil, err
	}

	// Forge a better object from the retrieved data
	imgsData := make(map[string]*ImgData)
	for _, elem := range remoteChecksums {
		imgsData[elem.Id] = elem
	}

	return &RepositoryData{
		ImgList:   imgsData,
		Endpoints: endpoints,
		Tokens:    tokens,
	}, nil
}

// Push a local image to the registry
func (r *Registry) PushImageJsonRegistry(imgData *ImgData, jsonRaw []byte, registry string, token []string) error {
	registry = "https://" + registry + "/v1"
	// FIXME: try json with UTF8
	req, err := http.NewRequest("PUT", registry+"/images/"+imgData.Id+"/json", strings.NewReader(string(jsonRaw)))
	if err != nil {
		return err
	}
	req.Header.Add("Content-type", "application/json")
	req.Header.Set("Authorization", "Token "+strings.Join(token, ","))
	req.Header.Set("X-Docker-Checksum", imgData.Checksum)

	utils.Debugf("Setting checksum for %s: %s", imgData.Id, imgData.Checksum)
	res, err := doWithCookies(r.client, req)
	if err != nil {
		return fmt.Errorf("Failed to upload metadata: %s", err)
	}
	defer res.Body.Close()
	if len(res.Cookies()) > 0 {
		r.client.Jar.SetCookies(req.URL, res.Cookies())
	}
	if res.StatusCode != 200 {
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("HTTP code %d while uploading metadata and error when trying to parse response body: %s", res.StatusCode, err)
		}
		var jsonBody map[string]string
		if err := json.Unmarshal(errBody, &jsonBody); err != nil {
			errBody = []byte(err.Error())
		} else if jsonBody["error"] == "Image already exists" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("HTTP code %d while uploading metadata: %s", res.StatusCode, errBody)
	}
	return nil
}

func (r *Registry) PushImageLayerRegistry(imgId string, layer io.Reader, registry string, token []string) error {
	registry = "https://" + registry + "/v1"
	req, err := http.NewRequest("PUT", registry+"/images/"+imgId+"/layer", layer)
	if err != nil {
		return err
	}
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	req.Header.Set("Authorization", "Token "+strings.Join(token, ","))
	res, err := doWithCookies(r.client, req)
	if err != nil {
		return fmt.Errorf("Failed to upload layer: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("HTTP code %d while uploading metadata and error when trying to parse response body: %s", res.StatusCode, err)
		}
		return fmt.Errorf("Received HTTP code %d while uploading layer: %s", res.StatusCode, errBody)
	}
	return nil
}

// push a tag on the registry.
// Remote has the format '<user>/<repo>
func (r *Registry) PushRegistryTag(remote, revision, tag, registry string, token []string) error {
	// "jsonify" the string
	revision = "\"" + revision + "\""
	registry = "https://" + registry + "/v1"

	req, err := http.NewRequest("PUT", registry+"/repositories/"+remote+"/tags/"+tag, strings.NewReader(revision))
	if err != nil {
		return err
	}
	req.Header.Add("Content-type", "application/json")
	req.Header.Set("Authorization", "Token "+strings.Join(token, ","))
	req.ContentLength = int64(len(revision))
	res, err := doWithCookies(r.client, req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return fmt.Errorf("Internal server error: %d trying to push tag %s on %s", res.StatusCode, tag, remote)
	}
	return nil
}

func (r *Registry) PushImageJsonIndex(remote string, imgList []*ImgData, validate bool) (*RepositoryData, error) {
	imgListJson, err := json.Marshal(imgList)
	if err != nil {
		return nil, err
	}
	var suffix string
	if validate {
		suffix = "images"
	}

	utils.Debugf("Image list pushed to index:\n%s\n", imgListJson)

	req, err := http.NewRequest("PUT", auth.IndexServerAddress()+"/repositories/"+remote+"/"+suffix, bytes.NewReader(imgListJson))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
	req.ContentLength = int64(len(imgListJson))
	req.Header.Set("X-Docker-Token", "true")

	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Redirect if necessary
	for res.StatusCode >= 300 && res.StatusCode < 400 {
		utils.Debugf("Redirected to %s\n", res.Header.Get("Location"))
		req, err = http.NewRequest("PUT", res.Header.Get("Location"), bytes.NewReader(imgListJson))
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
		req.ContentLength = int64(len(imgListJson))
		req.Header.Set("X-Docker-Token", "true")

		res, err = r.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
	}

	var tokens, endpoints []string
	if !validate {
		if res.StatusCode != 200 && res.StatusCode != 201 {
			errBody, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("Error: Status %d trying to push repository %s: %s", res.StatusCode, remote, errBody)
		}
		if res.Header.Get("X-Docker-Token") != "" {
			tokens = res.Header["X-Docker-Token"]
			utils.Debugf("Auth token: %v", tokens)
		} else {
			return nil, fmt.Errorf("Index response didn't contain an access token")
		}

		if res.Header.Get("X-Docker-Endpoints") != "" {
			endpoints = res.Header["X-Docker-Endpoints"]
		} else {
			return nil, fmt.Errorf("Index response didn't contain any endpoints")
		}
	}
	if validate {
		if res.StatusCode != 204 {
			if errBody, err := ioutil.ReadAll(res.Body); err != nil {
				return nil, err
			} else {
				return nil, fmt.Errorf("Error: Status %d trying to push checksums %s: %s", res.StatusCode, remote, errBody)
			}
		}
	}

	return &RepositoryData{
		Tokens:    tokens,
		Endpoints: endpoints,
	}, nil
}

func (r *Registry) SearchRepositories(term string) (*SearchResults, error) {
	u := auth.IndexServerAddress() + "/search?q=" + url.QueryEscape(term)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Unexepected status code %d", res.StatusCode)
	}
	rawData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	result := new(SearchResults)
	err = json.Unmarshal(rawData, result)
	return result, err
}

func (r *Registry) ResetClient(authConfig *auth.AuthConfig) {
	r.authConfig = authConfig
	r.client.Jar = cookiejar.NewCookieJar()
}

func (r *Registry) GetAuthConfig(withPasswd bool) *auth.AuthConfig {
	password := ""
	if withPasswd {
		password = r.authConfig.Password
	}
	return &auth.AuthConfig{
		Username: r.authConfig.Username,
		Password: password,
		Email:    r.authConfig.Email,
	}
}

type SearchResults struct {
	Query      string              `json:"query"`
	NumResults int                 `json:"num_results"`
	Results    []map[string]string `json:"results"`
}

type RepositoryData struct {
	ImgList   map[string]*ImgData
	Endpoints []string
	Tokens    []string
}

type ImgData struct {
	Id       string `json:"id"`
	Checksum string `json:"checksum,omitempty"`
	Tag      string `json:",omitempty"`
}

type Registry struct {
	client     *http.Client
	authConfig *auth.AuthConfig
}

func NewRegistry(root string) *Registry {
	// If the auth file does not exist, keep going
	authConfig, _ := auth.LoadConfig(root)

	r := &Registry{
		authConfig: authConfig,
		client:     &http.Client{},
	}
	r.client.Jar = cookiejar.NewCookieJar()
	return r
}
