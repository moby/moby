package main

import (
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/registry"
	"github.com/spf13/pflag"
)

// defaultTrustKeyFile is the default filename for the trust key
const defaultTrustKeyFile = "key.json"

// installCommonConfigFlags adds flags to the pflag.FlagSet to configure the daemon
func installCommonConfigFlags(conf *config.Config, flags *pflag.FlagSet) error {
	var err error
	conf.Pidfile, err = getDefaultPidFile()
	if err != nil {
		return err
	}
	conf.Root, err = getDefaultDataRoot()
	if err != nil {
		return err
	}
	conf.ExecRoot, err = getDefaultExecRoot()
	if err != nil {
		return err
	}

	var (
		allowNonDistributable = opts.NewNamedListOptsRef("allow-nondistributable-artifacts", &conf.AllowNondistributableArtifacts, registry.ValidateIndexName)
		registryMirrors       = opts.NewNamedListOptsRef("registry-mirrors", &conf.Mirrors, registry.ValidateMirror)
		insecureRegistries    = opts.NewNamedListOptsRef("insecure-registries", &conf.InsecureRegistries, registry.ValidateIndexName)
	)
	flags.Var(allowNonDistributable, "allow-nondistributable-artifacts", "Allow push of nondistributable artifacts to registry")
	flags.Var(registryMirrors, "registry-mirror", "Preferred Docker registry mirror")
	flags.Var(insecureRegistries, "insecure-registry", "Enable insecure registry communication")

	flags.Var(opts.NewNamedListOptsRef("storage-opts", &conf.GraphOptions, nil), "storage-opt", "Storage driver options")
	flags.Var(opts.NewNamedListOptsRef("authorization-plugins", &conf.AuthorizationPlugins, nil), "authorization-plugin", "Authorization plugins to load")
	flags.Var(opts.NewNamedListOptsRef("exec-opts", &conf.ExecOptions, nil), "exec-opt", "Runtime execution options")
	flags.StringVarP(&conf.Pidfile, "pidfile", "p", conf.Pidfile, "Path to use for daemon PID file")
	flags.StringVar(&conf.Root, "data-root", conf.Root, "Root directory of persistent Docker state")
	flags.StringVar(&conf.ExecRoot, "exec-root", conf.ExecRoot, "Root directory for execution state files")
	flags.StringVar(&conf.ContainerdAddr, "containerd", "", "containerd grpc address")
	flags.BoolVar(&conf.CriContainerd, "cri-containerd", false, "start containerd with cri")

	flags.IntVar(&conf.Mtu, "mtu", conf.Mtu, "Set the containers network MTU")
	flags.IntVar(&conf.NetworkControlPlaneMTU, "network-control-plane-mtu", conf.NetworkControlPlaneMTU, "Network Control plane MTU")
	flags.IntVar(&conf.NetworkDiagnosticPort, "network-diagnostic-port", 0, "TCP port number of the network diagnostic server")
	_ = flags.MarkHidden("network-diagnostic-port")

	flags.BoolVar(&conf.RawLogs, "raw-logs", false, "Full timestamps without ANSI coloring")
	flags.Var(opts.NewListOptsRef(&conf.DNS, opts.ValidateIPAddress), "dns", "DNS server to use")
	flags.Var(opts.NewNamedListOptsRef("dns-opts", &conf.DNSOptions, nil), "dns-opt", "DNS options to use")
	flags.Var(opts.NewListOptsRef(&conf.DNSSearch, opts.ValidateDNSSearch), "dns-search", "DNS search domains to use")
	flags.IPVar(&conf.HostGatewayIP, "host-gateway-ip", nil, "IP address that the special 'host-gateway' string in --add-host resolves to. Defaults to the IP address of the default bridge")
	flags.Var(opts.NewNamedListOptsRef("labels", &conf.Labels, opts.ValidateLabel), "label", "Set key=value labels to the daemon")
	flags.StringVar(&conf.LogConfig.Type, "log-driver", "json-file", "Default driver for container logs")
	flags.Var(opts.NewNamedMapOpts("log-opts", conf.LogConfig.Config, nil), "log-opt", "Default log driver options for containers")

	flags.StringVar(&conf.CorsHeaders, "api-cors-header", "", "Set CORS headers in the Engine API")
	flags.IntVar(&conf.MaxConcurrentDownloads, "max-concurrent-downloads", conf.MaxConcurrentDownloads, "Set the max concurrent downloads for each pull")
	flags.IntVar(&conf.MaxConcurrentUploads, "max-concurrent-uploads", conf.MaxConcurrentUploads, "Set the max concurrent uploads for each push")
	flags.IntVar(&conf.MaxDownloadAttempts, "max-download-attempts", conf.MaxDownloadAttempts, "Set the max download attempts for each pull")
	flags.IntVar(&conf.ShutdownTimeout, "shutdown-timeout", conf.ShutdownTimeout, "Set the default shutdown timeout")

	flags.StringVar(&conf.SwarmDefaultAdvertiseAddr, "swarm-default-advertise-addr", "", "Set default address or interface for swarm advertised address")
	flags.BoolVar(&conf.Experimental, "experimental", false, "Enable experimental features")
	flags.StringVar(&conf.MetricsAddress, "metrics-addr", "", "Set default address and port to serve the metrics api on")
	flags.Var(opts.NewNamedListOptsRef("node-generic-resources", &conf.NodeGenericResources, opts.ValidateSingleGenericResource), "node-generic-resource", "Advertise user-defined resource")

	flags.StringVar(&conf.ContainerdNamespace, "containerd-namespace", conf.ContainerdNamespace, "Containerd namespace to use")
	flags.StringVar(&conf.ContainerdPluginNamespace, "containerd-plugins-namespace", conf.ContainerdPluginNamespace, "Containerd namespace to use for plugins")
	flags.StringVar(&conf.DefaultRuntime, "default-runtime", conf.DefaultRuntime, "Default OCI runtime for containers")

	flags.StringVar(&conf.HTTPProxy, "http-proxy", "", "HTTP proxy URL to use for outgoing traffic")
	flags.StringVar(&conf.HTTPSProxy, "https-proxy", "", "HTTPS proxy URL to use for outgoing traffic")
	flags.StringVar(&conf.NoProxy, "no-proxy", "", "Comma-separated list of hosts or IP addresses for which the proxy is skipped")

	// Deprecated flags / options

	// "--graph" is "soft-deprecated" in favor of "data-root". This flag was added
	// before Docker 1.0, so won't be removed, only hidden, to discourage its usage.
	flags.StringVarP(&conf.Root, "graph", "g", conf.Root, "Root of the Docker runtime")
	_ = flags.MarkHidden("graph")
	flags.BoolVarP(&conf.AutoRestart, "restart", "r", true, "--restart on the daemon has been deprecated in favor of --restart policies on docker run")
	_ = flags.MarkDeprecated("restart", "Please use a restart policy on docker run")
	flags.StringVar(&conf.ClusterAdvertise, "cluster-advertise", "", "Address or interface name to advertise")
	_ = flags.MarkDeprecated("cluster-advertise", "Swarm classic is deprecated. Please use Swarm-mode (docker swarm init)")
	flags.StringVar(&conf.ClusterStore, "cluster-store", "", "URL of the distributed storage backend")
	_ = flags.MarkDeprecated("cluster-store", "Swarm classic is deprecated. Please use Swarm-mode (docker swarm init)")
	flags.Var(opts.NewNamedMapOpts("cluster-store-opts", conf.ClusterOpts, nil), "cluster-store-opt", "Set cluster store options")
	_ = flags.MarkDeprecated("cluster-store-opt", "Swarm classic is deprecated. Please use Swarm-mode (docker swarm init)")

	return nil
}
