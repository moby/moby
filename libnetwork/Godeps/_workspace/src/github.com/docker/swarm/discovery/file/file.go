package file

import (
	"io/ioutil"
	"strings"
	"time"

	"github.com/docker/swarm/discovery"
)

// Discovery is exported
type Discovery struct {
	heartbeat uint64
	path      string
}

func init() {
	discovery.Register("file", &Discovery{})
}

// Initialize is exported
func (s *Discovery) Initialize(path string, heartbeat uint64) error {
	s.path = path
	s.heartbeat = heartbeat
	return nil
}

func parseFileContent(content []byte) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		line = strings.TrimSpace(line)
		// Ignoring line starts with #
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Inlined # comment also ignored.
		if strings.Contains(line, "#") {
			line = line[0:strings.Index(line, "#")]
			// Trim additional spaces caused by above stripping.
			line = strings.TrimSpace(line)
		}
		for _, ip := range discovery.Generate(line) {
			result = append(result, ip)
		}
	}
	return result
}

// Fetch is exported
func (s *Discovery) Fetch() ([]*discovery.Entry, error) {
	fileContent, err := ioutil.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	return discovery.CreateEntries(parseFileContent(fileContent))
}

// Watch is exported
func (s *Discovery) Watch(callback discovery.WatchCallback) {
	for _ = range time.Tick(time.Duration(s.heartbeat) * time.Second) {
		entries, err := s.Fetch()
		if err == nil {
			callback(entries)
		}
	}
}

// Register is exported
func (s *Discovery) Register(addr string) error {
	return discovery.ErrNotImplemented
}
