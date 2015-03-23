package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/registry/v2"
	"github.com/docker/docker/utils"
)

const DockerDigestHeader = "Docker-Content-Digest"

func getV2Builder(e *Endpoint) *v2.URLBuilder {
	if e.URLBuilder == nil {
		e.URLBuilder = v2.NewURLBuilder(e.URL)
	}
	return e.URLBuilder
}

func (r *Session) V2RegistryEndpoint(index *IndexInfo) (ep *Endpoint, err error) {
	// TODO check if should use Mirror
	if index.Official {
		ep, err = newEndpoint(REGISTRYSERVER, true)
		if err != nil {
			return
		}
		err = validateEndpoint(ep)
		if err != nil {
			return
		}
	} else if r.indexEndpoint.String() == index.GetAuthConfigKey() {
		ep = r.indexEndpoint
	} else {
		ep, err = NewEndpoint(index)
		if err != nil {
			return
		}
	}

	ep.URLBuilder = v2.NewURLBuilder(ep.URL)
	return
}

// GetV2Authorization gets the authorization needed to the given image
// If readonly access is requested, then only the authorization may
// only be used for Get operations.
func (r *Session) GetV2Authorization(ep *Endpoint, imageName string, readOnly bool) (auth *RequestAuthorization, err error) {
	scopes := []string{"pull"}
	if !readOnly {
		scopes = append(scopes, "push")
	}

	log.Debugf("Getting authorization for %s %s", imageName, scopes)
	return NewRequestAuthorization(r.GetAuthConfig(true), ep, "repository", imageName, scopes), nil
}

//
// 1) Check if TarSum of each layer exists /v2/
//  1.a) if 200, continue
//  1.b) if 300, then push the
//  1.c) if anything else, err
// 2) PUT the created/signed manifest
//
func (r *Session) GetV2ImageManifest(ep *Endpoint, imageName, tagName string, auth *RequestAuthorization) ([]byte, string, error) {
	routeURL, err := getV2Builder(ep).BuildManifestURL(imageName, tagName)
	if err != nil {
		return nil, "", err
	}

	method := "GET"
	log.Debugf("[registry] Calling %q %s", method, routeURL)

	req, err := r.reqFactory.NewRequest(method, routeURL, nil)
	if err != nil {
		return nil, "", err
	}
	if err := auth.Authorize(req); err != nil {
		return nil, "", err
	}
	res, _, err := r.doRequest(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == 401 {
			return nil, "", errLoginRequired
		} else if res.StatusCode == 404 {
			return nil, "", ErrDoesNotExist
		}
		return nil, "", utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to fetch for %s:%s", res.StatusCode, imageName, tagName), res)
	}

	manifestBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", fmt.Errorf("Error while reading the http response: %s", err)
	}

	return manifestBytes, res.Header.Get(DockerDigestHeader), nil
}

// - Succeeded to head image blob (already exists)
// - Failed with no error (continue to Push the Blob)
// - Failed with error
func (r *Session) HeadV2ImageBlob(ep *Endpoint, imageName, sumType, sum string, auth *RequestAuthorization) (bool, error) {
	routeURL, err := getV2Builder(ep).BuildBlobURL(imageName, sumType+":"+sum)
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
	switch {
	case res.StatusCode >= 200 && res.StatusCode < 400:
		// return something indicating no push needed
		return true, nil
	case res.StatusCode == 401:
		return false, errLoginRequired
	case res.StatusCode == 404:
		// return something indicating blob push needed
		return false, nil
	}

	return false, utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying head request for %s - %s:%s", res.StatusCode, imageName, sumType, sum), res)
}

func (r *Session) GetV2ImageBlob(ep *Endpoint, imageName, sumType, sum string, blobWrtr io.Writer, auth *RequestAuthorization) error {
	routeURL, err := getV2Builder(ep).BuildBlobURL(imageName, sumType+":"+sum)
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

func (r *Session) GetV2ImageBlobReader(ep *Endpoint, imageName, sumType, sum string, auth *RequestAuthorization) (io.ReadCloser, int64, error) {
	routeURL, err := getV2Builder(ep).BuildBlobURL(imageName, sumType+":"+sum)
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
		return nil, 0, utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to pull %s blob - %s:%s", res.StatusCode, imageName, sumType, sum), res)
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
func (r *Session) PutV2ImageBlob(ep *Endpoint, imageName, sumType, sumStr string, blobRdr io.Reader, auth *RequestAuthorization) error {
	location, err := r.initiateBlobUpload(ep, imageName, auth)
	if err != nil {
		return err
	}

	method := "PUT"
	log.Debugf("[registry] Calling %q %s", method, location)
	req, err := r.reqFactory.NewRequest(method, location, ioutil.NopCloser(blobRdr))
	if err != nil {
		return err
	}
	queryParams := req.URL.Query()
	queryParams.Add("digest", sumType+":"+sumStr)
	req.URL.RawQuery = queryParams.Encode()
	if err := auth.Authorize(req); err != nil {
		return err
	}
	res, _, err := r.doRequest(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 201 {
		if res.StatusCode == 401 {
			return errLoginRequired
		}
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		log.Debugf("Unexpected response from server: %q %#v", errBody, res.Header)
		return utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to push %s blob - %s:%s", res.StatusCode, imageName, sumType, sumStr), res)
	}

	return nil
}

// initiateBlobUpload gets the blob upload location for the given image name.
func (r *Session) initiateBlobUpload(ep *Endpoint, imageName string, auth *RequestAuthorization) (location string, err error) {
	routeURL, err := getV2Builder(ep).BuildBlobUploadURL(imageName)
	if err != nil {
		return "", err
	}

	log.Debugf("[registry] Calling %q %s", "POST", routeURL)
	req, err := r.reqFactory.NewRequest("POST", routeURL, nil)
	if err != nil {
		return "", err
	}

	if err := auth.Authorize(req); err != nil {
		return "", err
	}
	res, _, err := r.doRequest(req)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusAccepted {
		if res.StatusCode == http.StatusUnauthorized {
			return "", errLoginRequired
		}
		if res.StatusCode == http.StatusNotFound {
			return "", ErrDoesNotExist
		}

		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return "", err
		}

		log.Debugf("Unexpected response from server: %q %#v", errBody, res.Header)
		return "", utils.NewHTTPRequestError(fmt.Sprintf("Server error: unexpected %d response status trying to initiate upload of %s", res.StatusCode, imageName), res)
	}

	if location = res.Header.Get("Location"); location == "" {
		return "", fmt.Errorf("registry did not return a Location header for resumable blob upload for image %s", imageName)
	}

	return
}

// Finally Push the (signed) manifest of the blobs we've just pushed
func (r *Session) PutV2ImageManifest(ep *Endpoint, imageName, tagName string, signedManifest, rawManifest []byte, auth *RequestAuthorization) (digest.Digest, error) {
	routeURL, err := getV2Builder(ep).BuildManifestURL(imageName, tagName)
	if err != nil {
		return "", err
	}

	method := "PUT"
	log.Debugf("[registry] Calling %q %s", method, routeURL)
	req, err := r.reqFactory.NewRequest(method, routeURL, bytes.NewReader(signedManifest))
	if err != nil {
		return "", err
	}
	if err := auth.Authorize(req); err != nil {
		return "", err
	}
	res, _, err := r.doRequest(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// All 2xx and 3xx responses can be accepted for a put.
	if res.StatusCode >= 400 {
		if res.StatusCode == 401 {
			return "", errLoginRequired
		}
		errBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return "", err
		}
		log.Debugf("Unexpected response from server: %q %#v", errBody, res.Header)
		return "", utils.NewHTTPRequestError(fmt.Sprintf("Server error: %d trying to push %s:%s manifest", res.StatusCode, imageName, tagName), res)
	}

	hdrDigest, err := digest.ParseDigest(res.Header.Get(DockerDigestHeader))
	if err != nil {
		return "", fmt.Errorf("invalid manifest digest from registry: %s", err)
	}

	dgstVerifier, err := digest.NewDigestVerifier(hdrDigest)
	if err != nil {
		return "", fmt.Errorf("invalid manifest digest from registry: %s", err)
	}

	dgstVerifier.Write(rawManifest)

	if !dgstVerifier.Verified() {
		computedDigest, _ := digest.FromBytes(rawManifest)
		return "", fmt.Errorf("unable to verify manifest digest: registry has %q, computed %q", hdrDigest, computedDigest)
	}

	return hdrDigest, nil
}

type remoteTags struct {
	name string
	tags []string
}

// Given a repository name, returns a json array of string tags
func (r *Session) GetV2RemoteTags(ep *Endpoint, imageName string, auth *RequestAuthorization) ([]string, error) {
	routeURL, err := getV2Builder(ep).BuildTagsURL(imageName)
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
