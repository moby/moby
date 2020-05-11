package registry // import "github.com/docker/docker/registry"

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	reference "github.com/containerd/containerd/reference/docker"
	"github.com/docker/docker/api/types/registry"
	"github.com/sirupsen/logrus"
)

// ServiceOptions holds command line options.
type ServiceOptions struct {
	AllowNondistributableArtifacts []string `json:"allow-nondistributable-artifacts,omitempty"`
	Mirrors                        []string `json:"registry-mirrors,omitempty"`
	InsecureRegistries             []string `json:"insecure-registries,omitempty"`
}

// serviceConfig holds daemon configuration for the registry service.
type serviceConfig registry.ServiceConfig

// TODO(thaJeztah) both the "index.docker.io" and "registry-1.docker.io" domains
// are here for historic reasons and backward-compatibility. These domains
// are still supported by Docker Hub (and will continue to be supported), but
// there are new domains already in use, and plans to consolidate all legacy
// domains to new "canonical" domains. Once those domains are decided on, we
// should update these consts (but making sure to preserve compatibility with
// existing installs, clients, and user configuration).
const (
	// DefaultNamespace is the default namespace
	DefaultNamespace = "docker.io"
	// DefaultRegistryHost is the hostname for the default (Docker Hub) registry
	// used for pushing and pulling images. This hostname is hard-coded to handle
	// the conversion from image references without registry name (e.g. "ubuntu",
	// or "ubuntu:latest"), as well as references using the "docker.io" domain
	// name, which is used as canonical reference for images on Docker Hub, but
	// does not match the domain-name of Docker Hub's registry.
	DefaultRegistryHost = "registry-1.docker.io"
	// IndexHostname is the index hostname, used for authentication and image search.
	IndexHostname = "index.docker.io"
	// IndexServer is used for user auth and image search
	IndexServer = "https://" + IndexHostname + "/v1/"
	// IndexName is the name of the index
	IndexName = "docker.io"
)

var (
	// DefaultV2Registry is the URI of the default (Docker Hub) registry.
	DefaultV2Registry = &url.URL{
		Scheme: "https",
		Host:   DefaultRegistryHost,
	}

	emptyServiceConfig, _ = newServiceConfig(ServiceOptions{})
	validHostPortRegex    = regexp.MustCompile(`^` + reference.DomainRegexp.String() + `$`)

	// for mocking in unit tests
	lookupIP = net.LookupIP

	// certsDir is used to override defaultCertsDir.
	certsDir string
)

// SetCertsDir allows the default certs directory to be changed. This function
// is used at daemon startup to set the correct location when running in
// rootless mode.
func SetCertsDir(path string) {
	certsDir = path
}

// CertsDir is the directory where certificates are stored.
func CertsDir() string {
	if certsDir != "" {
		return certsDir
	}
	return defaultCertsDir
}

// newServiceConfig returns a new instance of ServiceConfig
func newServiceConfig(options ServiceOptions) (*serviceConfig, error) {
	config := &serviceConfig{}
	if err := config.loadAllowNondistributableArtifacts(options.AllowNondistributableArtifacts); err != nil {
		return nil, err
	}
	if err := config.loadMirrors(options.Mirrors); err != nil {
		return nil, err
	}
	if err := config.loadInsecureRegistries(options.InsecureRegistries); err != nil {
		return nil, err
	}

	return config, nil
}

// copy constructs a new ServiceConfig with a copy of the configuration in config.
func (config *serviceConfig) copy() *registry.ServiceConfig {
	ic := make(map[string]*registry.IndexInfo)
	for key, value := range config.IndexConfigs {
		ic[key] = value
	}
	return &registry.ServiceConfig{
		AllowNondistributableArtifactsCIDRs:     append([]*registry.NetIPNet(nil), config.AllowNondistributableArtifactsCIDRs...),
		AllowNondistributableArtifactsHostnames: append([]string(nil), config.AllowNondistributableArtifactsHostnames...),
		InsecureRegistryCIDRs:                   append([]*registry.NetIPNet(nil), config.InsecureRegistryCIDRs...),
		IndexConfigs:                            ic,
		Mirrors:                                 append([]string(nil), config.Mirrors...),
	}
}

// loadAllowNondistributableArtifacts loads allow-nondistributable-artifacts registries into config.
func (config *serviceConfig) loadAllowNondistributableArtifacts(registries []string) error {
	cidrs := map[string]*registry.NetIPNet{}
	hostnames := map[string]bool{}

	for _, r := range registries {
		if _, err := ValidateIndexName(r); err != nil {
			return err
		}
		if hasScheme(r) {
			return invalidParamf("allow-nondistributable-artifacts registry %s should not contain '://'", r)
		}

		if _, ipnet, err := net.ParseCIDR(r); err == nil {
			// Valid CIDR.
			cidrs[ipnet.String()] = (*registry.NetIPNet)(ipnet)
		} else if err = validateHostPort(r); err == nil {
			// Must be `host:port` if not CIDR.
			hostnames[r] = true
		} else {
			return invalidParamWrapf(err, "allow-nondistributable-artifacts registry %s is not valid", r)
		}
	}

	config.AllowNondistributableArtifactsCIDRs = make([]*registry.NetIPNet, 0, len(cidrs))
	for _, c := range cidrs {
		config.AllowNondistributableArtifactsCIDRs = append(config.AllowNondistributableArtifactsCIDRs, c)
	}

	config.AllowNondistributableArtifactsHostnames = make([]string, 0, len(hostnames))
	for h := range hostnames {
		config.AllowNondistributableArtifactsHostnames = append(config.AllowNondistributableArtifactsHostnames, h)
	}

	return nil
}

// loadMirrors loads mirrors to config, after removing duplicates.
// Returns an error if mirrors contains an invalid mirror.
func (config *serviceConfig) loadMirrors(mirrors []string) error {
	mMap := map[string]struct{}{}
	unique := []string{}

	for _, mirror := range mirrors {
		m, err := ValidateMirror(mirror)
		if err != nil {
			return err
		}
		if _, exist := mMap[m]; !exist {
			mMap[m] = struct{}{}
			unique = append(unique, m)
		}
	}

	config.Mirrors = unique

	// Configure public registry since mirrors may have changed.
	config.IndexConfigs = map[string]*registry.IndexInfo{
		IndexName: {
			Name:     IndexName,
			Mirrors:  unique,
			Secure:   true,
			Official: true,
		},
	}

	return nil
}

// loadInsecureRegistries loads insecure registries to config
func (config *serviceConfig) loadInsecureRegistries(registries []string) error {
	// Localhost is by default considered as an insecure registry. This is a
	// stop-gap for people who are running a private registry on localhost.
	registries = append(registries, "127.0.0.0/8")

	var (
		insecureRegistryCIDRs = make([]*registry.NetIPNet, 0)
		indexConfigs          = make(map[string]*registry.IndexInfo)
	)

skip:
	for _, r := range registries {
		// validate insecure registry
		if _, err := ValidateIndexName(r); err != nil {
			return err
		}
		if strings.HasPrefix(strings.ToLower(r), "http://") {
			logrus.Warnf("insecure registry %s should not contain 'http://' and 'http://' has been removed from the insecure registry config", r)
			r = r[7:]
		} else if strings.HasPrefix(strings.ToLower(r), "https://") {
			logrus.Warnf("insecure registry %s should not contain 'https://' and 'https://' has been removed from the insecure registry config", r)
			r = r[8:]
		} else if hasScheme(r) {
			return invalidParamf("insecure registry %s should not contain '://'", r)
		}
		// Check if CIDR was passed to --insecure-registry
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			// Valid CIDR. If ipnet is already in config.InsecureRegistryCIDRs, skip.
			data := (*registry.NetIPNet)(ipnet)
			for _, value := range insecureRegistryCIDRs {
				if value.IP.String() == data.IP.String() && value.Mask.String() == data.Mask.String() {
					continue skip
				}
			}
			// ipnet is not found, add it in config.InsecureRegistryCIDRs
			insecureRegistryCIDRs = append(insecureRegistryCIDRs, data)
		} else {
			if err := validateHostPort(r); err != nil {
				return invalidParamWrapf(err, "insecure registry %s is not valid", r)
			}
			// Assume `host:port` if not CIDR.
			indexConfigs[r] = &registry.IndexInfo{
				Name:     r,
				Mirrors:  make([]string, 0),
				Secure:   false,
				Official: false,
			}
		}
	}

	// Configure public registry.
	indexConfigs[IndexName] = &registry.IndexInfo{
		Name:     IndexName,
		Mirrors:  config.Mirrors,
		Secure:   true,
		Official: true,
	}
	config.InsecureRegistryCIDRs = insecureRegistryCIDRs
	config.IndexConfigs = indexConfigs

	return nil
}

// allowNondistributableArtifacts returns true if the provided hostname is part of the list of registries
// that allow push of nondistributable artifacts.
//
// The list can contain elements with CIDR notation to specify a whole subnet. If the subnet contains an IP
// of the registry specified by hostname, true is returned.
//
// hostname should be a URL.Host (`host:port` or `host`) where the `host` part can be either a domain name
// or an IP address. If it is a domain name, then it will be resolved to IP addresses for matching. If
// resolution fails, CIDR matching is not performed.
func (config *serviceConfig) allowNondistributableArtifacts(hostname string) bool {
	for _, h := range config.AllowNondistributableArtifactsHostnames {
		if h == hostname {
			return true
		}
	}

	return isCIDRMatch(config.AllowNondistributableArtifactsCIDRs, hostname)
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
func (config *serviceConfig) isSecureIndex(indexName string) bool {
	// Check for configured index, first.  This is needed in case isSecureIndex
	// is called from anything besides newIndexInfo, in order to honor per-index configurations.
	if index, ok := config.IndexConfigs[indexName]; ok {
		return index.Secure
	}

	return !isCIDRMatch(config.InsecureRegistryCIDRs, indexName)
}

// isCIDRMatch returns true if URLHost matches an element of cidrs. URLHost is a URL.Host (`host:port` or `host`)
// where the `host` part can be either a domain name or an IP address. If it is a domain name, then it will be
// resolved to IP addresses for matching. If resolution fails, false is returned.
func isCIDRMatch(cidrs []*registry.NetIPNet, URLHost string) bool {
	host, _, err := net.SplitHostPort(URLHost)
	if err != nil {
		// Assume URLHost is of the form `host` without the port and go on.
		host = URLHost
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
		for _, ipnet := range cidrs {
			// check if the addr falls in the subnet
			if (*net.IPNet)(ipnet).Contains(addr) {
				return true
			}
		}
	}

	return false
}

// ValidateMirror validates an HTTP(S) registry mirror
func ValidateMirror(val string) (string, error) {
	uri, err := url.Parse(val)
	if err != nil {
		return "", invalidParamWrapf(err, "invalid mirror: %q is not a valid URI", val)
	}
	if uri.Scheme != "http" && uri.Scheme != "https" {
		return "", invalidParamf("invalid mirror: unsupported scheme %q in %q", uri.Scheme, uri)
	}
	if (uri.Path != "" && uri.Path != "/") || uri.RawQuery != "" || uri.Fragment != "" {
		return "", invalidParamf("invalid mirror: path, query, or fragment at end of the URI %q", uri)
	}
	if uri.User != nil {
		// strip password from output
		uri.User = url.UserPassword(uri.User.Username(), "xxxxx")
		return "", invalidParamf("invalid mirror: username/password not allowed in URI %q", uri)
	}
	return strings.TrimSuffix(val, "/") + "/", nil
}

// ValidateIndexName validates an index name.
func ValidateIndexName(val string) (string, error) {
	// TODO: upstream this to check to reference package
	if val == "index.docker.io" {
		val = "docker.io"
	}
	if strings.HasPrefix(val, "-") || strings.HasSuffix(val, "-") {
		return "", invalidParamf("invalid index name (%s). Cannot begin or end with a hyphen", val)
	}
	return val, nil
}

func hasScheme(reposName string) bool {
	return strings.Contains(reposName, "://")
}

func validateHostPort(s string) error {
	// Split host and port, and in case s can not be splitted, assume host only
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		host = s
		port = ""
	}
	// If match against the `host:port` pattern fails,
	// it might be `IPv6:port`, which will be captured by net.ParseIP(host)
	if !validHostPortRegex.MatchString(s) && net.ParseIP(host) == nil {
		return invalidParamf("invalid host %q", host)
	}
	if port != "" {
		v, err := strconv.Atoi(port)
		if err != nil {
			return err
		}
		if v < 0 || v > 65535 {
			return invalidParamf("invalid port %q", port)
		}
	}
	return nil
}

// newIndexInfo returns IndexInfo configuration from indexName
func newIndexInfo(config *serviceConfig, indexName string) (*registry.IndexInfo, error) {
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
	return &registry.IndexInfo{
		Name:     indexName,
		Mirrors:  make([]string, 0),
		Secure:   config.isSecureIndex(indexName),
		Official: false,
	}, nil
}

// GetAuthConfigKey special-cases using the full index address of the official
// index as the AuthConfig key, and uses the (host)name[:port] for private indexes.
func GetAuthConfigKey(index *registry.IndexInfo) string {
	if index.Official {
		return IndexServer
	}
	return index.Name
}

// newRepositoryInfo validates and breaks down a repository name into a RepositoryInfo
func newRepositoryInfo(config *serviceConfig, name reference.Named) (*RepositoryInfo, error) {
	index, err := newIndexInfo(config, reference.Domain(name))
	if err != nil {
		return nil, err
	}
	official := !strings.ContainsRune(reference.FamiliarName(name), '/')

	return &RepositoryInfo{
		Name:     reference.TrimNamed(name),
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
//
// TODO(thaJeztah) this function is only used by the CLI, and used to get
// information of the registry (to provide credentials if needed). We should
// move this function (or equivalent) to the CLI, as it's doing too much just
// for that.
func ParseSearchIndexInfo(reposName string) (*registry.IndexInfo, error) {
	indexName, _ := splitReposSearchTerm(reposName)

	indexInfo, err := newIndexInfo(emptyServiceConfig, indexName)
	if err != nil {
		return nil, err
	}
	return indexInfo, nil
}
