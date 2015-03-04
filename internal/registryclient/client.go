package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/registry/api/v2"
)

// Client implements the client interface to the registry http api
type Client interface {
	// GetImageManifest returns an image manifest for the image at the given
	// name, tag pair.
	GetImageManifest(name, tag string) (*manifest.SignedManifest, error)

	// PutImageManifest uploads an image manifest for the image at the given
	// name, tag pair.
	PutImageManifest(name, tag string, imageManifest *manifest.SignedManifest) error

	// DeleteImage removes the image at the given name, tag pair.
	DeleteImage(name, tag string) error

	// ListImageTags returns a list of all image tags with the given repository
	// name.
	ListImageTags(name string) ([]string, error)

	// BlobLength returns the length of the blob stored at the given name,
	// digest pair.
	// Returns a length value of -1 on error or if the blob does not exist.
	BlobLength(name string, dgst digest.Digest) (int, error)

	// GetBlob returns the blob stored at the given name, digest pair in the
	// form of an io.ReadCloser with the length of this blob.
	// A nonzero byteOffset can be provided to receive a partial blob beginning
	// at the given offset.
	GetBlob(name string, dgst digest.Digest, byteOffset int) (io.ReadCloser, int, error)

	// InitiateBlobUpload starts a blob upload in the given repository namespace
	// and returns a unique location url to use for other blob upload methods.
	InitiateBlobUpload(name string) (string, error)

	// GetBlobUploadStatus returns the byte offset and length of the blob at the
	// given upload location.
	GetBlobUploadStatus(location string) (int, int, error)

	// UploadBlob uploads a full blob to the registry.
	UploadBlob(location string, blob io.ReadCloser, length int, dgst digest.Digest) error

	// UploadBlobChunk uploads a blob chunk with a given length and startByte to
	// the registry.
	// FinishChunkedBlobUpload must be called to finalize this upload.
	UploadBlobChunk(location string, blobChunk io.ReadCloser, length, startByte int) error

	// FinishChunkedBlobUpload completes a chunked blob upload at a given
	// location.
	FinishChunkedBlobUpload(location string, length int, dgst digest.Digest) error

	// CancelBlobUpload deletes all content at the unfinished blob upload
	// location and invalidates any future calls to this blob upload.
	CancelBlobUpload(location string) error
}

var (
	patternRangeHeader = regexp.MustCompile("bytes=0-(\\d+)/(\\d+)")
)

// New returns a new Client which operates against a registry with the
// given base endpoint
// This endpoint should not include /v2/ or any part of the url after this.
func New(endpoint string) (Client, error) {
	ub, err := v2.NewURLBuilderFromString(endpoint)
	if err != nil {
		return nil, err
	}

	return &clientImpl{
		endpoint: endpoint,
		ub:       ub,
	}, nil
}

// clientImpl is the default implementation of the Client interface
type clientImpl struct {
	endpoint string
	ub       *v2.URLBuilder
}

// TODO(bbland): use consistent route generation between server and client

func (r *clientImpl) GetImageManifest(name, tag string) (*manifest.SignedManifest, error) {
	manifestURL, err := r.ub.BuildManifestURL(name, tag)
	if err != nil {
		return nil, err
	}

	response, err := http.Get(manifestURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusOK:
		break
	case response.StatusCode == http.StatusNotFound:
		return nil, &ImageManifestNotFoundError{Name: name, Tag: tag}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors

		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return nil, err
		}
		return nil, &errs
	default:
		return nil, &UnexpectedHTTPStatusError{Status: response.Status}
	}

	decoder := json.NewDecoder(response.Body)

	manifest := new(manifest.SignedManifest)
	err = decoder.Decode(manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func (r *clientImpl) PutImageManifest(name, tag string, manifest *manifest.SignedManifest) error {
	manifestURL, err := r.ub.BuildManifestURL(name, tag)
	if err != nil {
		return err
	}

	putRequest, err := http.NewRequest("PUT", manifestURL, bytes.NewReader(manifest.Raw))
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(putRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusOK || response.StatusCode == http.StatusAccepted:
		return nil
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errors v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errors)
		if err != nil {
			return err
		}

		return &errors
	default:
		return &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) DeleteImage(name, tag string) error {
	manifestURL, err := r.ub.BuildManifestURL(name, tag)
	if err != nil {
		return err
	}

	deleteRequest, err := http.NewRequest("DELETE", manifestURL, nil)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(deleteRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusNoContent:
		break
	case response.StatusCode == http.StatusNotFound:
		return &ImageManifestNotFoundError{Name: name, Tag: tag}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return err
		}
		return &errs
	default:
		return &UnexpectedHTTPStatusError{Status: response.Status}
	}

	return nil
}

func (r *clientImpl) ListImageTags(name string) ([]string, error) {
	tagsURL, err := r.ub.BuildTagsURL(name)
	if err != nil {
		return nil, err
	}

	response, err := http.Get(tagsURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusOK:
		break
	case response.StatusCode == http.StatusNotFound:
		return nil, &RepositoryNotFoundError{Name: name}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return nil, err
		}
		return nil, &errs
	default:
		return nil, &UnexpectedHTTPStatusError{Status: response.Status}
	}

	tags := struct {
		Tags []string `json:"tags"`
	}{}

	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&tags)
	if err != nil {
		return nil, err
	}

	return tags.Tags, nil
}

func (r *clientImpl) BlobLength(name string, dgst digest.Digest) (int, error) {
	blobURL, err := r.ub.BuildBlobURL(name, dgst)
	if err != nil {
		return -1, err
	}

	response, err := http.Head(blobURL)
	if err != nil {
		return -1, err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusOK:
		lengthHeader := response.Header.Get("Content-Length")
		length, err := strconv.ParseInt(lengthHeader, 10, 64)
		if err != nil {
			return -1, err
		}
		return int(length), nil
	case response.StatusCode == http.StatusNotFound:
		return -1, nil
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return -1, err
		}
		return -1, &errs
	default:
		return -1, &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) GetBlob(name string, dgst digest.Digest, byteOffset int) (io.ReadCloser, int, error) {
	blobURL, err := r.ub.BuildBlobURL(name, dgst)
	if err != nil {
		return nil, 0, err
	}

	getRequest, err := http.NewRequest("GET", blobURL, nil)
	if err != nil {
		return nil, 0, err
	}

	getRequest.Header.Add("Range", fmt.Sprintf("%d-", byteOffset))
	response, err := http.DefaultClient.Do(getRequest)
	if err != nil {
		return nil, 0, err
	}

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusOK:
		lengthHeader := response.Header.Get("Content-Length")
		length, err := strconv.ParseInt(lengthHeader, 10, 0)
		if err != nil {
			return nil, 0, err
		}
		return response.Body, int(length), nil
	case response.StatusCode == http.StatusNotFound:
		response.Body.Close()
		return nil, 0, &BlobNotFoundError{Name: name, Digest: dgst}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return nil, 0, err
		}
		return nil, 0, &errs
	default:
		response.Body.Close()
		return nil, 0, &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) InitiateBlobUpload(name string) (string, error) {
	uploadURL, err := r.ub.BuildBlobUploadURL(name)
	if err != nil {
		return "", err
	}

	postRequest, err := http.NewRequest("POST", uploadURL, nil)
	if err != nil {
		return "", err
	}

	response, err := http.DefaultClient.Do(postRequest)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusAccepted:
		return response.Header.Get("Location"), nil
	// case response.StatusCode == http.StatusNotFound:
	// return
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return "", err
		}
		return "", &errs
	default:
		return "", &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) GetBlobUploadStatus(location string) (int, int, error) {
	response, err := http.Get(location)
	if err != nil {
		return 0, 0, err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusNoContent:
		return parseRangeHeader(response.Header.Get("Range"))
	case response.StatusCode == http.StatusNotFound:
		return 0, 0, &BlobUploadNotFoundError{Location: location}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return 0, 0, err
		}
		return 0, 0, &errs
	default:
		return 0, 0, &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) UploadBlob(location string, blob io.ReadCloser, length int, dgst digest.Digest) error {
	defer blob.Close()

	putRequest, err := http.NewRequest("PUT", location, blob)
	if err != nil {
		return err
	}

	values := putRequest.URL.Query()
	values.Set("digest", dgst.String())
	putRequest.URL.RawQuery = values.Encode()

	putRequest.Header.Set("Content-Type", "application/octet-stream")
	putRequest.Header.Set("Content-Length", fmt.Sprint(length))

	response, err := http.DefaultClient.Do(putRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusCreated:
		return nil
	case response.StatusCode == http.StatusNotFound:
		return &BlobUploadNotFoundError{Location: location}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return err
		}
		return &errs
	default:
		return &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) UploadBlobChunk(location string, blobChunk io.ReadCloser, length, startByte int) error {
	defer blobChunk.Close()

	putRequest, err := http.NewRequest("PUT", location, blobChunk)
	if err != nil {
		return err
	}

	endByte := startByte + length

	putRequest.Header.Set("Content-Type", "application/octet-stream")
	putRequest.Header.Set("Content-Length", fmt.Sprint(length))
	putRequest.Header.Set("Content-Range",
		fmt.Sprintf("%d-%d/%d", startByte, endByte, endByte))

	response, err := http.DefaultClient.Do(putRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusAccepted:
		return nil
	case response.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		lastValidRange, blobSize, err := parseRangeHeader(response.Header.Get("Range"))
		if err != nil {
			return err
		}
		return &BlobUploadInvalidRangeError{
			Location:       location,
			LastValidRange: lastValidRange,
			BlobSize:       blobSize,
		}
	case response.StatusCode == http.StatusNotFound:
		return &BlobUploadNotFoundError{Location: location}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return err
		}
		return &errs
	default:
		return &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) FinishChunkedBlobUpload(location string, length int, dgst digest.Digest) error {
	putRequest, err := http.NewRequest("PUT", location, nil)
	if err != nil {
		return err
	}

	values := putRequest.URL.Query()
	values.Set("digest", dgst.String())
	putRequest.URL.RawQuery = values.Encode()

	putRequest.Header.Set("Content-Type", "application/octet-stream")
	putRequest.Header.Set("Content-Length", "0")
	putRequest.Header.Set("Content-Range",
		fmt.Sprintf("%d-%d/%d", length, length, length))

	response, err := http.DefaultClient.Do(putRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusCreated:
		return nil
	case response.StatusCode == http.StatusNotFound:
		return &BlobUploadNotFoundError{Location: location}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return err
		}
		return &errs
	default:
		return &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

func (r *clientImpl) CancelBlobUpload(location string) error {
	deleteRequest, err := http.NewRequest("DELETE", location, nil)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(deleteRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// TODO(bbland): handle other status codes, like 5xx errors
	switch {
	case response.StatusCode == http.StatusNoContent:
		return nil
	case response.StatusCode == http.StatusNotFound:
		return &BlobUploadNotFoundError{Location: location}
	case response.StatusCode >= 400 && response.StatusCode < 500:
		var errs v2.Errors
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&errs)
		if err != nil {
			return err
		}
		return &errs
	default:
		return &UnexpectedHTTPStatusError{Status: response.Status}
	}
}

// parseRangeHeader parses out the offset and length from a returned Range
// header
func parseRangeHeader(byteRangeHeader string) (int, int, error) {
	submatches := patternRangeHeader.FindStringSubmatch(byteRangeHeader)
	if submatches == nil || len(submatches) < 3 {
		return 0, 0, fmt.Errorf("Malformed Range header")
	}

	offset, err := strconv.Atoi(submatches[1])
	if err != nil {
		return 0, 0, err
	}
	length, err := strconv.Atoi(submatches[2])
	if err != nil {
		return 0, 0, err
	}
	return offset, length, nil
}
