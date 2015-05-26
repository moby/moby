package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/docker/swarm/discovery"
)

// DiscoveryUrl is exported
const DiscoveryURL = "https://discovery-stage.hub.docker.com/v1"

// Discovery is exported
type Discovery struct {
	heartbeat time.Duration
	ttl       time.Duration
	url       string
	token     string
}

func init() {
	Init()
}

// Init is exported
func Init() {
	discovery.Register("token", &Discovery{})
}

// Initialize is exported
func (s *Discovery) Initialize(urltoken string, heartbeat time.Duration, ttl time.Duration) error {
	if i := strings.LastIndex(urltoken, "/"); i != -1 {
		s.url = "https://" + urltoken[:i]
		s.token = urltoken[i+1:]
	} else {
		s.url = DiscoveryURL
		s.token = urltoken
	}

	if s.token == "" {
		return errors.New("token is empty")
	}
	s.heartbeat = heartbeat
	s.ttl = ttl

	return nil
}

// Fetch returns the list of entries for the discovery service at the specified endpoint
func (s *Discovery) fetch() (discovery.Entries, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s/%s", s.url, "clusters", s.token))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var addrs []string
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&addrs); err != nil {
			return nil, fmt.Errorf("Failed to decode response: %v", err)
		}
	} else {
		return nil, fmt.Errorf("Failed to fetch entries, Discovery service returned %d HTTP status code", resp.StatusCode)
	}

	return discovery.CreateEntries(addrs)
}

// Watch is exported
func (s *Discovery) Watch(stopCh <-chan struct{}) (<-chan discovery.Entries, <-chan error) {
	ch := make(chan discovery.Entries)
	ticker := time.NewTicker(s.heartbeat)
	errCh := make(chan error)

	go func() {
		defer close(ch)
		defer close(errCh)

		// Send the initial entries if available.
		currentEntries, err := s.fetch()
		if err != nil {
			errCh <- err
		} else {
			ch <- currentEntries
		}

		// Periodically send updates.
		for {
			select {
			case <-ticker.C:
				newEntries, err := s.fetch()
				if err != nil {
					errCh <- err
					continue
				}

				// Check if the file has really changed.
				if !newEntries.Equals(currentEntries) {
					ch <- newEntries
				}
				currentEntries = newEntries
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()

	return ch, nil
}

// Register adds a new entry identified by the into the discovery service
func (s *Discovery) Register(addr string) error {
	buf := strings.NewReader(addr)

	resp, err := http.Post(fmt.Sprintf("%s/%s/%s", s.url,
		"clusters", s.token), "application/json", buf)

	if err != nil {
		return err
	}

	resp.Body.Close()
	return nil
}

// CreateCluster returns a unique cluster token
func (s *Discovery) CreateCluster() (string, error) {
	resp, err := http.Post(fmt.Sprintf("%s/%s", s.url, "clusters"), "", nil)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	token, err := ioutil.ReadAll(resp.Body)
	return string(token), err
}
