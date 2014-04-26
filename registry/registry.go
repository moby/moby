package registry

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrAlreadyExists         = errors.New("Image already exists")
	ErrInvalidRepositoryName = errors.New("Invalid repository name (ex: \"registry.domain.tld/myrepos\")")
	errLoginRequired         = errors.New("Authentication is required.")
)

func pingRegistryEndpoint(endpoint string) (bool, error) {
	if endpoint == IndexServerAddress() {
		// Skip the check, we now this one is valid
		// (and we never want to fallback to http in case of error)
		return false, nil
	}
	httpDial := func(proto string, addr string) (net.Conn, error) {
		// Set the connect timeout to 5 seconds
		conn, err := net.DialTimeout(proto, addr, time.Duration(5)*time.Second)
		if err != nil {
			return nil, err
		}
		// Set the recv timeout to 10 seconds
		conn.SetDeadline(time.Now().Add(time.Duration(10) * time.Second))
		return conn, nil
	}
	httpTransport := &http.Transport{
		Dial:  httpDial,
		Proxy: http.ProxyFromEnvironment,
	}
	client := &http.Client{Transport: httpTransport}
	resp, err := client.Get(endpoint + "_ping")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Docker-Registry-Version") == "" {
		return false, errors.New("This does not look like a Registry server (\"X-Docker-Registry-Version\" header not found in the response)")
	}

	standalone := resp.Header.Get("X-Docker-Registry-Standalone")
	utils.Debugf("Registry standalone header: '%s'", standalone)
	// If the header is absent, we assume true for compatibility with earlier
	// versions of the registry
	if standalone == "" {
		return true, nil
		// Accepted values are "true" (case-insensitive) and "1".
	} else if strings.EqualFold(standalone, "true") || standalone == "1" {
		return true, nil
	}
	// Otherwise, not standalone
	return false, nil
}

func validateRepositoryName(repositoryName string) error {
	var (
		namespace string
		name      string
	)
	nameParts := strings.SplitN(repositoryName, "/", 2)
	if len(nameParts) < 2 {
		namespace = "library"
		name = nameParts[0]
	} else {
		namespace = nameParts[0]
		name = nameParts[1]
	}
	validNamespace := regexp.MustCompile(`^([a-z0-9_]{4,30})$`)
	if !validNamespace.MatchString(namespace) {
		return fmt.Errorf("Invalid namespace name (%s), only [a-z0-9_] are allowed, size between 4 and 30", namespace)
	}
	validRepo := regexp.MustCompile(`^([a-z0-9-_.]+)$`)
	if !validRepo.MatchString(name) {
		return fmt.Errorf("Invalid repository name (%s), only [a-z0-9-_.] are allowed", name)
	}
	return nil
}

// Resolves a repository name to a hostname + name
func ResolveRepositoryName(reposName string) (string, string, error) {
	if strings.Contains(reposName, "://") {
		// It cannot contain a scheme!
		return "", "", ErrInvalidRepositoryName
	}
	nameParts := strings.SplitN(reposName, "/", 2)
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") && !strings.Contains(nameParts[0], ":") &&
		nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		err := validateRepositoryName(reposName)
		return IndexServerAddress(), reposName, err
	}
	hostname := nameParts[0]
	reposName = nameParts[1]
	if strings.Contains(hostname, "index.docker.io") {
		return "", "", fmt.Errorf("Invalid repository name, try \"%s\" instead", reposName)
	}
	if err := validateRepositoryName(reposName); err != nil {
		return "", "", err
	}

	return hostname, reposName, nil
}

// this method expands the registry name as used in the prefix of a repo
// to a full url. if it already is a url, there will be no change.
// The registry is pinged to test if it http or https
func ExpandAndVerifyRegistryUrl(hostname string) (string, error) {
	if strings.HasPrefix(hostname, "http:") || strings.HasPrefix(hostname, "https:") {
		// if there is no slash after https:// (8 characters) then we have no path in the url
		if strings.LastIndex(hostname, "/") < 9 {
			// there is no path given. Expand with default path
			hostname = hostname + "/v1/"
		}
		if _, err := pingRegistryEndpoint(hostname); err != nil {
			return "", errors.New("Invalid Registry endpoint: " + err.Error())
		}
		return hostname, nil
	}
	endpoint := fmt.Sprintf("https://%s/v1/", hostname)
	if _, err := pingRegistryEndpoint(endpoint); err != nil {
		utils.Debugf("Registry %s does not work (%s), falling back to http", endpoint, err)
		endpoint = fmt.Sprintf("http://%s/v1/", hostname)
		if _, err = pingRegistryEndpoint(endpoint); err != nil {
			//TODO: triggering highland build can be done there without "failing"
			return "", errors.New("Invalid Registry endpoint: " + err.Error())
		}
	}
	return endpoint, nil
}

func setTokenAuth(req *http.Request, token []string) {
	if req.Header.Get("Authorization") == "" { // Don't override
		req.Header.Set("Authorization", "Token "+strings.Join(token, ","))
	}
}

// Retrieve the history of a given image from the Registry.
// Return a list of the parent's json (requested image included)
func (r *Registry) GetRemoteHistory(imgID, registry string, token []string) ([]string, error) {
	req, err := r.reqFactory.NewRequest("GET", registry+"images/"+imgID+"/ancestry", nil)
	if err != nil {
		return nil, err
	}
	setTokenAuth(req, token)
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == 401 {
			return nil, errLoginRequired
		}
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to fetch remote history for %s", res.StatusCode, imgID), res)
	}

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
// TODO: This method should return the errors instead of masking them and returning false
func (r *Registry) LookupRemoteImage(imgID, registry string, token []string) bool {

	req, err := r.reqFactory.NewRequest("GET", registry+"images/"+imgID+"/json", nil)
	if err != nil {
		utils.Errorf("Error in LookupRemoteImage %s", err)
		return false
	}
	setTokenAuth(req, token)
	res, err := r.client.Do(req)
	if err != nil {
		utils.Errorf("Error in LookupRemoteImage %s", err)
		return false
	}
	res.Body.Close()
	return res.StatusCode == 200
}

// Retrieve an image from the Registry.
func (r *Registry) GetRemoteImageJSON(imgID, registry string, token []string) ([]byte, int, error) {
	// Get the JSON
	req, err := r.reqFactory.NewRequest("GET", registry+"images/"+imgID+"/json", nil)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to download json: %s", err)
	}
	setTokenAuth(req, token)
	res, err := r.client.Do(req)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to download json: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, -1, utils.NewHTTPRequestError(fmt.Sprintf("HTTP code %d", res.StatusCode), res)
	}

	imageSize, err := strconv.Atoi(res.Header.Get("X-Docker-Size"))
	if err != nil {
		return nil, -1, err
	}

	jsonString, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to parse downloaded json: %s (%s)", err, jsonString)
	}
	return jsonString, imageSize, nil
}

func (r *Registry) GetRemoteImageLayer(imgID, registry string, token []string) (io.ReadCloser, error) {
	req, err := r.reqFactory.NewRequest("GET", registry+"images/"+imgID+"/layer", nil)
	if err != nil {
		return nil, fmt.Errorf("Error while getting from the server: %s\n", err)
	}
	setTokenAuth(req, token)
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("Server error: Status %d while fetching image layer (%s)",
			res.StatusCode, imgID)
	}
	return res.Body, nil
}

func (r *Registry) GetRemoteTags(registries []string, repository string, token []string) (map[string]string, error) {
	if strings.Count(repository, "/") == 0 {
		// This will be removed once the Registry supports auto-resolution on
		// the "library" namespace
		repository = "library/" + repository
	}
	for _, host := range registries {
		endpoint := fmt.Sprintf("%srepositories/%s/tags", host, repository)
		req, err := r.reqFactory.NewRequest("GET", endpoint, nil)

		if err != nil {
			return nil, err
		}
		setTokenAuth(req, token)
		res, err := r.client.Do(req)
		if err != nil {
			return nil, err
		}

		utils.Debugf("Got status code %d from %s", res.StatusCode, endpoint)
		defer res.Body.Close()

		if res.StatusCode != 200 && res.StatusCode != 404 {
			continue
		} else if res.StatusCode == 404 {
			return nil, fmt.Errorf("Repository not found")
		}

		result := make(map[string]string)
		rawJSON, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rawJSON, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, fmt.Errorf("Could not reach any registry endpoint")
}

func (r *Registry) GetRepositoryData(remote string) (*RepositoryData, error) {
	indexEp := r.indexEndpoint
	repositoryTarget := fmt.Sprintf("%srepositories/%s/images", indexEp, remote)

	utils.Debugf("[registry] Calling GET %s", repositoryTarget)

	req, err := r.reqFactory.NewRequest("GET", repositoryTarget, nil)
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
		return nil, errLoginRequired
	}
	// TODO: Right now we're ignoring checksums in the response body.
	// In the future, we need to use them to check image validity.
	if res.StatusCode != 200 {
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("HTTP code: %d", res.StatusCode), res)
	}

	var tokens []string
	if res.Header.Get("X-Docker-Token") != "" {
		tokens = res.Header["X-Docker-Token"]
	}

	var endpoints []string
	var urlScheme = indexEp[:strings.Index(indexEp, ":")]
	if res.Header.Get("X-Docker-Endpoints") != "" {
		// The Registry's URL scheme has to match the Index'
		for _, ep := range res.Header["X-Docker-Endpoints"] {
			endpoints = append(endpoints, fmt.Sprintf("%s://%s/v1/", urlScheme, ep))
		}
	} else {
		return nil, fmt.Errorf("Index response didn't contain any endpoints")
	}

	checksumsJSON, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	remoteChecksums := []*ImgData{}
	if err := json.Unmarshal(checksumsJSON, &remoteChecksums); err != nil {
		return nil, err
	}

	// Forge a better object from the retrieved data
	imgsData := make(map[string]*ImgData)
	for _, elem := range remoteChecksums {
		imgsData[elem.ID] = elem
	}

	return &RepositoryData{
		ImgList:   imgsData,
		Endpoints: endpoints,
		Tokens:    tokens,
	}, nil
}

func (r *Registry) PushImageChecksumRegistry(imgData *ImgData, registry string, token []string) error {

	utils.Debugf("[registry] Calling PUT %s", registry+"images/"+imgData.ID+"/checksum")

	req, err := r.reqFactory.NewRequest("PUT", registry+"images/"+imgData.ID+"/checksum", nil)
	if err != nil {
		return err
	}
	setTokenAuth(req, token)
	req.Header.Set("X-Docker-Checksum", imgData.Checksum)
	req.Header.Set("X-Docker-Checksum-Payload", imgData.ChecksumPayload)

	res, err := r.client.Do(req)
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

// Push a local image to the registry
func (r *Registry) PushImageJSONRegistry(imgData *ImgData, jsonRaw []byte, registry string, token []string) error {

	utils.Debugf("[registry] Calling PUT %s", registry+"images/"+imgData.ID+"/json")

	req, err := r.reqFactory.NewRequest("PUT", registry+"images/"+imgData.ID+"/json", bytes.NewReader(jsonRaw))
	if err != nil {
		return err
	}
	req.Header.Add("Content-type", "application/json")
	setTokenAuth(req, token)

	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to upload metadata: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode == 401 && strings.HasPrefix(registry, "http://") {
		return utils.NewHTTPRequestError("HTTP code 401, Docker will not send auth headers over HTTP.", res)
	}
	if res.StatusCode != 200 {
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return utils.NewHTTPRequestError(fmt.Sprintf("HTTP code %d while uploading metadata and error when trying to parse response body: %s", res.StatusCode, err), res)
		}
		var jsonBody map[string]string
		if err := json.Unmarshal(errBody, &jsonBody); err != nil {
			errBody = []byte(err.Error())
		} else if jsonBody["error"] == "Image already exists" {
			return ErrAlreadyExists
		}
		return utils.NewHTTPRequestError(fmt.Sprintf("HTTP code %d while uploading metadata: %s", res.StatusCode, errBody), res)
	}
	return nil
}

func (r *Registry) PushImageLayerRegistry(imgID string, layer io.Reader, registry string, token []string, jsonRaw []byte) (checksum string, checksumPayload string, err error) {

	utils.Debugf("[registry] Calling PUT %s", registry+"images/"+imgID+"/layer")

	tarsumLayer := &utils.TarSum{Reader: layer}
	h := sha256.New()
	h.Write(jsonRaw)
	h.Write([]byte{'\n'})
	checksumLayer := &utils.CheckSum{Reader: tarsumLayer, Hash: h}

	req, err := r.reqFactory.NewRequest("PUT", registry+"images/"+imgID+"/layer", checksumLayer)
	if err != nil {
		return "", "", err
	}
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	setTokenAuth(req, token)
	res, err := r.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Failed to upload layer: %s", err)
	}
	if rc, ok := layer.(io.Closer); ok {
		if err := rc.Close(); err != nil {
			return "", "", err
		}
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return "", "", utils.NewHTTPRequestError(fmt.Sprintf("HTTP code %d while uploading metadata and error when trying to parse response body: %s", res.StatusCode, err), res)
		}
		return "", "", utils.NewHTTPRequestError(fmt.Sprintf("Received HTTP code %d while uploading layer: %s", res.StatusCode, errBody), res)
	}

	checksumPayload = "sha256:" + checksumLayer.Sum()
	return tarsumLayer.Sum(jsonRaw), checksumPayload, nil
}

// push a tag on the registry.
// Remote has the format '<user>/<repo>
func (r *Registry) PushRegistryTag(remote, revision, tag, registry string, token []string) error {
	// "jsonify" the string
	revision = "\"" + revision + "\""
	path := fmt.Sprintf("repositories/%s/tags/%s", remote, tag)

	req, err := r.reqFactory.NewRequest("PUT", registry+path, strings.NewReader(revision))
	if err != nil {
		return err
	}
	req.Header.Add("Content-type", "application/json")
	setTokenAuth(req, token)
	req.ContentLength = int64(len(revision))
	res, err := r.client.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return utils.NewHTTPRequestError(fmt.Sprintf("Internal server error: %d trying to push tag %s on %s", res.StatusCode, tag, remote), res)
	}
	return nil
}

func (r *Registry) PushImageJSONIndex(remote string, imgList []*ImgData, validate bool, regs []string) (*RepositoryData, error) {
	cleanImgList := []*ImgData{}
	indexEp := r.indexEndpoint

	if validate {
		for _, elem := range imgList {
			if elem.Checksum != "" {
				cleanImgList = append(cleanImgList, elem)
			}
		}
	} else {
		cleanImgList = imgList
	}

	imgListJSON, err := json.Marshal(cleanImgList)
	if err != nil {
		return nil, err
	}
	var suffix string
	if validate {
		suffix = "images"
	}
	u := fmt.Sprintf("%srepositories/%s/%s", indexEp, remote, suffix)
	utils.Debugf("[registry] PUT %s", u)
	utils.Debugf("Image list pushed to index:\n%s", imgListJSON)
	req, err := r.reqFactory.NewRequest("PUT", u, bytes.NewReader(imgListJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-type", "application/json")
	req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
	req.ContentLength = int64(len(imgListJSON))
	req.Header.Set("X-Docker-Token", "true")
	if validate {
		req.Header["X-Docker-Endpoints"] = regs
	}

	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Redirect if necessary
	for res.StatusCode >= 300 && res.StatusCode < 400 {
		utils.Debugf("Redirected to %s", res.Header.Get("Location"))
		req, err = r.reqFactory.NewRequest("PUT", res.Header.Get("Location"), bytes.NewReader(imgListJSON))
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
		req.ContentLength = int64(len(imgListJSON))
		req.Header.Set("X-Docker-Token", "true")
		if validate {
			req.Header["X-Docker-Endpoints"] = regs
		}
		res, err = r.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
	}

	var tokens, endpoints []string
	var urlScheme = indexEp[:strings.Index(indexEp, ":")]
	if !validate {
		if res.StatusCode != 200 && res.StatusCode != 201 {
			errBody, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return nil, err
			}
			return nil, utils.NewHTTPRequestError(fmt.Sprintf("Error: Status %d trying to push repository %s: %s", res.StatusCode, remote, errBody), res)
		}
		if res.Header.Get("X-Docker-Token") != "" {
			tokens = res.Header["X-Docker-Token"]
			utils.Debugf("Auth token: %v", tokens)
		} else {
			return nil, fmt.Errorf("Index response didn't contain an access token")
		}

		if res.Header.Get("X-Docker-Endpoints") != "" {
			// The Registry's URL scheme has to match the Index'
			for _, ep := range res.Header["X-Docker-Endpoints"] {
				endpoints = append(endpoints, fmt.Sprintf("%s://%s/v1/", urlScheme, ep))
			}
		} else {
			return nil, fmt.Errorf("Index response didn't contain any endpoints")
		}
	}
	if validate {
		if res.StatusCode != 204 {
			errBody, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return nil, err
			}
			return nil, utils.NewHTTPRequestError(fmt.Sprintf("Error: Status %d trying to push checksums %s: %s", res.StatusCode, remote, errBody), res)
		}
	}

	return &RepositoryData{
		Tokens:    tokens,
		Endpoints: endpoints,
	}, nil
}

func (r *Registry) SearchRepositories(term string) (*SearchResults, error) {
	utils.Debugf("Index server: %s", r.indexEndpoint)
	u := r.indexEndpoint + "search?q=" + url.QueryEscape(term)
	req, err := r.reqFactory.NewRequest("GET", u, nil)
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
	if res.StatusCode != 200 {
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("Unexepected status code %d", res.StatusCode), res)
	}
	rawData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	result := new(SearchResults)
	err = json.Unmarshal(rawData, result)
	return result, err
}

func (r *Registry) GetAuthConfig(withPasswd bool) *AuthConfig {
	password := ""
	if withPasswd {
		password = r.authConfig.Password
	}
	return &AuthConfig{
		Username: r.authConfig.Username,
		Password: password,
		Email:    r.authConfig.Email,
	}
}

type SearchResult struct {
	StarCount   int    `json:"star_count"`
	IsOfficial  bool   `json:"is_official"`
	Name        string `json:"name"`
	IsTrusted   bool   `json:"is_trusted"`
	Description string `json:"description"`
}

type SearchResults struct {
	Query      string         `json:"query"`
	NumResults int            `json:"num_results"`
	Results    []SearchResult `json:"results"`
}

type RepositoryData struct {
	ImgList   map[string]*ImgData
	Endpoints []string
	Tokens    []string
}

type ImgData struct {
	ID              string `json:"id"`
	Checksum        string `json:"checksum,omitempty"`
	ChecksumPayload string `json:"-"`
	Tag             string `json:",omitempty"`
}

type Registry struct {
	client        *http.Client
	authConfig    *AuthConfig
	reqFactory    *utils.HTTPRequestFactory
	indexEndpoint string
}

func NewRegistry(authConfig *AuthConfig, factory *utils.HTTPRequestFactory, indexEndpoint string) (r *Registry, err error) {
	httpTransport := &http.Transport{
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment,
	}

	r = &Registry{
		authConfig: authConfig,
		client: &http.Client{
			Transport: httpTransport,
		},
		indexEndpoint: indexEndpoint,
	}
	r.client.Jar, err = cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	// If we're working with a standalone private registry over HTTPS, send Basic Auth headers
	// alongside our requests.
	if indexEndpoint != IndexServerAddress() && strings.HasPrefix(indexEndpoint, "https://") {
		standalone, err := pingRegistryEndpoint(indexEndpoint)
		if err != nil {
			return nil, err
		}
		if standalone {
			utils.Debugf("Endpoint %s is eligible for private registry registry. Enabling decorator.", indexEndpoint)
			dec := utils.NewHTTPAuthDecorator(authConfig.Username, authConfig.Password)
			factory.AddDecorator(dec)
		}
	}

	r.reqFactory = factory
	return r, nil
}
