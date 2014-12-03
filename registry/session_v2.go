package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"
	"github.com/gorilla/mux"
)

func newV2RegistryRouter() *mux.Router {
	router := mux.NewRouter()

	v2Router := router.PathPrefix("/v2/").Subrouter()

	// Version Info
	v2Router.Path("/version").Name("version")

	// Image Manifests
	v2Router.Path("/manifest/{imagename:[a-z0-9-._/]+}/{tagname:[a-zA-Z0-9-._]+}").Name("manifests")

	// List Image Tags
	v2Router.Path("/tags/{imagename:[a-z0-9-._/]+}").Name("tags")

	// Download a blob
	v2Router.Path("/blob/{imagename:[a-z0-9-._/]+}/{sumtype:[a-z0-9._+-]+}/{sum:[a-fA-F0-9]{4,}}").Name("downloadBlob")

	// Upload a blob
	v2Router.Path("/blob/{imagename:[a-z0-9-._/]+}/{sumtype:[a-z0-9._+-]+}").Name("uploadBlob")

	// Mounting a blob in an image
	v2Router.Path("/mountblob/{imagename:[a-z0-9-._/]+}/{sumtype:[a-z0-9._+-]+}/{sum:[a-fA-F0-9]{4,}}").Name("mountBlob")

	return router
}

// APIVersion2 /v2/
var v2HTTPRoutes = newV2RegistryRouter()

func getV2URL(e *Endpoint, routeName string, vars map[string]string) (*url.URL, error) {
	route := v2HTTPRoutes.Get(routeName)
	if route == nil {
		return nil, fmt.Errorf("unknown regisry v2 route name: %q", routeName)
	}

	varReplace := make([]string, 0, len(vars)*2)
	for key, val := range vars {
		varReplace = append(varReplace, key, val)
	}

	routePath, err := route.URLPath(varReplace...)
	if err != nil {
		return nil, fmt.Errorf("unable to make registry route %q with vars %v: %s", routeName, vars, err)
	}
	u, err := url.Parse(REGISTRYSERVER)
	if err != nil {
		return nil, fmt.Errorf("invalid registry url: %s", err)
	}

	return &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   routePath.Path,
	}, nil
}

// V2 Provenance POC

func (r *Session) GetV2Version(token []string) (*RegistryInfo, error) {
	routeURL, err := getV2URL(r.indexEndpoint, "version", nil)
	if err != nil {
		return nil, err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())

	req, err := r.reqFactory.NewRequest(method, routeURL.String(), nil)
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
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d fetching Version", res.StatusCode), res)
	}

	decoder := json.NewDecoder(res.Body)
	versionInfo := new(RegistryInfo)

	err = decoder.Decode(versionInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to decode GetV2Version JSON response: %s", err)
	}

	return versionInfo, nil
}

//
// 1) Check if TarSum of each layer exists /v2/
//  1.a) if 200, continue
//  1.b) if 300, then push the
//  1.c) if anything else, err
// 2) PUT the created/signed manifest
//
func (r *Session) GetV2ImageManifest(imageName, tagName string, token []string) ([]byte, error) {
	vars := map[string]string{
		"imagename": imageName,
		"tagname":   tagName,
	}

	routeURL, err := getV2URL(r.indexEndpoint, "manifests", vars)
	if err != nil {
		return nil, err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())

	req, err := r.reqFactory.NewRequest(method, routeURL.String(), nil)
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
		} else if res.StatusCode == 404 {
			return nil, ErrDoesNotExist
		}
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to fetch for %s:%s", res.StatusCode, imageName, tagName), res)
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Error while reading the http response: %s", err)
	}
	return buf, nil
}

// - Succeeded to mount for this image scope
// - Failed with no error (So continue to Push the Blob)
// - Failed with error
func (r *Session) PostV2ImageMountBlob(imageName, sumType, sum string, token []string) (bool, error) {
	vars := map[string]string{
		"imagename": imageName,
		"sumtype":   sumType,
		"sum":       sum,
	}

	routeURL, err := getV2URL(r.indexEndpoint, "mountBlob", vars)
	if err != nil {
		return false, err
	}

	method := "POST"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())

	req, err := r.reqFactory.NewRequest(method, routeURL.String(), nil)
	if err != nil {
		return false, err
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
	if err != nil {
		return false, err
	}
	res.Body.Close() // close early, since we're not needing a body on this call .. yet?
	switch res.StatusCode {
	case 200:
		// return something indicating no push needed
		return true, nil
	case 300:
		// return something indicating blob push needed
		return false, nil
	}
	return false, fmt.Errorf("Failed to mount %q - %s:%s : %d", imageName, sumType, sum, res.StatusCode)
}

func (r *Session) GetV2ImageBlob(imageName, sumType, sum string, blobWrtr io.Writer, token []string) error {
	vars := map[string]string{
		"imagename": imageName,
		"sumtype":   sumType,
		"sum":       sum,
	}

	routeURL, err := getV2URL(r.indexEndpoint, "downloadBlob", vars)
	if err != nil {
		return err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())
	req, err := r.reqFactory.NewRequest(method, routeURL.String(), nil)
	if err != nil {
		return err
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == 401 {
			return errLoginRequired
		}
		return utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to pull %s blob", res.StatusCode, imageName), res)
	}

	_, err = io.Copy(blobWrtr, res.Body)
	return err
}

func (r *Session) GetV2ImageBlobReader(imageName, sumType, sum string, token []string) (io.ReadCloser, int64, error) {
	vars := map[string]string{
		"imagename": imageName,
		"sumtype":   sumType,
		"sum":       sum,
	}

	routeURL, err := getV2URL(r.indexEndpoint, "downloadBlob", vars)
	if err != nil {
		return nil, 0, err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())
	req, err := r.reqFactory.NewRequest(method, routeURL.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
	if err != nil {
		return nil, 0, err
	}
	if res.StatusCode != 200 {
		if res.StatusCode == 401 {
			return nil, 0, errLoginRequired
		}
		return nil, 0, utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to pull %s blob", res.StatusCode, imageName), res)
	}
	lenStr := res.Header.Get("Content-Length")
	l, err := strconv.ParseInt(lenStr, 10, 64)
	if err != nil {
		return nil, 0, err
	}

	return res.Body, l, err
}

// Push the image to the server for storage.
// 'layer' is an uncompressed reader of the blob to be pushed.
// The server will generate it's own checksum calculation.
func (r *Session) PutV2ImageBlob(imageName, sumType string, blobRdr io.Reader, token []string) (serverChecksum string, err error) {
	vars := map[string]string{
		"imagename": imageName,
		"sumtype":   sumType,
	}

	routeURL, err := getV2URL(r.indexEndpoint, "uploadBlob", vars)
	if err != nil {
		return "", err
	}

	method := "PUT"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())
	req, err := r.reqFactory.NewRequest(method, routeURL.String(), blobRdr)
	if err != nil {
		return "", err
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		if res.StatusCode == 401 {
			return "", errLoginRequired
		}
		return "", utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to push %s blob", res.StatusCode, imageName), res)
	}

	type sumReturn struct {
		Checksum string `json:"checksum"`
	}

	decoder := json.NewDecoder(res.Body)
	var sumInfo sumReturn

	err = decoder.Decode(&sumInfo)
	if err != nil {
		return "", fmt.Errorf("unable to decode PutV2ImageBlob JSON response: %s", err)
	}

	// XXX this is a json struct from the registry, with its checksum
	return sumInfo.Checksum, nil
}

// Finally Push the (signed) manifest of the blobs we've just pushed
func (r *Session) PutV2ImageManifest(imageName, tagName string, manifestRdr io.Reader, token []string) error {
	vars := map[string]string{
		"imagename": imageName,
		"tagname":   tagName,
	}

	routeURL, err := getV2URL(r.indexEndpoint, "manifests", vars)
	if err != nil {
		return err
	}

	method := "PUT"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())
	req, err := r.reqFactory.NewRequest(method, routeURL.String(), manifestRdr)
	if err != nil {
		return err
	}
	setTokenAuth(req, token)
	res, _, err := r.doRequest(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != 201 {
		if res.StatusCode == 401 {
			return errLoginRequired
		}
		return utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to push %s:%s manifest", res.StatusCode, imageName, tagName), res)
	}

	return nil
}

// Given a repository name, returns a json array of string tags
func (r *Session) GetV2RemoteTags(imageName string, token []string) ([]string, error) {
	vars := map[string]string{
		"imagename": imageName,
	}

	routeURL, err := getV2URL(r.indexEndpoint, "tags", vars)
	if err != nil {
		return nil, err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL.String())

	req, err := r.reqFactory.NewRequest(method, routeURL.String(), nil)
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
		} else if res.StatusCode == 404 {
			return nil, ErrDoesNotExist
		}
		return nil, utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to fetch for %s", res.StatusCode, imageName), res)
	}

	decoder := json.NewDecoder(res.Body)
	var tags []string
	err = decoder.Decode(&tags)
	if err != nil {
		return nil, fmt.Errorf("Error while decoding the http response: %s", err)
	}
	return tags, nil
}
