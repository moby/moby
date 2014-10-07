package registry

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
)

// Options holds command line options.
type Options struct {
	Mirrors            opts.ListOpts
	InsecureRegistries opts.ListOpts
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
func (options *Options) InstallFlags() {
	options.Mirrors = opts.NewListOpts(ValidateMirror)
	flag.Var(&options.Mirrors, []string{"-registry-mirror"}, "Specify a preferred Docker registry mirror")
	options.InsecureRegistries = opts.NewListOpts(ValidateIndexName)
	flag.Var(&options.InsecureRegistries, []string{"-insecure-registry"}, "Enable insecure communication with specified registries (no certificate verification for HTTPS and enable HTTP fallback) (e.g., localhost:5000 or 10.20.0.0/16)")
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
