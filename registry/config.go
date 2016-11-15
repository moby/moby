package registry

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/reference"
	"github.com/spf13/pflag"
)

// ServiceOptions holds command line options.
type ServiceOptions struct {
	Mirrors            []string `json:"registry-mirrors,omitempty"`
	InsecureRegistries []string `json:"insecure-registries,omitempty"`

	// V2Only controls access to legacy registries.  If it is set to true via the
	// command line flag the daemon will not attempt to contact v1 legacy registries
	V2Only bool `json:"disable-legacy-registry,omitempty"`
}

// serviceConfig holds daemon configuration for the registry service.
type serviceConfig struct {
	registrytypes.ServiceConfig
	V2Only bool
}

var (
	// DefaultNamespace is the default namespace
	DefaultNamespace = "docker.io"
	// DefaultRegistryVersionHeader is the name of the default HTTP header
	// that carries Registry version info
	DefaultRegistryVersionHeader = "Docker-Distribution-Api-Version"

	// IndexHostname is the index hostname
	IndexHostname = "index.docker.io"
	// IndexServer is used for user auth and image search
	IndexServer = "https://" + IndexHostname + "/v1/"
	// IndexName is the name of the index
	IndexName = "docker.io"

	// NotaryServer is the endpoint serving the Notary trust server
	NotaryServer = "https://notary.docker.io"

	// DefaultV2Registry is the URI of the default v2 registry
	DefaultV2Registry = &url.URL{
		Scheme: "https",
		Host:   "registry-1.docker.io",
	}
)

var (
	// ErrInvalidRepositoryName is an error returned if the repository name did
	// not have the correct form
	ErrInvalidRepositoryName = errors.New("Invalid repository name (ex: \"registry.domain.tld/myrepos\")")

	emptyServiceConfig = newServiceConfig(ServiceOptions{})
)

// for mocking in unit tests
var lookupIP = net.LookupIP

// InstallCliFlags adds command-line options to the top-level flag parser for
// the current process.
func (options *ServiceOptions) InstallCliFlags(flags *pflag.FlagSet) {
	mirrors := opts.NewNamedListOptsRef("registry-mirrors", &options.Mirrors, ValidateMirror)
	insecureRegistries := opts.NewNamedListOptsRef("insecure-registries", &options.InsecureRegistries, ValidateIndexName)

	flags.Var(mirrors, "registry-mirror", "Preferred Docker registry mirror")
	flags.Var(insecureRegistries, "insecure-registry", "Enable insecure registry communication")

	options.installCliPlatformFlags(flags)
}

// newServiceConfig returns a new instance of ServiceConfig
func newServiceConfig(options ServiceOptions) *serviceConfig {
	config := &serviceConfig{
		ServiceConfig: registrytypes.ServiceConfig{
			InsecureRegistryCIDRs: make([]*registrytypes.NetIPNet, 0),
			IndexConfigs:          make(map[string]*registrytypes.IndexInfo, 0),
			// Hack: Bypass setting the mirrors to IndexConfigs since they are going away
			// and Mirrors are only for the official registry anyways.
			Mirrors: options.Mirrors,
		},
		V2Only: options.V2Only,
	}

	config.LoadInsecureRegistries(options.InsecureRegistries)

	return config
}

// LoadInsecureRegistries loads insecure registries to config
func (config *serviceConfig) LoadInsecureRegistries(registries []string) error {
	// Localhost is by default considered as an insecure registry
	// This is a stop-gap for people who are running a private registry on localhost (especially on Boot2docker).
	//
	// TODO: should we deprecate this once it is easier for people to set up a TLS registry or change
	// daemon flags on boot2docker?
	registries = append(registries, "127.0.0.0/8")

	// Store original InsecureRegistryCIDRs and IndexConfigs
	// Clean InsecureRegistryCIDRs and IndexConfigs in config, as passed registries has all insecure registry info.
	originalCIDRs := config.ServiceConfig.InsecureRegistryCIDRs
	originalIndexInfos := config.ServiceConfig.IndexConfigs

	config.ServiceConfig.InsecureRegistryCIDRs = make([]*registrytypes.NetIPNet, 0)
	config.ServiceConfig.IndexConfigs = make(map[string]*registrytypes.IndexInfo, 0)

skip:
	for _, r := range registries {
		// validate insecure registry
		if _, err := ValidateIndexName(r); err != nil {
			// before returning err, roll back to original data
			config.ServiceConfig.InsecureRegistryCIDRs = originalCIDRs
			config.ServiceConfig.IndexConfigs = originalIndexInfos
			return err
		}
		// Check if CIDR was passed to --insecure-registry
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			// Valid CIDR. If ipnet is already in config.InsecureRegistryCIDRs, skip.
			data := (*registrytypes.NetIPNet)(ipnet)
			for _, value := range config.InsecureRegistryCIDRs {
				if value.IP.String() == data.IP.String() && value.Mask.String() == data.Mask.String() {
					continue skip
				}
			}
			// ipnet is not found, add it in config.InsecureRegistryCIDRs
			config.InsecureRegistryCIDRs = append(config.InsecureRegistryCIDRs, data)

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

	return nil
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
func isSecureIndex(config *serviceConfig, indexName string) bool {
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

func validateNoScheme(reposName string) error {
	if strings.Contains(reposName, "://") {
		// It cannot contain a scheme!
		return ErrInvalidRepositoryName
	}
	return nil
}

// newIndexInfo returns IndexInfo configuration from indexName
func newIndexInfo(config *serviceConfig, indexName string) (*registrytypes.IndexInfo, error) {
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
func newRepositoryInfo(config *serviceConfig, name reference.Named) (*RepositoryInfo, error) {
	index, err := newIndexInfo(config, name.Hostname())
	if err != nil {
		return nil, err
	}
	official := !strings.ContainsRune(name.Name(), '/')
	return &RepositoryInfo{
		Named:    name,
		Index:    index,
		Official: official,
	}, nil
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
