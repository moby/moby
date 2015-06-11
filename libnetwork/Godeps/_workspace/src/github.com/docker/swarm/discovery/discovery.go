package discovery

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

// An Entry represents a swarm host.
type Entry struct {
	Host string
	Port string
}

// NewEntry creates a new entry.
func NewEntry(url string) (*Entry, error) {
	host, port, err := net.SplitHostPort(url)
	if err != nil {
		return nil, err
	}
	return &Entry{host, port}, nil
}

// String returns the string form of an entry.
func (e *Entry) String() string {
	return fmt.Sprintf("%s:%s", e.Host, e.Port)
}

// Equals returns true if cmp contains the same data.
func (e *Entry) Equals(cmp *Entry) bool {
	return e.Host == cmp.Host && e.Port == cmp.Port
}

// Entries is a list of *Entry with some helpers.
type Entries []*Entry

// Equals returns true if cmp contains the same data.
func (e Entries) Equals(cmp Entries) bool {
	// Check if the file has really changed.
	if len(e) != len(cmp) {
		return false
	}
	for i := range e {
		if !e[i].Equals(cmp[i]) {
			return false
		}
	}
	return true
}

// Contains returns true if the Entries contain a given Entry.
func (e Entries) Contains(entry *Entry) bool {
	for _, curr := range e {
		if curr.Equals(entry) {
			return true
		}
	}
	return false
}

// Diff compares two entries and returns the added and removed entries.
func (e Entries) Diff(cmp Entries) (Entries, Entries) {
	added := Entries{}
	for _, entry := range cmp {
		if !e.Contains(entry) {
			added = append(added, entry)
		}
	}

	removed := Entries{}
	for _, entry := range e {
		if !cmp.Contains(entry) {
			removed = append(removed, entry)
		}
	}

	return added, removed
}

// The Discovery interface is implemented by Discovery backends which
// manage swarm host entries.
type Discovery interface {
	// Initialize the discovery with URIs, a heartbeat and a ttl.
	Initialize(string, time.Duration, time.Duration) error

	// Watch the discovery for entry changes.
	// Returns a channel that will receive changes or an error.
	// Providing a non-nil stopCh can be used to stop watching.
	Watch(stopCh <-chan struct{}) (<-chan Entries, <-chan error)

	// Register to the discovery
	Register(string) error
}

var (
	discoveries map[string]Discovery
	// ErrNotSupported is returned when a discovery service is not supported.
	ErrNotSupported = errors.New("discovery service not supported")
	// ErrNotImplemented is returned when discovery feature is not implemented
	// by discovery backend.
	ErrNotImplemented = errors.New("not implemented in this discovery service")
)

func init() {
	discoveries = make(map[string]Discovery)
}

// Register makes a discovery backend available by the provided scheme.
// If Register is called twice with the same scheme an error is returned.
func Register(scheme string, d Discovery) error {
	if _, exists := discoveries[scheme]; exists {
		return fmt.Errorf("scheme already registered %s", scheme)
	}
	log.WithField("name", scheme).Debug("Registering discovery service")
	discoveries[scheme] = d

	return nil
}

func parse(rawurl string) (string, string) {
	parts := strings.SplitN(rawurl, "://", 2)

	// nodes:port,node2:port => nodes://node1:port,node2:port
	if len(parts) == 1 {
		return "nodes", parts[0]
	}
	return parts[0], parts[1]
}

// New returns a new Discovery given a URL, heartbeat and ttl settings.
// Returns an error if the URL scheme is not supported.
func New(rawurl string, heartbeat time.Duration, ttl time.Duration) (Discovery, error) {
	scheme, uri := parse(rawurl)

	if discovery, exists := discoveries[scheme]; exists {
		log.WithFields(log.Fields{"name": scheme, "uri": uri}).Debug("Initializing discovery service")
		err := discovery.Initialize(uri, heartbeat, ttl)
		return discovery, err
	}

	return nil, ErrNotSupported
}

// CreateEntries returns an array of entries based on the given addresses.
func CreateEntries(addrs []string) (Entries, error) {
	entries := Entries{}
	if addrs == nil {
		return entries, nil
	}

	for _, addr := range addrs {
		if len(addr) == 0 {
			continue
		}
		entry, err := NewEntry(addr)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
