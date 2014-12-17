package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/registry/v2"
	"github.com/docker/docker/utils"
)

var registryURLBuilder *v2.URLBuilder

func init() {
	u, err := url.Parse(REGISTRYSERVER)
	if err != nil {
		panic(fmt.Errorf("invalid registry url: %s", err))
	}
	registryURLBuilder = v2.NewURLBuilder(u)
}

func getV2Builder(e *Endpoint) *v2.URLBuilder {
	return registryURLBuilder
}

// GetV2Authorization gets the authorization needed to the given image
// If readonly access is requested, then only the authorization may
// only be used for Get operations.
func (r *Session) GetV2Authorization(imageName string, readOnly bool) (*RequestAuthorization, error) {
	scopes := []string{"pull"}
	if !readOnly {
		scopes = append(scopes, "push")
	}

	return NewRequestAuthorization(r.GetAuthConfig(true), r.indexEndpoint, "repository", imageName, scopes)
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
	auth.Authorize(req)
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
	auth.Authorize(req)
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
	auth.Authorize(req)
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
	auth.Authorize(req)
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

	auth.Authorize(req)
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
	queryParams := url.Values{}
	queryParams.Add("digest", sumType+":"+sumStr)
	req.URL.RawQuery = queryParams.Encode()
	auth.Authorize(req)
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
	auth.Authorize(req)
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
	auth.Authorize(req)
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
