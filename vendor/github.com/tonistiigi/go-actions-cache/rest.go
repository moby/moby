package actionscache

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

const (
	apiURL  = "https://api.github.com"
	perPage = 100
)

type RestAPI struct {
	repo  string
	token string
	opt   Opt
}

type CacheKey struct {
	ID           int    `json:"id"`
	Ref          string `json:"ref"`
	Key          string `json:"key"`
	Version      string `json:"version"`
	LastAccessed string `json:"last_accessed_at"`
	CreatedAt    string `json:"created_at"`
	SizeInBytes  int    `json:"size_in_bytes"`
}

func NewRestAPI(repo, token string, opt Opt) (*RestAPI, error) {
	opt = optsWithDefaults(opt)
	return &RestAPI{
		repo:  repo,
		token: token,
		opt:   opt,
	}, nil
}

func (r *RestAPI) httpReq(ctx context.Context, method string, url *url.URL) (*http.Request, error) {
	req, err := http.NewRequest(method, url.String(), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return req, nil
}

func (r *RestAPI) ListKeys(ctx context.Context, prefix, ref string) ([]CacheKey, error) {
	var out []CacheKey
	page := 1
	for {
		keys, total, err := r.listKeysPage(ctx, prefix, ref, page)
		if err != nil {
			return nil, err
		}
		out = append(out, keys...)
		if total > page*perPage {
			page++
		} else {
			break
		}
	}
	return out, nil
}

func (r *RestAPI) listKeysPage(ctx context.Context, prefix, ref string, page int) ([]CacheKey, int, error) {
	u, err := url.Parse(apiURL + "/repos/" + r.repo + "/actions/caches")
	if err != nil {
		return nil, 0, err
	}
	q := u.Query()
	q.Set("per_page", strconv.Itoa(perPage))
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if prefix != "" {
		q.Set("key", prefix)
	}
	if ref != "" {
		q.Set("ref", ref)
	}
	u.RawQuery = q.Encode()

	req, err := r.httpReq(ctx, "GET", u)
	if err != nil {
		return nil, 0, err
	}

	resp, err := r.opt.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	dec := json.NewDecoder(resp.Body)
	var keys struct {
		Total  int        `json:"total_count"`
		Caches []CacheKey `json:"actions_caches"`
	}

	if err := dec.Decode(&keys); err != nil {
		return nil, 0, err
	}

	resp.Body.Close()
	return keys.Caches, keys.Total, nil
}
