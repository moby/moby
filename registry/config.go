package registry

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/reference"
	registrytypes "github.com/docker/engine-api/types/registry"
)

// Options holds command line options.
type Options struct {
	Mirrors            opts.ListOpts
	InsecureRegistries opts.ListOpts
}

var (
	// DefaultNamespace is the default namespace
	DefaultNamespace = "docker.io"
	// DefaultRegistryVersionHeader is the name of the default HTTP header
	// that carries Registry version info
	DefaultRegistryVersionHeader = "Docker-Distribution-Api-Version"

	// IndexServer is the v1 registry server used for user auth + account creation
	IndexServer = DefaultV1Registry.String() + "/v1/"
	// IndexName is the name of the index
	IndexName = "docker.io"

	// NotaryServer is the endpoint serving the Notary trust server
	NotaryServer = "https://notary.docker.io"

	// IndexServer = "https://registry-stage.hub.docker.com/v1/"
)

var (
	// ErrInvalidRepositoryName is an error returned if the repository name did
	// not have the correct form
	ErrInvalidRepositoryName = errors.New("Invalid repository name (ex: \"registry.domain.tld/myrepos\")")

	emptyServiceConfig = NewServiceConfig(nil)

	// V2Only controls access to legacy registries.  If it is set to true via the
	// command line flag the daemon will not attempt to contact v1 legacy registries
	V2Only = false
)

// for mocking in unit tests
var lookupIP = net.LookupIP

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
func (options *Options) InstallFlags(cmd *flag.FlagSet, usageFn func(string) string) {
	options.Mirrors = opts.NewListOpts(ValidateMirror)
	cmd.Var(&options.Mirrors, []string{"-registry-mirror"}, usageFn("Preferred Docker registry mirror"))
	options.InsecureRegistries = opts.NewListOpts(ValidateIndexName)
	cmd.Var(&options.InsecureRegistries, []string{"-insecure-registry"}, usageFn("Enable insecure registry communication"))
	cmd.BoolVar(&V2Only, []string{"-disable-legacy-registry"}, false, usageFn("Do not contact legacy registries"))
}

// NewServiceConfig returns a new instance of ServiceConfig
func NewServiceConfig(options *Options) *registrytypes.ServiceConfig {
	if options == nil {
		options = &Options{
			Mirrors:            opts.NewListOpts(nil),
			InsecureRegistries: opts.NewListOpts(nil),
		}
	}

	// Localhost is by default considered as an insecure registry
	// This is a stop-gap for people who are running a private registry on localhost (especially on Boot2docker).
	//
	// TODO: should we deprecate this once it is easier for people to set up a TLS registry or change
	// daemon flags on boot2docker?
	options.InsecureRegistries.Set("127.0.0.0/8")

	config := &registrytypes.ServiceConfig{
		InsecureRegistryCIDRs: make([]*registrytypes.NetIPNet, 0),
		IndexConfigs:          make(map[string]*registrytypes.IndexInfo, 0),
		// Hack: Bypass setting the mirrors to IndexConfigs since they are going away
		// and Mirrors are only for the official registry anyways.
		Mirrors: options.Mirrors.GetAll(),
	}
	// Split --insecure-registry into CIDR and registry-specific settings.
	for _, r := range options.InsecureRegistries.GetAll() {
		// Check if CIDR was passed to --insecure-registry
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			// Valid CIDR.
			config.InsecureRegistryCIDRs = append(config.InsecureRegistryCIDRs, (*registrytypes.NetIPNet)(ipnet))
		} else {
			// Assume `host:port` if not CIDR.
			config.IndexConfigs[r] = &registrytypes.IndexInfo{
				Name:     r,
				Mirrors:  make([]string, 0),
				Secure:   false,
				Official: false,
			}
		}
	}

	// Configure public registry.
	config.IndexConfigs[IndexName] = &registrytypes.IndexInfo{
		Name:     IndexName,
		Mirrors:  config.Mirrors,
		Secure:   true,
		Official: true,
	}

	return config
}

// isSecureIndex returns false if the provided indexName is part of the list of insecure registries
// Insecure registries accept HTTP and/or accept HTTPS with certificates from unknown CAs.
//
// The list of insecure registries can contain an element with CIDR notation to specify a whole subnet.
// If the subnet contains one of the IPs of the registry specified by indexName, the latter is considered
// insecure.
//
// indexName should be a URL.Host (`host:port` or `host`) where the `host` part can be either a domain name
// or an IP address. If it is a domain name, then it will be resolved in order to check if the IP is contained
// in a subnet. If the resolving is not successful, isSecureIndex will only try to match hostname to any element
// of insecureRegistries.
func isSecureIndex(config *registrytypes.ServiceConfig, indexName string) bool {
	// Check for configured index, first.  This is needed in case isSecureIndex
	// is called from anything besides newIndexInfo, in order to honor per-index configurations.
	if index, ok := config.IndexConfigs[indexName]; ok {
		return index.Secure
	}

	host, _, err := net.SplitHostPort(indexName)
	if err != nil {
		// assume indexName is of the form `host` without the port and go on.
		host = indexName
	}

	addrs, err := lookupIP(host)
	if err != nil {
		ip := net.ParseIP(host)
		if ip != nil {
			addrs = []net.IP{ip}
		}

		// if ip == nil, then `host` is neither an IP nor it could be looked up,
		// either because the index is unreachable, or because the index is behind an HTTP proxy.
		// So, len(addrs) == 0 and we're not aborting.
	}

	// Try CIDR notation only if addrs has any elements, i.e. if `host`'s IP could be determined.
	for _, addr := range addrs {
		for _, ipnet := range config.InsecureRegistryCIDRs {
			// check if the addr falls in the subnet
			if (*net.IPNet)(ipnet).Contains(addr) {
				return false
			}
		}
	}

	return true
}

// ValidateMirror validates an HTTP(S) registry mirror
func ValidateMirror(val string) (string, error) {
	uri, err := url.Parse(val)
	if err != nil {
		return "", fmt.Errorf("%s is not a valid URI", val)
	}

	if uri.Scheme != "http" && uri.Scheme != "https" {
		return "", fmt.Errorf("Unsupported scheme %s", uri.Scheme)
	}

	if uri.Path != "" || uri.RawQuery != "" || uri.Fragment != "" {
		return "", fmt.Errorf("Unsupported path/query/fragment at end of the URI")
	}

	return fmt.Sprintf("%s://%s/", uri.Scheme, uri.Host), nil
}

// ValidateIndexName validates an index name.
func ValidateIndexName(val string) (string, error) {
	if val == reference.LegacyDefaultHostname {
		val = reference.DefaultHostname
	}
	if strings.HasPrefix(val, "-") || strings.HasSuffix(val, "-") {
		return "", fmt.Errorf("Invalid index name (%s). Cannot begin or end with a hyphen.", val)
	}
	return val, nil
}

func validateNoSchema(reposName string) error {
	if strings.Contains(reposName, "://") {
		// It cannot contain a scheme!
		return ErrInvalidRepositoryName
	}
	return nil
}

// newIndexInfo returns IndexInfo configuration from indexName
func newIndexInfo(config *registrytypes.ServiceConfig, indexName string) (*registrytypes.IndexInfo, error) {
	var err error
	indexName, err = ValidateIndexName(indexName)
	if err != nil {
		return nil, err
	}

	// Return any configured index info, first.
	if index, ok := config.IndexConfigs[indexName]; ok {
		return index, nil
	}

	// Construct a non-configured index info.
	index := &registrytypes.IndexInfo{
		Name:     indexName,
		Mirrors:  make([]string, 0),
		Official: false,
	}
	index.Secure = isSecureIndex(config, indexName)
	return index, nil
}

// GetAuthConfigKey special-cases using the full index address of the official
// index as the AuthConfig key, and uses the (host)name[:port] for private indexes.
func GetAuthConfigKey(index *registrytypes.IndexInfo) string {
	if index.Official {
		return IndexServer
	}
	return index.Name
}

// newRepositoryInfo validates and breaks down a repository name into a RepositoryInfo
func newRepositoryInfo(config *registrytypes.ServiceConfig, name reference.Named) (*RepositoryInfo, error) {
	index, err := newIndexInfo(config, name.Hostname())
	if err != nil {
		return nil, err
	}
	official := !strings.ContainsRune(name.Name(), '/')
	return &RepositoryInfo{name, index, official}, nil
}

// ParseRepositoryInfo performs the breakdown of a repository name into a RepositoryInfo, but
// lacks registry configuration.
func ParseRepositoryInfo(reposName reference.Named) (*RepositoryInfo, error) {
	return newRepositoryInfo(emptyServiceConfig, reposName)
}

// ParseSearchIndexInfo will use repository name to get back an indexInfo.
func ParseSearchIndexInfo(reposName string) (*registrytypes.IndexInfo, error) {
	indexName, _ := splitReposSearchTerm(reposName)

	indexInfo, err := newIndexInfo(emptyServiceConfig, indexName)
	if err != nil {
		return nil, err
	}
	return indexInfo, nil
}
