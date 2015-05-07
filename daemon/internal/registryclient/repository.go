package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/docker/distribution/manifest"

	"github.com/docker/distribution/digest"

	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/api/v2"
	"golang.org/x/net/context"
)

// NewRepository creates a new Repository for the given repository name and endpoint
func NewRepository(ctx context.Context, name string, endpoint *RepositoryEndpoint) (distribution.Repository, error) {
	if err := v2.ValidateRespositoryName(name); err != nil {
		return nil, err
	}

	ub, err := endpoint.URLBuilder()
	if err != nil {
		return nil, err
	}

	client, err := endpoint.HTTPClient(name)
	if err != nil {
		return nil, err
	}

	return &repository{
		client:  client,
		ub:      ub,
		name:    name,
		context: ctx,
		mirror:  endpoint.Mirror,
	}, nil
}

type repository struct {
	client  *http.Client
	ub      *v2.URLBuilder
	context context.Context
	name    string
	mirror  bool
}

func (r *repository) Name() string {
	return r.name
}

func (r *repository) Layers() distribution.LayerService {
	return &layers{
		repository: r,
	}
}

func (r *repository) Manifests() distribution.ManifestService {
	return &manifests{
		repository: r,
	}
}

func (r *repository) Signatures() distribution.SignatureService {
	return &signatures{
		repository: r,
	}
}

type signatures struct {
	*repository
}

func (s *signatures) Get(dgst digest.Digest) ([][]byte, error) {
	panic("not implemented")
}

func (s *signatures) Put(dgst digest.Digest, signatures ...[]byte) error {
	panic("not implemented")
}

type manifests struct {
	*repository
}

func (ms *manifests) Tags() ([]string, error) {
	panic("not implemented")
}

func (ms *manifests) Exists(dgst digest.Digest) (bool, error) {
	return ms.ExistsByTag(dgst.String())
}

func (ms *manifests) ExistsByTag(tag string) (bool, error) {
	u, err := ms.ub.BuildManifestURL(ms.name, tag)
	if err != nil {
		return false, err
	}

	resp, err := ms.client.Head(u)
	if err != nil {
		return false, err
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return false, parseHTTPErrorResponse(resp)
	default:
		return false, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ms *manifests) Get(dgst digest.Digest) (*manifest.SignedManifest, error) {
	return ms.GetByTag(dgst.String())
}

func (ms *manifests) GetByTag(tag string) (*manifest.SignedManifest, error) {
	u, err := ms.ub.BuildManifestURL(ms.name, tag)
	if err != nil {
		return nil, err
	}

	resp, err := ms.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		var sm manifest.SignedManifest
		decoder := json.NewDecoder(resp.Body)

		if err := decoder.Decode(&sm); err != nil {
			return nil, err
		}

		return &sm, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, parseHTTPErrorResponse(resp)
	default:
		return nil, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ms *manifests) Put(m *manifest.SignedManifest) error {
	manifestURL, err := ms.ub.BuildManifestURL(ms.name, m.Tag)
	if err != nil {
		return err
	}

	putRequest, err := http.NewRequest("PUT", manifestURL, bytes.NewReader(m.Raw))
	if err != nil {
		return err
	}

	resp, err := ms.client.Do(putRequest)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusAccepted:
		// TODO(dmcgowan): Use or check digest header
		return nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return parseHTTPErrorResponse(resp)
	default:
		return &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ms *manifests) Delete(dgst digest.Digest) error {
	u, err := ms.ub.BuildManifestURL(ms.name, dgst.String())
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return err
	}

	resp, err := ms.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return parseHTTPErrorResponse(resp)
	default:
		return &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

type layers struct {
	*repository
}

func sanitizeLocation(location, source string) (string, error) {
	locationURL, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	if locationURL.Scheme == "" {
		sourceURL, err := url.Parse(source)
		if err != nil {
			return "", err
		}
		locationURL = &url.URL{
			Scheme: sourceURL.Scheme,
			Host:   sourceURL.Host,
			Path:   location,
		}
		location = locationURL.String()
	}
	return location, nil
}

func (ls *layers) Exists(dgst digest.Digest) (bool, error) {
	_, err := ls.fetchLayer(dgst)
	if err != nil {
		switch err := err.(type) {
		case distribution.ErrUnknownLayer:
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func (ls *layers) Fetch(dgst digest.Digest) (distribution.Layer, error) {
	return ls.fetchLayer(dgst)
}

func (ls *layers) Upload() (distribution.LayerUpload, error) {
	u, err := ls.ub.BuildBlobUploadURL(ls.name)

	resp, err := ls.client.Post(u, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusAccepted:
		// TODO(dmcgowan): Check for invalid UUID
		uuid := resp.Header.Get("Docker-Upload-UUID")
		location, err := sanitizeLocation(resp.Header.Get("Location"), u)
		if err != nil {
			return nil, err
		}

		return &httpLayerUpload{
			repo:      ls.repository,
			client:    ls.client,
			uuid:      uuid,
			startedAt: time.Now(),
			location:  location,
		}, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, parseHTTPErrorResponse(resp)
	default:
		return nil, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ls *layers) Resume(uuid string) (distribution.LayerUpload, error) {
	panic("not implemented")
}

func (ls *layers) fetchLayer(dgst digest.Digest) (distribution.Layer, error) {
	u, err := ls.ub.BuildBlobURL(ls.name, dgst)
	if err != nil {
		return nil, err
	}

	resp, err := ls.client.Head(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		lengthHeader := resp.Header.Get("Content-Length")
		length, err := strconv.ParseInt(lengthHeader, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing content-length: %v", err)
		}

		var t time.Time
		lastModified := resp.Header.Get("Last-Modified")
		if lastModified != "" {
			t, err = http.ParseTime(lastModified)
			if err != nil {
				return nil, fmt.Errorf("error parsing last-modified: %v", err)
			}
		}

		return &httpLayer{
			layers:    ls,
			size:      length,
			digest:    dgst,
			createdAt: t,
		}, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, distribution.ErrUnknownLayer{
			FSLayer: manifest.FSLayer{
				BlobSum: dgst,
			},
		}
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, parseHTTPErrorResponse(resp)
	default:
		return nil, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}
