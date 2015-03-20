package registry

import (
	"bytes"
	"crypto/sha256"
	// this is required for some certificates
	_ "crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/utils"
)

type Session struct {
	authConfig    *AuthConfig
	reqFactory    *utils.HTTPRequestFactory
	indexEndpoint *Endpoint
	jar           *cookiejar.Jar
	timeout       TimeoutType
}

func NewSession(authConfig *AuthConfig, factory *utils.HTTPRequestFactory, endpoint *Endpoint, timeout bool) (r *Session, err error) {
	r = &Session{
		authConfig:    authConfig,
		indexEndpoint: endpoint,
	}

	if timeout {
		r.timeout = ReceiveTimeout
	}

	r.jar, err = cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	// If we're working with a standalone private registry over HTTPS, send Basic Auth headers
	// alongside our requests.
	if r.indexEndpoint.VersionString(1) != IndexServerAddress() && r.indexEndpoint.URL.Scheme == "https" {
		info, err := r.indexEndpoint.Ping()
		if err != nil {
			return nil, err
		}
		if info.Standalone {
			log.Debugf("Endpoint %s is eligible for private registry. Enabling decorator.", r.indexEndpoint.String())
			dec := utils.NewHTTPAuthDecorator(authConfig.Username, authConfig.Password)
			factory.AddDecorator(dec)
		}
	}

	r.reqFactory = factory
	return r, nil
}

func (r *Session) doRequest(req *http.Request) (*http.Response, *http.Client, error) {
	return doRequest(req, r.jar, r.timeout, r.indexEndpoint.IsSecure)
}

// Retrieve the history of a given image from the Registry.
// Return a list of the parent's json (requested image included)
func (r *Session) GetRemoteHistory(imgID, registry string, token []string) ([]string, error) {
	req, err := r.reqFactory.NewRequest("GET", registry+"images/"+imgID+"/ancestry", nil)
	if err != nil {
		return nil, err
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
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

	log.Debugf("Ancestry: %s", jsonString)
	history := new([]string)
	if err := json.Unmarshal(jsonString, history); err != nil {
		return nil, err
	}
	return *history, nil
}

// Check if an image exists in the Registry
func (r *Session) LookupRemoteImage(imgID, registry string, token []string) error {
	req, err := r.reqFactory.NewRequest("GET", registry+"images/"+imgID+"/json", nil)
	if err != nil {
		return err
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		return utils.NewHTTPRequestError(fmt.Sprintf("HTTP code %d", res.StatusCode), res)
	}
	return nil
}

// Retrieve an image from the Registry.
func (r *Session) GetRemoteImageJSON(imgID, registry string, token []string) ([]byte, int, error) {
	// Get the JSON
	req, err := r.reqFactory.NewRequest("GET", registry+"images/"+imgID+"/json", nil)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to download json: %s", err)
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to download json: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, -1, utils.NewHTTPRequestError(fmt.Sprintf("HTTP code %d", res.StatusCode), res)
	}
	// if the size header is not present, then set it to '-1'
	imageSize := -1
	if hdr := res.Header.Get("X-Docker-Size"); hdr != "" {
		imageSize, err = strconv.Atoi(hdr)
		if err != nil {
			return nil, -1, err
		}
	}

	jsonString, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, -1, fmt.Errorf("Failed to parse downloaded json: %s (%s)", err, jsonString)
	}
	return jsonString, imageSize, nil
}

func (r *Session) GetRemoteImageLayer(imgID, registry string, token []string, imgSize int64) (io.ReadCloser, error) {
	var (
		retries    = 5
		statusCode = 0
		client     *http.Client
		res        *http.Response
		imageURL   = fmt.Sprintf("%simages/%s/layer", registry, imgID)
	)

	req, err := r.reqFactory.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Error while getting from the server: %s\n", err)
	}
	setTokenAuth(req, token)
	for i := 1; i <= retries; i++ {
		statusCode = 0
		res, client, err = r.doRequest(req)
		if err != nil {
			log.Debugf("Error contacting registry: %s", err)
			if res != nil {
				if res.Body != nil {
					res.Body.Close()
				}
				statusCode = res.StatusCode
			}
			if i == retries {
				return nil, fmt.Errorf("Server error: Status %d while fetching image layer (%s)",
					statusCode, imgID)
			}
			time.Sleep(time.Duration(i) * 5 * time.Second)
			continue
		}
		break
	}

	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("Server error: Status %d while fetching image layer (%s)",
			res.StatusCode, imgID)
	}

	if res.Header.Get("Accept-Ranges") == "bytes" && imgSize > 0 {
		log.Debugf("server supports resume")
		return httputils.ResumableRequestReaderWithInitialResponse(client, req, 5, imgSize, res), nil
	}
	log.Debugf("server doesn't support resume")
	return res.Body, nil
}

func (r *Session) GetRemoteTags(registries []string, repository string, token []string) (map[string]string, error) {
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
		res, _, err := r.doRequest(req)
		if err != nil {
			return nil, err
		}

		log.Debugf("Got status code %d from %s", res.StatusCode, endpoint)
		defer res.Body.Close()

		if res.StatusCode != 200 && res.StatusCode != 404 {
			continue
		} else if res.StatusCode == 404 {
			return nil, fmt.Errorf("Repository not found")
		}

		result := make(map[string]string)
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, fmt.Errorf("Could not reach any registry endpoint")
}

func buildEndpointsList(headers []string, indexEp string) ([]string, error) {
	var endpoints []string
	parsedURL, err := url.Parse(indexEp)
	if err != nil {
		return nil, err
	}
	var urlScheme = parsedURL.Scheme
	// The Registry's URL scheme has to match the Index'
	for _, ep := range headers {
		epList := strings.Split(ep, ",")
		for _, epListElement := range epList {
			endpoints = append(
				endpoints,
				fmt.Sprintf("%s://%s/v1/", urlScheme, strings.TrimSpace(epListElement)))
		}
	}
	return endpoints, nil
}

func (r *Session) GetRepositoryData(remote string) (*RepositoryData, error) {
	repositoryTarget := fmt.Sprintf("%srepositories/%s/images", r.indexEndpoint.VersionString(1), remote)

	log.Debugf("[registry] Calling GET %s", repositoryTarget)

	req, err := r.reqFactory.NewRequest("GET", repositoryTarget, nil)
	if err != nil {
		return nil, err
	}
	if r.authConfig != nil && len(r.authConfig.Username) > 0 {
		req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
	}
	req.Header.Set("X-Docker-Token", "true")

	res, _, err := r.doRequest(req)
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
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Debugf("Error reading response body: %s", err)
		}
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("Error: Status %d trying to pull repository %s: %q", res.StatusCode, remote, errBody), res)
	}

	var tokens []string
	if res.Header.Get("X-Docker-Token") != "" {
		tokens = res.Header["X-Docker-Token"]
	}

	var endpoints []string
	if res.Header.Get("X-Docker-Endpoints") != "" {
		endpoints, err = buildEndpointsList(res.Header["X-Docker-Endpoints"], r.indexEndpoint.VersionString(1))
		if err != nil {
			return nil, err
		}
	} else {
		// Assume the endpoint is on the same host
		endpoints = append(endpoints, fmt.Sprintf("%s://%s/v1/", r.indexEndpoint.URL.Scheme, req.URL.Host))
	}

	remoteChecksums := []*ImgData{}
	if err := json.NewDecoder(res.Body).Decode(&remoteChecksums); err != nil {
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

func (r *Session) PushImageChecksumRegistry(imgData *ImgData, registry string, token []string) error {

	log.Debugf("[registry] Calling PUT %s", registry+"images/"+imgData.ID+"/checksum")

	req, err := r.reqFactory.NewRequest("PUT", registry+"images/"+imgData.ID+"/checksum", nil)
	if err != nil {
		return err
	}
	setTokenAuth(req, token)
	req.Header.Set("X-Docker-Checksum", imgData.Checksum)
	req.Header.Set("X-Docker-Checksum-Payload", imgData.ChecksumPayload)

	res, _, err := r.doRequest(req)
	if err != nil {
		return fmt.Errorf("Failed to upload metadata: %s", err)
	}
	defer res.Body.Close()
	if len(res.Cookies()) > 0 {
		r.jar.SetCookies(req.URL, res.Cookies())
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
		return fmt.Errorf("HTTP code %d while uploading metadata: %q", res.StatusCode, errBody)
	}
	return nil
}

// Push a local image to the registry
func (r *Session) PushImageJSONRegistry(imgData *ImgData, jsonRaw []byte, registry string, token []string) error {

	log.Debugf("[registry] Calling PUT %s", registry+"images/"+imgData.ID+"/json")

	req, err := r.reqFactory.NewRequest("PUT", registry+"images/"+imgData.ID+"/json", bytes.NewReader(jsonRaw))
	if err != nil {
		return err
	}
	req.Header.Add("Content-type", "application/json")
	setTokenAuth(req, token)

	res, _, err := r.doRequest(req)
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
		return utils.NewHTTPRequestError(fmt.Sprintf("HTTP code %d while uploading metadata: %q", res.StatusCode, errBody), res)
	}
	return nil
}

func (r *Session) PushImageLayerRegistry(imgID string, layer io.Reader, registry string, token []string, jsonRaw []byte) (checksum string, checksumPayload string, err error) {

	log.Debugf("[registry] Calling PUT %s", registry+"images/"+imgID+"/layer")

	tarsumLayer, err := tarsum.NewTarSum(layer, false, tarsum.Version0)
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	h.Write(jsonRaw)
	h.Write([]byte{'\n'})
	checksumLayer := io.TeeReader(tarsumLayer, h)

	req, err := r.reqFactory.NewRequest("PUT", registry+"images/"+imgID+"/layer", checksumLayer)
	if err != nil {
		return "", "", err
	}
	req.Header.Add("Content-Type", "application/octet-stream")
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
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
		return "", "", utils.NewHTTPRequestError(fmt.Sprintf("Received HTTP code %d while uploading layer: %q", res.StatusCode, errBody), res)
	}

	checksumPayload = "sha256:" + hex.EncodeToString(h.Sum(nil))
	return tarsumLayer.Sum(jsonRaw), checksumPayload, nil
}

// push a tag on the registry.
// Remote has the format '<user>/<repo>
func (r *Session) PushRegistryTag(remote, revision, tag, registry string, token []string) error {
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
	res, _, err := r.doRequest(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return utils.NewHTTPRequestError(fmt.Sprintf("Internal server error: %d trying to push tag %s on %s", res.StatusCode, tag, remote), res)
	}
	return nil
}

func (r *Session) PushImageJSONIndex(remote string, imgList []*ImgData, validate bool, regs []string) (*RepositoryData, error) {
	cleanImgList := []*ImgData{}
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
	u := fmt.Sprintf("%srepositories/%s/%s", r.indexEndpoint.VersionString(1), remote, suffix)
	log.Debugf("[registry] PUT %s", u)
	log.Debugf("Image list pushed to index:\n%s", imgListJSON)
	headers := map[string][]string{
		"Content-type":   {"application/json"},
		"X-Docker-Token": {"true"},
	}
	if validate {
		headers["X-Docker-Endpoints"] = regs
	}

	// Redirect if necessary
	var res *http.Response
	for {
		if res, err = r.putImageRequest(u, headers, imgListJSON); err != nil {
			return nil, err
		}
		if !shouldRedirect(res) {
			break
		}
		res.Body.Close()
		u = res.Header.Get("Location")
		log.Debugf("Redirected to %s", u)
	}
	defer res.Body.Close()

	var tokens, endpoints []string
	if !validate {
		if res.StatusCode != 200 && res.StatusCode != 201 {
			errBody, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Debugf("Error reading response body: %s", err)
			}
			return nil, utils.NewHTTPRequestError(fmt.Sprintf("Error: Status %d trying to push repository %s: %q", res.StatusCode, remote, errBody), res)
		}
		if res.Header.Get("X-Docker-Token") != "" {
			tokens = res.Header["X-Docker-Token"]
			log.Debugf("Auth token: %v", tokens)
		} else {
			return nil, fmt.Errorf("Index response didn't contain an access token")
		}

		if res.Header.Get("X-Docker-Endpoints") != "" {
			endpoints, err = buildEndpointsList(res.Header["X-Docker-Endpoints"], r.indexEndpoint.VersionString(1))
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("Index response didn't contain any endpoints")
		}
	}
	if validate {
		if res.StatusCode != 204 {
			errBody, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Debugf("Error reading response body: %s", err)
			}
			return nil, utils.NewHTTPRequestError(fmt.Sprintf("Error: Status %d trying to push checksums %s: %q", res.StatusCode, remote, errBody), res)
		}
	}

	return &RepositoryData{
		Tokens:    tokens,
		Endpoints: endpoints,
	}, nil
}

func (r *Session) putImageRequest(u string, headers map[string][]string, body []byte) (*http.Response, error) {
	req, err := r.reqFactory.NewRequest("PUT", u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
	req.ContentLength = int64(len(body))
	for k, v := range headers {
		req.Header[k] = v
	}
	response, _, err := r.doRequest(req)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func shouldRedirect(response *http.Response) bool {
	return response.StatusCode >= 300 && response.StatusCode < 400
}

func (r *Session) SearchRepositories(term string) (*SearchResults, error) {
	log.Debugf("Index server: %s", r.indexEndpoint)
	u := r.indexEndpoint.VersionString(1) + "search?q=" + url.QueryEscape(term)
	req, err := r.reqFactory.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	if r.authConfig != nil && len(r.authConfig.Username) > 0 {
		req.SetBasicAuth(r.authConfig.Username, r.authConfig.Password)
	}
	req.Header.Set("X-Docker-Token", "true")
	res, _, err := r.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("Unexpected status code %d", res.StatusCode), res)
	}
	result := new(SearchResults)
	err = json.NewDecoder(res.Body).Decode(result)
	return result, err
}

func (r *Session) GetAuthConfig(withPasswd bool) *AuthConfig {
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

func setTokenAuth(req *http.Request, token []string) {
	if req.Header.Get("Authorization") == "" { // Don't override
		req.Header.Set("Authorization", "Token "+strings.Join(token, ","))
	}
}
