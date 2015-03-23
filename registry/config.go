package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

// Options holds command line options.
type Options struct {
	Mirrors            opts.ListOpts
	InsecureRegistries opts.ListOpts
}

const (
	// Only used for user auth + account creation
	INDEXSERVER    = "https://index.docker.io/v1/"
	REGISTRYSERVER = "https://registry-1.docker.io/v2/"
	INDEXNAME      = "docker.io"

	// INDEXSERVER = "https://registry-stage.hub.docker.com/v1/"
)

var (
	ErrInvalidRepositoryName = errors.New("Invalid repository name (ex: \"registry.domain.tld/myrepos\")")
	emptyServiceConfig       = NewServiceConfig(nil)
	validNamespaceChars      = regexp.MustCompile(`^([a-z0-9-_]*)$`)
	validRepo                = regexp.MustCompile(`^([a-z0-9-_.]+)$`)
)

func IndexServerAddress() string {
	return INDEXSERVER
}

func IndexServerName() string {
	return INDEXNAME
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
func (options *Options) InstallFlags() {
	options.Mirrors = opts.NewListOpts(ValidateMirror)
	flag.Var(&options.Mirrors, []string{"-registry-mirror"}, "Preferred Docker registry mirror")
	options.InsecureRegistries = opts.NewListOpts(ValidateIndexName)
	flag.Var(&options.InsecureRegistries, []string{"-insecure-registry"}, "Enable insecure registry communication")
}

type netIPNet net.IPNet

func (ipnet *netIPNet) MarshalJSON() ([]byte, error) {
	return json.Marshal((*net.IPNet)(ipnet).String())
}

func (ipnet *netIPNet) UnmarshalJSON(b []byte) (err error) {
	var ipnet_str string
	if err = json.Unmarshal(b, &ipnet_str); err == nil {
		var cidr *net.IPNet
		if _, cidr, err = net.ParseCIDR(ipnet_str); err == nil {
			*ipnet = netIPNet(*cidr)
		}
	}
	return
}

// ServiceConfig stores daemon registry services configuration.
type ServiceConfig struct {
	InsecureRegistryCIDRs []*netIPNet           `json:"InsecureRegistryCIDRs"`
	IndexConfigs          map[string]*IndexInfo `json:"IndexConfigs"`
}

// NewServiceConfig returns a new instance of ServiceConfig
func NewServiceConfig(options *Options) *ServiceConfig {
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

	config := &ServiceConfig{
		InsecureRegistryCIDRs: make([]*netIPNet, 0),
		IndexConfigs:          make(map[string]*IndexInfo, 0),
	}
	// Split --insecure-registry into CIDR and registry-specific settings.
	for _, r := range options.InsecureRegistries.GetAll() {
		// Check if CIDR was passed to --insecure-registry
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			// Valid CIDR.
			config.InsecureRegistryCIDRs = append(config.InsecureRegistryCIDRs, (*netIPNet)(ipnet))
		} else {
			// Assume `host:port` if not CIDR.
			config.IndexConfigs[r] = &IndexInfo{
				Name:     r,
				Mirrors:  make([]string, 0),
				Secure:   false,
				Official: false,
			}
		}
	}

	// Configure public registry.
	config.IndexConfigs[IndexServerName()] = &IndexInfo{
		Name:     IndexServerName(),
		Mirrors:  options.Mirrors.GetAll(),
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
func (config *ServiceConfig) isSecureIndex(indexName string) bool {
	// Check for configured index, first.  This is needed in case isSecureIndex
	// is called from anything besides NewIndexInfo, in order to honor per-index configurations.
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

	return fmt.Sprintf("%s://%s/v1/", uri.Scheme, uri.Host), nil
}

// ValidateIndexName validates an index name.
func ValidateIndexName(val string) (string, error) {
	// 'index.docker.io' => 'docker.io'
	if val == "index."+IndexServerName() {
		val = IndexServerName()
	}
	// *TODO: Check if valid hostname[:port]/ip[:port]?
	return val, nil
}

func validateRemoteName(remoteName string) error {
	var (
		namespace string
		name      string
	)
	nameParts := strings.SplitN(remoteName, "/", 2)
	if len(nameParts) < 2 {
		namespace = "library"
		name = nameParts[0]

		// the repository name must not be a valid image ID
		if err := utils.ValidateID(name); err == nil {
			return fmt.Errorf("Invalid repository name (%s), cannot specify 64-byte hexadecimal strings", name)
		}
	} else {
		namespace = nameParts[0]
		name = nameParts[1]
	}
	if !validNamespaceChars.MatchString(namespace) {
		return fmt.Errorf("Invalid namespace name (%s). Only [a-z0-9-_] are allowed.", namespace)
	}
	if len(namespace) < 2 || len(namespace) > 255 {
		return fmt.Errorf("Invalid namespace name (%s). Cannot be fewer than 2 or more than 255 characters.", namespace)
	}
	if strings.HasPrefix(namespace, "-") || strings.HasSuffix(namespace, "-") {
		return fmt.Errorf("Invalid namespace name (%s). Cannot begin or end with a hyphen.", namespace)
	}
	if strings.Contains(namespace, "--") {
		return fmt.Errorf("Invalid namespace name (%s). Cannot contain consecutive hyphens.", namespace)
	}
	if !validRepo.MatchString(name) {
		return fmt.Errorf("Invalid repository name (%s), only [a-z0-9-_.] are allowed", name)
	}
	return nil
}

func validateNoSchema(reposName string) error {
	if strings.Contains(reposName, "://") {
		// It cannot contain a scheme!
		return ErrInvalidRepositoryName
	}
	return nil
}

// ValidateRepositoryName validates a repository name
func ValidateRepositoryName(reposName string) error {
	var err error
	if err = validateNoSchema(reposName); err != nil {
		return err
	}
	indexName, remoteName := splitReposName(reposName)
	if _, err = ValidateIndexName(indexName); err != nil {
		return err
	}
	return validateRemoteName(remoteName)
}

// NewIndexInfo returns IndexInfo configuration from indexName
func (config *ServiceConfig) NewIndexInfo(indexName string) (*IndexInfo, error) {
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
	index := &IndexInfo{
		Name:     indexName,
		Mirrors:  make([]string, 0),
		Official: false,
	}
	index.Secure = config.isSecureIndex(indexName)
	return index, nil
}

// GetAuthConfigKey special-cases using the full index address of the official
// index as the AuthConfig key, and uses the (host)name[:port] for private indexes.
func (index *IndexInfo) GetAuthConfigKey() string {
	if index.Official {
		return IndexServerAddress()
	}
	return index.Name
}

// splitReposName breaks a reposName into an index name and remote name
func splitReposName(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var indexName, remoteName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		indexName = IndexServerName()
		remoteName = reposName
	} else {
		indexName = nameParts[0]
		remoteName = nameParts[1]
	}
	return indexName, remoteName
}

// NewRepositoryInfo validates and breaks down a repository name into a RepositoryInfo
func (config *ServiceConfig) NewRepositoryInfo(reposName string) (*RepositoryInfo, error) {
	if err := validateNoSchema(reposName); err != nil {
		return nil, err
	}

	indexName, remoteName := splitReposName(reposName)
	if err := validateRemoteName(remoteName); err != nil {
		return nil, err
	}

	repoInfo := &RepositoryInfo{
		RemoteName: remoteName,
	}

	var err error
	repoInfo.Index, err = config.NewIndexInfo(indexName)
	if err != nil {
		return nil, err
	}

	if repoInfo.Index.Official {
		normalizedName := repoInfo.RemoteName
		if strings.HasPrefix(normalizedName, "library/") {
			// If pull "library/foo", it's stored locally under "foo"
			normalizedName = strings.SplitN(normalizedName, "/", 2)[1]
		}

		repoInfo.LocalName = normalizedName
		repoInfo.RemoteName = normalizedName
		// If the normalized name does not contain a '/' (e.g. "foo")
		// then it is an official repo.
		if strings.IndexRune(normalizedName, '/') == -1 {
			repoInfo.Official = true
			// Fix up remote name for official repos.
			repoInfo.RemoteName = "library/" + normalizedName
		}

		// *TODO: Prefix this with 'docker.io/'.
		repoInfo.CanonicalName = repoInfo.LocalName
	} else {
		// *TODO: Decouple index name from hostname (via registry configuration?)
		repoInfo.LocalName = repoInfo.Index.Name + "/" + repoInfo.RemoteName
		repoInfo.CanonicalName = repoInfo.LocalName
	}
	return repoInfo, nil
}

// GetSearchTerm special-cases using local name for official index, and
// remote name for private indexes.
func (repoInfo *RepositoryInfo) GetSearchTerm() string {
	if repoInfo.Index.Official {
		return repoInfo.LocalName
	}
	return repoInfo.RemoteName
}

// ParseRepositoryInfo performs the breakdown of a repository name into a RepositoryInfo, but
// lacks registry configuration.
func ParseRepositoryInfo(reposName string) (*RepositoryInfo, error) {
	return emptyServiceConfig.NewRepositoryInfo(reposName)
}

// NormalizeLocalName transforms a repository name into a normalize LocalName
// Passes through the name without transformation on error (image id, etc)
func NormalizeLocalName(name string) string {
	repoInfo, err := ParseRepositoryInfo(name)
	if err != nil {
		return name
	}
	return repoInfo.LocalName
}
