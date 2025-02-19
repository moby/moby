package actionscache

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/pkg/errors"
)

func (c *Cache) reserveV2(ctx context.Context, key string) (string, error) {
	dt, err := json.Marshal(ReserveCacheReq{Key: key, Version: version(key)})
	if err != nil {
		return "", errors.WithStack(err)
	}
	req := c.newRequestV2(c.urlV2("CreateCacheEntry"), func() io.Reader {
		return bytes.NewReader(dt)
	})
	Log("save cache req %s body=%s", req.url, dt)
	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer resp.Body.Close()

	dt, err = io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return "", errors.WithStack(err)
	}
	Log("save cache resp %s body=%s", req.url, dt)
	var cr struct {
		OK              bool   `json:"ok"`
		SignedUploadURL string `json:"signed_upload_url"`
	}
	if err := json.Unmarshal(dt, &cr); err != nil {
		return "", errors.WithStack(err)
	}

	if !cr.OK {
		return "", errors.New("failed to reserve cache")
	}
	return cr.SignedUploadURL, nil
}

func (c *Cache) uploadV2(ctx context.Context, url string, b Blob) error {
	client, err := blockblob.NewClientWithNoCredential(url, nil)
	if err != nil {
		return errors.WithStack(err)
	}

	// uploading with sized blocks requires io.File ¯\_(ツ)_/¯
	resp, err := client.UploadStream(ctx, &rc{ReaderAt: b}, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	Log("upload cache %s %s", url, *resp.RequestID)
	return nil
}

func (ce *Entry) downloadV2(ctx context.Context) ReaderAtCloser {
	return toReaderAtCloser(func(offset int64) (io.ReadCloser, error) {
		client, err := blockblob.NewClientWithNoCredential(ce.URL, nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		resp, err := client.DownloadStream(ctx, &blob.DownloadStreamOptions{
			Range: blob.HTTPRange{Offset: offset},
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return resp.Body, nil
	})
}

func (c *Cache) commitV2(ctx context.Context, key string, size int64) error {
	var payload = struct {
		Key       string `json:"key"`
		SizeBytes int64  `json:"size_bytes"`
		Version   string `json:"version"`
	}{
		Key:       key,
		SizeBytes: size,
		Version:   version(key),
	}

	dt, err := json.Marshal(payload)
	if err != nil {
		return errors.WithStack(err)
	}

	req := c.newRequestV2(c.urlV2("FinalizeCacheEntryUpload"), func() io.Reader {
		return bytes.NewReader(dt)
	})

	Log("commit cache req %s body=%s", req.url, dt)

	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	dt, err = io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return errors.WithStack(err)
	}

	var cr struct {
		OK      bool   `json:"ok"`
		EntryID string `json:"entry_id"`
	}
	if err := json.Unmarshal(dt, &cr); err != nil {
		return errors.WithStack(err)
	}
	if !cr.OK {
		return errors.New("failed to commit cache")
	}

	Log("commit cache resp %s %s", req.url, cr.EntryID)
	return nil
}

func (c *Cache) loadV2(ctx context.Context, keys ...string) (*Entry, error) {
	var payload = struct {
		Key         string   `json:"key"`
		RestoreKeys []string `json:"restore_keys"`
		Version     string   `json:"version"`
	}{
		Key:         keys[0],
		RestoreKeys: keys,
		Version:     version(keys[0]),
	}
	dt, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	req := c.newRequestV2(c.urlV2("GetCacheEntryDownloadURL"), func() io.Reader {
		return bytes.NewReader(dt)
	})

	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close()

	dt, err = io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var val struct {
		OK                bool   `json:"ok"`
		SignedDownloadURL string `json:"signed_download_url"`
		MatchedKey        string `json:"matched_key"`
	}

	if err := json.Unmarshal(dt, &val); err != nil {
		return nil, errors.WithStack(err)
	}

	if !val.OK {
		return nil, nil
	}

	var ce Entry
	ce.Key = val.MatchedKey
	ce.URL = val.SignedDownloadURL
	ce.IsAzureBlob = true
	ce.client = c.opt.Client

	return &ce, nil
}

func (c *Cache) newRequestV2(url string, body func() io.Reader) *request {
	return &request{
		method: "POST",
		url:    url,
		body:   body,
		headers: map[string]string{
			"Authorization": "Bearer " + c.Token.Raw,
			"Content-Type":  "application/json",
			"User-Agent":    c.opt.UserAgent,
		},
	}
}

func (c *Cache) urlV2(p string) string {
	return c.URL + "twirp/github.actions.results.api.v1.CacheService/" + p
}
