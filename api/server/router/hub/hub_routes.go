package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/hub"
	"github.com/docker/docker/dockerversion"
)

// getHubImageTags requires a valid image name
func (hr *hubRouter) getHubImageTags(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	c := &http.Client{
		Timeout: 5 * time.Second,
	}

	imageName, ok := vars["name"]
	if !ok || imageName == "" {
		return errors.New("missing image name on the `name` path parameter")
	}

	u := &url.URL{
		Scheme:   "https",
		Host:     "hub.docker.com",
		Path:     "/v2/repositories/library/" + imageName + "/tags/",
		RawQuery: r.URL.RawQuery,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating hub image tags request: %w", err)
	}
	req.Header.Set("User-Agent", dockerversion.DockerUserAgent(ctx))
	if r.Header.Get("Authorization") != "" {
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("error sending hub image tags request: %w", err)
	}
	defer resp.Body.Close()

	var tags *hub.ImageTags
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("error decoding hub image tags response: %w", err)
	}

	return httputils.WriteJSON(w, http.StatusOK, tags)
}

func (hr *hubRouter) getHubImageSearch(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	c := &http.Client{
		Timeout: 5 * time.Second,
	}

	u := &url.URL{
		Scheme:   "https",
		Host:     "hub.docker.com",
		Path:     "/api/search/v3/catalog/search",
		RawQuery: r.URL.RawQuery,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating hub image search request: %w", err)
	}
	req.Header.Set("User-Agent", dockerversion.DockerUserAgent(ctx))
	if r.Header.Get("Authorization") != "" {
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("error sending hub image search request: %w", err)
	}
	defer resp.Body.Close()

	var images *hub.SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return fmt.Errorf("error decoding hub image search response: %w", err)
	}
	return httputils.WriteJSON(w, http.StatusOK, images)
}
