package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/registry/v2"
	"github.com/docker/docker/utils"
)

func getV2Builder(e *Endpoint) *v2.URLBuilder {
	if e.URLBuilder == nil {
		e.URLBuilder = v2.NewURLBuilder(e.URL)
	}
	return e.URLBuilder
}

// GetV2Authorization gets the authorization needed to the given image
// If readonly access is requested, then only the authorization may
// only be used for Get operations.
func (r *Session) GetV2Authorization(imageName string, readOnly bool) (auth *RequestAuthorization, err error) {
	scopes := []string{"pull"}
	if !readOnly {
		scopes = append(scopes, "push")
	}

	var registry *Endpoint
	if r.indexEndpoint.String() == IndexServerAddress() {
		registry, err = newEndpoint(REGISTRYSERVER, true)
		if err != nil {
			return
		}
		err = validateEndpoint(registry)
		if err != nil {
			return
		}
	} else {
		registry = r.indexEndpoint
	}
	registry.URLBuilder = v2.NewURLBuilder(registry.URL)
	r.indexEndpoint = registry

	log.Debugf("Getting authorization for %s %s", imageName, scopes)
	return NewRequestAuthorization(r.GetAuthConfig(true), registry, "repository", imageName, scopes), nil
}

//
// 1) Check if TarSum of each layer exists /v2/
//  1.a) if 200, continue
//  1.b) if 300, then push the
//  1.c) if anything else, err
// 2) PUT the created/signed manifest
//
func (r *Session) GetV2ImageManifest(imageName, tagName string, auth *RequestAuthorization) ([]byte, error) {
	routeURL, err := getV2Builder(r.indexEndpoint).BuildManifestURL(imageName, tagName)
	if err != nil {
		return nil, err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL)

	req, err := r.reqFactory.NewRequest(method, routeURL, nil)
	if err != nil {
		return nil, err
	}
	if err := auth.Authorize(req); err != nil {
		return nil, err
	}
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
func (r *Session) PostV2ImageMountBlob(imageName, sumType, sum string, auth *RequestAuthorization) (bool, error) {
	routeURL, err := getV2Builder(r.indexEndpoint).BuildBlobURL(imageName, sumType+":"+sum)
	if err != nil {
		return false, err
	}

	method := "HEAD"
	log.Debugf("[registry] Calling %q %s", method, routeURL)

	req, err := r.reqFactory.NewRequest(method, routeURL, nil)
	if err != nil {
		return false, err
	}
	if err := auth.Authorize(req); err != nil {
		return false, err
	}
	res, _, err := r.doRequest(req)
	if err != nil {
		return false, err
	}
	res.Body.Close() // close early, since we're not needing a body on this call .. yet?
	switch res.StatusCode {
	case 200:
		// return something indicating no push needed
		return true, nil
	case 404:
		// return something indicating blob push needed
		return false, nil
	}
	return false, fmt.Errorf("Failed to mount %q - %s:%s : %d", imageName, sumType, sum, res.StatusCode)
}

func (r *Session) GetV2ImageBlob(imageName, sumType, sum string, blobWrtr io.Writer, auth *RequestAuthorization) error {
	routeURL, err := getV2Builder(r.indexEndpoint).BuildBlobURL(imageName, sumType+":"+sum)
	if err != nil {
		return err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL)
	req, err := r.reqFactory.NewRequest(method, routeURL, nil)
	if err != nil {
		return err
	}
	if err := auth.Authorize(req); err != nil {
		return err
	}
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

func (r *Session) GetV2ImageBlobReader(imageName, sumType, sum string, auth *RequestAuthorization) (io.ReadCloser, int64, error) {
	routeURL, err := getV2Builder(r.indexEndpoint).BuildBlobURL(imageName, sumType+":"+sum)
	if err != nil {
		return nil, 0, err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL)
	req, err := r.reqFactory.NewRequest(method, routeURL, nil)
	if err != nil {
		return nil, 0, err
	}
	if err := auth.Authorize(req); err != nil {
		return nil, 0, err
	}
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
func (r *Session) PutV2ImageBlob(imageName, sumType, sumStr string, blobRdr io.Reader, auth *RequestAuthorization) error {
	routeURL, err := getV2Builder(r.indexEndpoint).BuildBlobUploadURL(imageName)
	if err != nil {
		return err
	}

	log.Debugf("[registry] Calling %q %s", "POST", routeURL)
	req, err := r.reqFactory.NewRequest("POST", routeURL, nil)
	if err != nil {
		return err
	}

	if err := auth.Authorize(req); err != nil {
		return err
	}
	res, _, err := r.doRequest(req)
	if err != nil {
		return err
	}
	location := res.Header.Get("Location")

	method := "PUT"
	log.Debugf("[registry] Calling %q %s", method, location)
	req, err = r.reqFactory.NewRequest(method, location, blobRdr)
	if err != nil {
		return err
	}
	queryParams := req.URL.Query()
	queryParams.Add("digest", sumType+":"+sumStr)
	req.URL.RawQuery = queryParams.Encode()
	if err := auth.Authorize(req); err != nil {
		return err
	}
	res, _, err = r.doRequest(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 201 {
		if res.StatusCode == 401 {
			return errLoginRequired
		}
		return utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to push %s blob", res.StatusCode, imageName), res)
	}

	return nil
}

// Finally Push the (signed) manifest of the blobs we've just pushed
func (r *Session) PutV2ImageManifest(imageName, tagName string, manifestRdr io.Reader, auth *RequestAuthorization) error {
	routeURL, err := getV2Builder(r.indexEndpoint).BuildManifestURL(imageName, tagName)
	if err != nil {
		return err
	}

	method := "PUT"
	log.Debugf("[registry] Calling %q %s", method, routeURL)
	req, err := r.reqFactory.NewRequest(method, routeURL, manifestRdr)
	if err != nil {
		return err
	}
	if err := auth.Authorize(req); err != nil {
		return err
	}
	res, _, err := r.doRequest(req)
	if err != nil {
		return err
	}
	b, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == 401 {
			return errLoginRequired
		}
		log.Debugf("Unexpected response from server: %q %#v", b, res.Header)
		return utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to push %s:%s manifest", res.StatusCode, imageName, tagName), res)
	}

	return nil
}

type remoteTags struct {
	name string
	tags []string
}

// Given a repository name, returns a json array of string tags
func (r *Session) GetV2RemoteTags(imageName string, auth *RequestAuthorization) ([]string, error) {
	routeURL, err := getV2Builder(r.indexEndpoint).BuildTagsURL(imageName)
	if err != nil {
		return nil, err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL)

	req, err := r.reqFactory.NewRequest(method, routeURL, nil)
	if err != nil {
		return nil, err
	}
	if err := auth.Authorize(req); err != nil {
		return nil, err
	}
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
	var remote remoteTags
	err = decoder.Decode(&remote)
	if err != nil {
		return nil, fmt.Errorf("Error while decoding the http response: %s", err)
	}
	return remote.tags, nil
}
