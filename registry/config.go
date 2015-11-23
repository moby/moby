package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/docker/image"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
)

// Options holds command line options.
type Options struct {
	Mirrors            opts.ListOpts
	InsecureRegistries opts.ListOpts
}

const (
	// DefaultNamespace is the default namespace
	DefaultNamespace = "docker.io"
	// DefaultRegistryVersionHeader is the name of the default HTTP header
	// that carries Registry version info
	DefaultRegistryVersionHeader = "Docker-Distribution-Api-Version"

	// IndexServer is the v1 registry server used for user auth + account creation
	IndexServer = DefaultV1Registry + "/v1/"
	// IndexName is the name of the index
	IndexName = "docker.io"

	// NotaryServer is the endpoint serving the Notary trust server
	NotaryServer = "https://notary.docker.io"

	// IndexServer = "https://registry-stage.hub.docker.com/v1/"
)

var (
	// BlockedRegistries is a set of registries that can't be contacted. A
	// special entry "*" causes all registries but those present in
	// RegistryList to be blocked.
	BlockedRegistries map[string]struct{}
	// RegistryList is a list of default registries..
	RegistryList = []string{IndexName}
	// ErrInvalidRepositoryName is an error returned if the repository name did
	// not have the correct form
	ErrInvalidRepositoryName = errors.New("Invalid repository name (ex: \"registry.domain.tld/myrepos\")")

	emptyServiceConfig = NewServiceConfig(nil)

	// V2Only controls access to legacy registries.  If it is set to true via the
	// command line flag the daemon will not attempt to contact v1 legacy registries
	V2Only = false
)

func init() {
	BlockedRegistries = make(map[string]struct{})
}

// IndexServerName returns the name of default index server.
func IndexServerName() string {
	if len(RegistryList) < 1 {
		return ""
	}
	return RegistryList[0]
}

// IndexServerAddress returns index uri of default registry.
func IndexServerAddress() string {
	if IndexServerName() == IndexName {
		return IndexServer
	} else if IndexServerName() == "" {
		return ""
	} else {
		return fmt.Sprintf("https://%s/v1/", IndexServerName())
	}
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
func (options *Options) InstallFlags(cmd *flag.FlagSet, usageFn func(string) string) {
	options.Mirrors = opts.NewListOpts(ValidateMirror)
	cmd.Var(&options.Mirrors, []string{"-registry-mirror"}, usageFn("Preferred Docker registry mirror"))
	options.InsecureRegistries = opts.NewListOpts(ValidateIndexName)
	cmd.Var(&options.InsecureRegistries, []string{"-insecure-registry"}, usageFn("Enable insecure registry communication"))
	cmd.BoolVar(&V2Only, []string{"-disable-legacy-registry"}, false, "Do not contact legacy registries")
}

type netIPNet net.IPNet

func (ipnet *netIPNet) MarshalJSON() ([]byte, error) {
	return json.Marshal((*net.IPNet)(ipnet).String())
}

func (ipnet *netIPNet) UnmarshalJSON(b []byte) (err error) {
	var ipnetStr string
	if err = json.Unmarshal(b, &ipnetStr); err == nil {
		var cidr *net.IPNet
		if _, cidr, err = net.ParseCIDR(ipnetStr); err == nil {
			*ipnet = netIPNet(*cidr)
		}
	}
	return
}

// ServiceConfig stores daemon registry services configuration.
type ServiceConfig struct {
	InsecureRegistryCIDRs []*netIPNet           `json:"InsecureRegistryCIDRs"`
	IndexConfigs          map[string]*IndexInfo `json:"IndexConfigs"`
	Mirrors               []string
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

	for _, r := range RegistryList {
		var mirrors []string
		if config.IndexConfigs[r] == nil {
			// Use mirrors only with official index
			if r == IndexName {
				mirrors = config.Mirrors
			} else {
				mirrors = make([]string, 0)
			}
			config.IndexConfigs[r] = &IndexInfo{
				Name:     r,
				Mirrors:  mirrors,
				Secure:   config.isSecureIndex(r),
				Official: r == IndexName,
			}
		}
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
	if indexName == IndexName {
		return true
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
	// 'index.docker.io' => 'docker.io'
	if val == "index."+IndexName {
		val = IndexName
	}
	for _, r := range RegistryList {
		if val == r {
			break
		}
		if val == "index."+r {
			val = r
		}
	}
	if strings.HasPrefix(val, "-") || strings.HasSuffix(val, "-") {
		return "", fmt.Errorf("Invalid index name (%s). Cannot begin or end with a hyphen.", val)
	}
	// *TODO: Check if valid hostname[:port]/ip[:port]?
	return val, nil
}

func validateRemoteName(remoteName string) error {

	if !strings.Contains(remoteName, "/") {

		// the repository name must not be a valid image ID
		if err := image.ValidateID(remoteName); err == nil {
			return fmt.Errorf("Invalid repository name (%s), cannot specify 64-byte hexadecimal strings", remoteName)
		}
	}

	return v2.ValidateRepositoryName(remoteName)
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
	_, _, err := loadRepositoryName(reposName, true)
	return err
}

// loadRepositoryName returns the repo name splitted into index name
// and remote repo name. It returns an error if the name is not valid.
func loadRepositoryName(reposName string, checkRemoteName bool) (string, string, error) {
	if err := validateNoSchema(reposName); err != nil {
		return "", "", err
	}
	indexName, remoteName := SplitReposName(reposName, true)

	var err error
	if indexName, err = ValidateIndexName(indexName); err != nil {
		return "", "", err
	}
	if checkRemoteName {
		if err = validateRemoteName(remoteName); err != nil {
			return "", "", err
		}
	}
	return indexName, remoteName, nil
}

// RepositoryNameHasIndex determines whether the given reposName has prepended
// name of index.
func RepositoryNameHasIndex(reposName string) bool {
	indexName, _ := SplitReposName(reposName, false)
	return indexName != ""
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
		Official: indexName == IndexName,
		Secure:   config.isSecureIndex(indexName),
	}
	return index, nil
}

// GetAuthConfigKey special-cases using the full index address of the official
// index as the AuthConfig key, and uses the (host)name[:port] for private indexes.
func (index *IndexInfo) GetAuthConfigKey() string {
	if index.Official {
		return IndexServer
	}
	return index.Name
}

// SplitReposName breaks a reposName into an index name and remote name
// fixMissingIndex says to return current index server name if missing in
// reposName
func SplitReposName(reposName string, fixMissingIndex bool) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var indexName, remoteName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		if fixMissingIndex {
			indexName = IndexServerName()
		}
		remoteName = reposName
	} else {
		indexName = nameParts[0]
		remoteName = nameParts[1]
	}
	return indexName, remoteName
}

// IsIndexBlocked allows to check whether index/registry or endpoint
// is on a block list.
func IsIndexBlocked(indexName string) bool {
	if _, ok := BlockedRegistries[indexName]; ok {
		return true
	}
	if _, ok := BlockedRegistries["*"]; ok {
		for _, name := range RegistryList {
			if indexName == name {
				return false
			}
		}
		return true
	}
	return false
}

// NewRepositoryInfo validates and breaks down a repository name into a RepositoryInfo
func (config *ServiceConfig) NewRepositoryInfo(reposName string, bySearch bool) (*RepositoryInfo, error) {
	indexName, remoteName, err := loadRepositoryName(reposName, !bySearch)
	if err != nil {
		return nil, err
	}

	if indexName == "" {
		indexName = IndexServerName()
	}

	if IsIndexBlocked(indexName) {
		return nil, fmt.Errorf("Blocked registry %q", indexName)
	}

	repoInfo := &RepositoryInfo{
		RemoteName: remoteName,
	}

	repoInfo.Index, err = config.NewIndexInfo(indexName)
	if err != nil {
		return nil, err
	}

	if repoInfo.Index.Official {
		normalizedName := normalizeLibraryRepoName(repoInfo.RemoteName)
		repoInfo.RemoteName = normalizedName
		// If the normalized name does not contain a '/' (e.g. "foo")
		// then it is an official repo.
		if strings.IndexRune(normalizedName, '/') == -1 {
			repoInfo.Official = true
			// Fix up remote name for official repos.
			repoInfo.RemoteName = "library/" + normalizedName
		}

		repoInfo.CanonicalName = IndexName + "/" + repoInfo.RemoteName
		repoInfo.LocalName = repoInfo.Index.Name + "/" + normalizedName
	} else {
		if repoInfo.Index.Name != "" {
			repoInfo.LocalName = repoInfo.Index.Name + "/" + repoInfo.RemoteName
		} else {
			repoInfo.LocalName = repoInfo.RemoteName
		}
		repoInfo.CanonicalName = repoInfo.LocalName
	}
	return repoInfo, nil
}

// GetSearchTerm special-cases using local name for official index, and
// remote name for private indexes.
func (repoInfo *RepositoryInfo) GetSearchTerm() string {
	if repoInfo.Index.Official {
		return strings.TrimPrefix(repoInfo.RemoteName, "library/")
	}
	return repoInfo.RemoteName
}

// ParseRepositoryInfo performs the breakdown of a repository name into a RepositoryInfo, but
// lacks registry configuration.
func ParseRepositoryInfo(reposName string) (*RepositoryInfo, error) {
	return emptyServiceConfig.NewRepositoryInfo(reposName, false)
}

// ParseIndexInfo will use repository name to get back an indexInfo.
func ParseIndexInfo(reposName string) (*IndexInfo, error) {
	indexName, _ := SplitReposName(reposName, true)

	indexInfo, err := emptyServiceConfig.NewIndexInfo(indexName)
	if err != nil {
		return nil, err
	}
	return indexInfo, nil
}

// NormalizeLocalName transforms a repository name into a normalize LocalName
// Passes through the name without transformation on error (image id, etc)
// It does not use the repository info because we don't want to load
// the repository index and do request over the network.
func NormalizeLocalName(name string) string {
	indexName, remoteName, err := loadRepositoryName(name, true)
	if err != nil {
		return name
	}

	if indexName == "" {
		indexName = IndexServerName()
	}

	var officialIndex bool
	// Return any configured index info, first.
	if index, ok := emptyServiceConfig.IndexConfigs[indexName]; ok {
		officialIndex = index.Official
	}

	if officialIndex {
		return IndexName + "/" + normalizeLibraryRepoName(remoteName)
	}
	return localNameFromRemote(indexName, remoteName)
}

// normalizeLibraryRepoName removes the library prefix from
// the repository name for official repos. It preserves index name if
// it's part of the name.
func normalizeLibraryRepoName(name string) string {
	indexName, remoteName := SplitReposName(name, false)
	if strings.HasPrefix(remoteName, "library/") {
		// If pull "library/foo", it's stored locally under "docker.io/foo"
		remoteName = strings.SplitN(name, "/", 2)[1]
	}
	if indexName == "" {
		return remoteName
	}
	return indexName + "/" + remoteName
}

// localNameFromRemote combines the index name and the repo remote name
// to generate a repo local name.
func localNameFromRemote(indexName, remoteName string) string {
	if indexName == "" {
		indexName = IndexServerName()
	}
	if indexName == "" {
		return remoteName
	}
	return indexName + "/" + remoteName
}
