package discovery

import (
	"errors"
	"fmt"
	"net"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// Entry is exported
type Entry struct {
	Host string
	Port string
}

// NewEntry is exported
func NewEntry(url string) (*Entry, error) {
	host, port, err := net.SplitHostPort(url)
	if err != nil {
		return nil, err
	}
	return &Entry{host, port}, nil
}

func (m Entry) String() string {
	return fmt.Sprintf("%s:%s", m.Host, m.Port)
}

// WatchCallback is exported
type WatchCallback func(entries []*Entry)

// Discovery is exported
type Discovery interface {
	Initialize(string, uint64) error
	Fetch() ([]*Entry, error)
	Watch(WatchCallback)
	Register(string) error
}

var (
	discoveries map[string]Discovery
	// ErrNotSupported is exported
	ErrNotSupported = errors.New("discovery service not supported")
	// ErrNotImplemented is exported
	ErrNotImplemented = errors.New("not implemented in this discovery service")
)

func init() {
	discoveries = make(map[string]Discovery)
}

// Register is exported
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

// New is exported
func New(rawurl string, heartbeat uint64) (Discovery, error) {
	scheme, uri := parse(rawurl)

	if discovery, exists := discoveries[scheme]; exists {
		log.WithFields(log.Fields{"name": scheme, "uri": uri}).Debug("Initializing discovery service")
		err := discovery.Initialize(uri, heartbeat)
		return discovery, err
	}

	return nil, ErrNotSupported
}

// CreateEntries is exported
func CreateEntries(addrs []string) ([]*Entry, error) {
	entries := []*Entry{}
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
