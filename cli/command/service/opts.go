package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/defaults"
	shlex "github.com/flynn-archive/go-shlex"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
)

type int64Value interface {
	Value() int64
}

// PositiveDurationOpt is an option type for time.Duration that uses a pointer.
// It bahave similarly to DurationOpt but only allows positive duration values.
type PositiveDurationOpt struct {
	DurationOpt
}

// Set a new value on the option. Setting a negative duration value will cause
// an error to be returned.
func (d *PositiveDurationOpt) Set(s string) error {
	err := d.DurationOpt.Set(s)
	if err != nil {
		return err
	}
	if *d.DurationOpt.value < 0 {
		return errors.Errorf("duration cannot be negative")
	}
	return nil
}

// DurationOpt is an option type for time.Duration that uses a pointer. This
// allows us to get nil values outside, instead of defaulting to 0
type DurationOpt struct {
	value *time.Duration
}

// Set a new value on the option
func (d *DurationOpt) Set(s string) error {
	v, err := time.ParseDuration(s)
	d.value = &v
	return err
}

// Type returns the type of this option, which will be displayed in `--help` output
func (d *DurationOpt) Type() string {
	return "duration"
}

// String returns a string repr of this option
func (d *DurationOpt) String() string {
	if d.value != nil {
		return d.value.String()
	}
	return ""
}

// Value returns the time.Duration
func (d *DurationOpt) Value() *time.Duration {
	return d.value
}

// Uint64Opt represents a uint64.
type Uint64Opt struct {
	value *uint64
}

// Set a new value on the option
func (i *Uint64Opt) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	i.value = &v
	return err
}

// Type returns the type of this option, which will be displayed in `--help` output
func (i *Uint64Opt) Type() string {
	return "uint"
}

// String returns a string repr of this option
func (i *Uint64Opt) String() string {
	if i.value != nil {
		return fmt.Sprintf("%v", *i.value)
	}
	return ""
}

// Value returns the uint64
func (i *Uint64Opt) Value() *uint64 {
	return i.value
}

type floatValue float32

func (f *floatValue) Set(s string) error {
	v, err := strconv.ParseFloat(s, 32)
	*f = floatValue(v)
	return err
}

func (f *floatValue) Type() string {
	return "float"
}

func (f *floatValue) String() string {
	return strconv.FormatFloat(float64(*f), 'g', -1, 32)
}

func (f *floatValue) Value() float32 {
	return float32(*f)
}

// placementPrefOpts holds a list of placement preferences.
type placementPrefOpts struct {
	prefs   []swarm.PlacementPreference
	strings []string
}

func (opts *placementPrefOpts) String() string {
	if len(opts.strings) == 0 {
		return ""
	}
	return fmt.Sprintf("%v", opts.strings)
}

// Set validates the input value and adds it to the internal slices.
// Note: in the future strategies other than "spread", may be supported,
// as well as additional comma-separated options.
func (opts *placementPrefOpts) Set(value string) error {
	fields := strings.Split(value, "=")
	if len(fields) != 2 {
		return errors.New(`placement preference must be of the format "<strategy>=<arg>"`)
	}
	if fields[0] != "spread" {
		return errors.Errorf("unsupported placement preference %s (only spread is supported)", fields[0])
	}

	opts.prefs = append(opts.prefs, swarm.PlacementPreference{
		Spread: &swarm.SpreadOver{
			SpreadDescriptor: fields[1],
		},
	})
	opts.strings = append(opts.strings, value)
	return nil
}

// Type returns a string name for this Option type
func (opts *placementPrefOpts) Type() string {
	return "pref"
}

// ShlexOpt is a flag Value which parses a string as a list of shell words
type ShlexOpt []string

// Set the value
func (s *ShlexOpt) Set(value string) error {
	valueSlice, err := shlex.Split(value)
	*s = ShlexOpt(valueSlice)
	return err
}

// Type returns the tyep of the value
func (s *ShlexOpt) Type() string {
	return "command"
}

func (s *ShlexOpt) String() string {
	if len(*s) == 0 {
		return ""
	}
	return fmt.Sprint(*s)
}

// Value returns the value as a string slice
func (s *ShlexOpt) Value() []string {
	return []string(*s)
}

type updateOptions struct {
	parallelism     uint64
	delay           time.Duration
	monitor         time.Duration
	onFailure       string
	maxFailureRatio floatValue
	order           string
}

func updateConfigFromDefaults(defaultUpdateConfig *api.UpdateConfig) *swarm.UpdateConfig {
	defaultFailureAction := strings.ToLower(api.UpdateConfig_FailureAction_name[int32(defaultUpdateConfig.FailureAction)])
	defaultMonitor, _ := gogotypes.DurationFromProto(defaultUpdateConfig.Monitor)
	return &swarm.UpdateConfig{
		Parallelism:     defaultUpdateConfig.Parallelism,
		Delay:           defaultUpdateConfig.Delay,
		Monitor:         defaultMonitor,
		FailureAction:   defaultFailureAction,
		MaxFailureRatio: defaultUpdateConfig.MaxFailureRatio,
		Order:           defaultOrder(defaultUpdateConfig.Order),
	}
}

func (opts updateOptions) updateConfig(flags *pflag.FlagSet) *swarm.UpdateConfig {
	if !anyChanged(flags, flagUpdateParallelism, flagUpdateDelay, flagUpdateMonitor, flagUpdateFailureAction, flagUpdateMaxFailureRatio) {
		return nil
	}

	updateConfig := updateConfigFromDefaults(defaults.Service.Update)

	if flags.Changed(flagUpdateParallelism) {
		updateConfig.Parallelism = opts.parallelism
	}
	if flags.Changed(flagUpdateDelay) {
		updateConfig.Delay = opts.delay
	}
	if flags.Changed(flagUpdateMonitor) {
		updateConfig.Monitor = opts.monitor
	}
	if flags.Changed(flagUpdateFailureAction) {
		updateConfig.FailureAction = opts.onFailure
	}
	if flags.Changed(flagUpdateMaxFailureRatio) {
		updateConfig.MaxFailureRatio = opts.maxFailureRatio.Value()
	}
	if flags.Changed(flagUpdateOrder) {
		updateConfig.Order = opts.order
	}

	return updateConfig
}

func (opts updateOptions) rollbackConfig(flags *pflag.FlagSet) *swarm.UpdateConfig {
	if !anyChanged(flags, flagRollbackParallelism, flagRollbackDelay, flagRollbackMonitor, flagRollbackFailureAction, flagRollbackMaxFailureRatio) {
		return nil
	}

	updateConfig := updateConfigFromDefaults(defaults.Service.Rollback)

	if flags.Changed(flagRollbackParallelism) {
		updateConfig.Parallelism = opts.parallelism
	}
	if flags.Changed(flagRollbackDelay) {
		updateConfig.Delay = opts.delay
	}
	if flags.Changed(flagRollbackMonitor) {
		updateConfig.Monitor = opts.monitor
	}
	if flags.Changed(flagRollbackFailureAction) {
		updateConfig.FailureAction = opts.onFailure
	}
	if flags.Changed(flagRollbackMaxFailureRatio) {
		updateConfig.MaxFailureRatio = opts.maxFailureRatio.Value()
	}
	if flags.Changed(flagRollbackOrder) {
		updateConfig.Order = opts.order
	}

	return updateConfig
}

type resourceOptions struct {
	limitCPU      opts.NanoCPUs
	limitMemBytes opts.MemBytes
	resCPU        opts.NanoCPUs
	resMemBytes   opts.MemBytes
}

func (r *resourceOptions) ToResourceRequirements() *swarm.ResourceRequirements {
	return &swarm.ResourceRequirements{
		Limits: &swarm.Resources{
			NanoCPUs:    r.limitCPU.Value(),
			MemoryBytes: r.limitMemBytes.Value(),
		},
		Reservations: &swarm.Resources{
			NanoCPUs:    r.resCPU.Value(),
			MemoryBytes: r.resMemBytes.Value(),
		},
	}
}

type restartPolicyOptions struct {
	condition   string
	delay       DurationOpt
	maxAttempts Uint64Opt
	window      DurationOpt
}

func defaultRestartPolicy() *swarm.RestartPolicy {
	defaultMaxAttempts := defaults.Service.Task.Restart.MaxAttempts
	rp := &swarm.RestartPolicy{
		MaxAttempts: &defaultMaxAttempts,
	}

	if defaults.Service.Task.Restart.Delay != nil {
		defaultRestartDelay, _ := gogotypes.DurationFromProto(defaults.Service.Task.Restart.Delay)
		rp.Delay = &defaultRestartDelay
	}
	if defaults.Service.Task.Restart.Window != nil {
		defaultRestartWindow, _ := gogotypes.DurationFromProto(defaults.Service.Task.Restart.Window)
		rp.Window = &defaultRestartWindow
	}
	rp.Condition = defaultRestartCondition()

	return rp
}

func defaultRestartCondition() swarm.RestartPolicyCondition {
	switch defaults.Service.Task.Restart.Condition {
	case api.RestartOnNone:
		return "none"
	case api.RestartOnFailure:
		return "on-failure"
	case api.RestartOnAny:
		return "any"
	default:
		return ""
	}
}

func defaultOrder(order api.UpdateConfig_UpdateOrder) string {
	switch order {
	case api.UpdateConfig_STOP_FIRST:
		return "stop-first"
	case api.UpdateConfig_START_FIRST:
		return "start-first"
	default:
		return ""
	}
}

func (r *restartPolicyOptions) ToRestartPolicy(flags *pflag.FlagSet) *swarm.RestartPolicy {
	if !anyChanged(flags, flagRestartDelay, flagRestartMaxAttempts, flagRestartWindow, flagRestartCondition) {
		return nil
	}

	restartPolicy := defaultRestartPolicy()

	if flags.Changed(flagRestartDelay) {
		restartPolicy.Delay = r.delay.Value()
	}
	if flags.Changed(flagRestartCondition) {
		restartPolicy.Condition = swarm.RestartPolicyCondition(r.condition)
	}
	if flags.Changed(flagRestartMaxAttempts) {
		restartPolicy.MaxAttempts = r.maxAttempts.Value()
	}
	if flags.Changed(flagRestartWindow) {
		restartPolicy.Window = r.window.Value()
	}

	return restartPolicy
}

type credentialSpecOpt struct {
	value  *swarm.CredentialSpec
	source string
}

func (c *credentialSpecOpt) Set(value string) error {
	c.source = value
	c.value = &swarm.CredentialSpec{}
	switch {
	case strings.HasPrefix(value, "file://"):
		c.value.File = strings.TrimPrefix(value, "file://")
	case strings.HasPrefix(value, "registry://"):
		c.value.Registry = strings.TrimPrefix(value, "registry://")
	default:
		return errors.New("Invalid credential spec - value must be prefixed file:// or registry:// followed by a value")
	}

	return nil
}

func (c *credentialSpecOpt) Type() string {
	return "credential-spec"
}

func (c *credentialSpecOpt) String() string {
	return c.source
}

func (c *credentialSpecOpt) Value() *swarm.CredentialSpec {
	return c.value
}

func convertNetworks(ctx context.Context, apiClient client.NetworkAPIClient, networks []string) ([]swarm.NetworkAttachmentConfig, error) {
	nets := []swarm.NetworkAttachmentConfig{}
	for _, networkIDOrName := range networks {
		network, err := apiClient.NetworkInspect(ctx, networkIDOrName, false)
		if err != nil {
			return nil, err
		}
		nets = append(nets, swarm.NetworkAttachmentConfig{Target: network.ID})
	}
	sort.Sort(byNetworkTarget(nets))
	return nets, nil
}

type endpointOptions struct {
	mode         string
	publishPorts opts.PortOpt
}

func (e *endpointOptions) ToEndpointSpec() *swarm.EndpointSpec {
	return &swarm.EndpointSpec{
		Mode:  swarm.ResolutionMode(strings.ToLower(e.mode)),
		Ports: e.publishPorts.Value(),
	}
}

type logDriverOptions struct {
	name string
	opts opts.ListOpts
}

func newLogDriverOptions() logDriverOptions {
	return logDriverOptions{opts: opts.NewListOpts(opts.ValidateEnv)}
}

func (ldo *logDriverOptions) toLogDriver() *swarm.Driver {
	if ldo.name == "" {
		return nil
	}

	// set the log driver only if specified.
	return &swarm.Driver{
		Name:    ldo.name,
		Options: runconfigopts.ConvertKVStringsToMap(ldo.opts.GetAll()),
	}
}

type healthCheckOptions struct {
	cmd           string
	interval      PositiveDurationOpt
	timeout       PositiveDurationOpt
	retries       int
	startPeriod   PositiveDurationOpt
	noHealthcheck bool
}

func (opts *healthCheckOptions) toHealthConfig() (*container.HealthConfig, error) {
	var healthConfig *container.HealthConfig
	haveHealthSettings := opts.cmd != "" ||
		opts.interval.Value() != nil ||
		opts.timeout.Value() != nil ||
		opts.retries != 0
	if opts.noHealthcheck {
		if haveHealthSettings {
			return nil, errors.Errorf("--%s conflicts with --health-* options", flagNoHealthcheck)
		}
		healthConfig = &container.HealthConfig{Test: []string{"NONE"}}
	} else if haveHealthSettings {
		var test []string
		if opts.cmd != "" {
			test = []string{"CMD-SHELL", opts.cmd}
		}
		var interval, timeout, startPeriod time.Duration
		if ptr := opts.interval.Value(); ptr != nil {
			interval = *ptr
		}
		if ptr := opts.timeout.Value(); ptr != nil {
			timeout = *ptr
		}
		if ptr := opts.startPeriod.Value(); ptr != nil {
			startPeriod = *ptr
		}
		healthConfig = &container.HealthConfig{
			Test:        test,
			Interval:    interval,
			Timeout:     timeout,
			Retries:     opts.retries,
			StartPeriod: startPeriod,
		}
	}
	return healthConfig, nil
}

// convertExtraHostsToSwarmHosts converts an array of extra hosts in cli
//     <host>:<ip>
// into a swarmkit host format:
//     IP_address canonical_hostname [aliases...]
// This assumes input value (<host>:<ip>) has already been validated
func convertExtraHostsToSwarmHosts(extraHosts []string) []string {
	hosts := []string{}
	for _, extraHost := range extraHosts {
		parts := strings.SplitN(extraHost, ":", 2)
		hosts = append(hosts, fmt.Sprintf("%s %s", parts[1], parts[0]))
	}
	return hosts
}

type serviceOptions struct {
	detach bool
	quiet  bool

	name            string
	labels          opts.ListOpts
	containerLabels opts.ListOpts
	image           string
	entrypoint      ShlexOpt
	args            []string
	hostname        string
	env             opts.ListOpts
	envFile         opts.ListOpts
	workdir         string
	user            string
	groups          opts.ListOpts
	credentialSpec  credentialSpecOpt
	stopSignal      string
	tty             bool
	readOnly        bool
	mounts          opts.MountOpt
	dns             opts.ListOpts
	dnsSearch       opts.ListOpts
	dnsOption       opts.ListOpts
	hosts           opts.ListOpts

	resources resourceOptions
	stopGrace DurationOpt

	replicas Uint64Opt
	mode     string

	restartPolicy  restartPolicyOptions
	constraints    opts.ListOpts
	placementPrefs placementPrefOpts
	update         updateOptions
	rollback       updateOptions
	networks       opts.ListOpts
	endpoint       endpointOptions

	registryAuth bool

	logDriver logDriverOptions

	healthcheck healthCheckOptions
	secrets     opts.SecretOpt
}

func newServiceOptions() *serviceOptions {
	return &serviceOptions{
		labels:          opts.NewListOpts(opts.ValidateEnv),
		constraints:     opts.NewListOpts(nil),
		containerLabels: opts.NewListOpts(opts.ValidateEnv),
		env:             opts.NewListOpts(opts.ValidateEnv),
		envFile:         opts.NewListOpts(nil),
		groups:          opts.NewListOpts(nil),
		logDriver:       newLogDriverOptions(),
		dns:             opts.NewListOpts(opts.ValidateIPAddress),
		dnsOption:       opts.NewListOpts(nil),
		dnsSearch:       opts.NewListOpts(opts.ValidateDNSSearch),
		hosts:           opts.NewListOpts(opts.ValidateExtraHost),
		networks:        opts.NewListOpts(nil),
	}
}

func (opts *serviceOptions) ToServiceMode() (swarm.ServiceMode, error) {
	serviceMode := swarm.ServiceMode{}
	switch opts.mode {
	case "global":
		if opts.replicas.Value() != nil {
			return serviceMode, errors.Errorf("replicas can only be used with replicated mode")
		}

		serviceMode.Global = &swarm.GlobalService{}
	case "replicated":
		serviceMode.Replicated = &swarm.ReplicatedService{
			Replicas: opts.replicas.Value(),
		}
	default:
		return serviceMode, errors.Errorf("Unknown mode: %s, only replicated and global supported", opts.mode)
	}
	return serviceMode, nil
}

func (opts *serviceOptions) ToStopGracePeriod(flags *pflag.FlagSet) *time.Duration {
	if flags.Changed(flagStopGracePeriod) {
		return opts.stopGrace.Value()
	}
	return nil
}

func (opts *serviceOptions) ToService(ctx context.Context, apiClient client.APIClient, flags *pflag.FlagSet) (swarm.ServiceSpec, error) {
	var service swarm.ServiceSpec

	envVariables, err := runconfigopts.ReadKVStrings(opts.envFile.GetAll(), opts.env.GetAll())
	if err != nil {
		return service, err
	}

	currentEnv := make([]string, 0, len(envVariables))
	for _, env := range envVariables { // need to process each var, in order
		k := strings.SplitN(env, "=", 2)[0]
		for i, current := range currentEnv { // remove duplicates
			if current == env {
				continue // no update required, may hide this behind flag to preserve order of envVariables
			}
			if strings.HasPrefix(current, k+"=") {
				currentEnv = append(currentEnv[:i], currentEnv[i+1:]...)
			}
		}
		currentEnv = append(currentEnv, env)
	}

	healthConfig, err := opts.healthcheck.toHealthConfig()
	if err != nil {
		return service, err
	}

	serviceMode, err := opts.ToServiceMode()
	if err != nil {
		return service, err
	}

	networks, err := convertNetworks(ctx, apiClient, opts.networks.GetAll())
	if err != nil {
		return service, err
	}

	service = swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   opts.name,
			Labels: runconfigopts.ConvertKVStringsToMap(opts.labels.GetAll()),
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:      opts.image,
				Args:       opts.args,
				Command:    opts.entrypoint.Value(),
				Env:        currentEnv,
				Hostname:   opts.hostname,
				Labels:     runconfigopts.ConvertKVStringsToMap(opts.containerLabels.GetAll()),
				Dir:        opts.workdir,
				User:       opts.user,
				Groups:     opts.groups.GetAll(),
				StopSignal: opts.stopSignal,
				TTY:        opts.tty,
				ReadOnly:   opts.readOnly,
				Mounts:     opts.mounts.Value(),
				DNSConfig: &swarm.DNSConfig{
					Nameservers: opts.dns.GetAll(),
					Search:      opts.dnsSearch.GetAll(),
					Options:     opts.dnsOption.GetAll(),
				},
				Hosts:           convertExtraHostsToSwarmHosts(opts.hosts.GetAll()),
				StopGracePeriod: opts.ToStopGracePeriod(flags),
				Secrets:         nil,
				Healthcheck:     healthConfig,
			},
			Networks:      networks,
			Resources:     opts.resources.ToResourceRequirements(),
			RestartPolicy: opts.restartPolicy.ToRestartPolicy(flags),
			Placement: &swarm.Placement{
				Constraints: opts.constraints.GetAll(),
				Preferences: opts.placementPrefs.prefs,
			},
			LogDriver: opts.logDriver.toLogDriver(),
		},
		Mode:           serviceMode,
		UpdateConfig:   opts.update.updateConfig(flags),
		RollbackConfig: opts.update.rollbackConfig(flags),
		EndpointSpec:   opts.endpoint.ToEndpointSpec(),
	}

	if opts.credentialSpec.Value() != nil {
		service.TaskTemplate.ContainerSpec.Privileges = &swarm.Privileges{
			CredentialSpec: opts.credentialSpec.Value(),
		}
	}

	return service, nil
}

type flagDefaults map[string]interface{}

func (fd flagDefaults) getUint64(flagName string) uint64 {
	if val, ok := fd[flagName].(uint64); ok {
		return val
	}
	return 0
}

func (fd flagDefaults) getString(flagName string) string {
	if val, ok := fd[flagName].(string); ok {
		return val
	}
	return ""
}

func buildServiceDefaultFlagMapping() flagDefaults {
	defaultFlagValues := make(map[string]interface{})

	defaultFlagValues[flagStopGracePeriod], _ = gogotypes.DurationFromProto(defaults.Service.Task.GetContainer().StopGracePeriod)
	defaultFlagValues[flagRestartCondition] = `"` + defaultRestartCondition() + `"`
	defaultFlagValues[flagRestartDelay], _ = gogotypes.DurationFromProto(defaults.Service.Task.Restart.Delay)

	if defaults.Service.Task.Restart.MaxAttempts != 0 {
		defaultFlagValues[flagRestartMaxAttempts] = defaults.Service.Task.Restart.MaxAttempts
	}

	defaultRestartWindow, _ := gogotypes.DurationFromProto(defaults.Service.Task.Restart.Window)
	if defaultRestartWindow != 0 {
		defaultFlagValues[flagRestartWindow] = defaultRestartWindow
	}

	defaultFlagValues[flagUpdateParallelism] = defaults.Service.Update.Parallelism
	defaultFlagValues[flagUpdateDelay] = defaults.Service.Update.Delay
	defaultFlagValues[flagUpdateMonitor], _ = gogotypes.DurationFromProto(defaults.Service.Update.Monitor)
	defaultFlagValues[flagUpdateFailureAction] = `"` + strings.ToLower(api.UpdateConfig_FailureAction_name[int32(defaults.Service.Update.FailureAction)]) + `"`
	defaultFlagValues[flagUpdateMaxFailureRatio] = defaults.Service.Update.MaxFailureRatio
	defaultFlagValues[flagUpdateOrder] = `"` + defaultOrder(defaults.Service.Update.Order) + `"`

	defaultFlagValues[flagRollbackParallelism] = defaults.Service.Rollback.Parallelism
	defaultFlagValues[flagRollbackDelay] = defaults.Service.Rollback.Delay
	defaultFlagValues[flagRollbackMonitor], _ = gogotypes.DurationFromProto(defaults.Service.Rollback.Monitor)
	defaultFlagValues[flagRollbackFailureAction] = `"` + strings.ToLower(api.UpdateConfig_FailureAction_name[int32(defaults.Service.Rollback.FailureAction)]) + `"`
	defaultFlagValues[flagRollbackMaxFailureRatio] = defaults.Service.Rollback.MaxFailureRatio
	defaultFlagValues[flagRollbackOrder] = `"` + defaultOrder(defaults.Service.Rollback.Order) + `"`

	defaultFlagValues[flagEndpointMode] = "vip"

	return defaultFlagValues
}

// addServiceFlags adds all flags that are common to both `create` and `update`.
// Any flags that are not common are added separately in the individual command
func addServiceFlags(flags *pflag.FlagSet, opts *serviceOptions, defaultFlagValues flagDefaults) {
	flagDesc := func(flagName string, desc string) string {
		if defaultValue, ok := defaultFlagValues[flagName]; ok {
			return fmt.Sprintf("%s (default %v)", desc, defaultValue)
		}
		return desc
	}

	flags.BoolVarP(&opts.detach, "detach", "d", true, "Exit immediately instead of waiting for the service to converge")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Suppress progress output")

	flags.StringVarP(&opts.workdir, flagWorkdir, "w", "", "Working directory inside the container")
	flags.StringVarP(&opts.user, flagUser, "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
	flags.Var(&opts.credentialSpec, flagCredentialSpec, "Credential spec for managed service account (Windows only)")
	flags.SetAnnotation(flagCredentialSpec, "version", []string{"1.29"})
	flags.StringVar(&opts.hostname, flagHostname, "", "Container hostname")
	flags.SetAnnotation(flagHostname, "version", []string{"1.25"})
	flags.Var(&opts.entrypoint, flagEntrypoint, "Overwrite the default ENTRYPOINT of the image")

	flags.Var(&opts.resources.limitCPU, flagLimitCPU, "Limit CPUs")
	flags.Var(&opts.resources.limitMemBytes, flagLimitMemory, "Limit Memory")
	flags.Var(&opts.resources.resCPU, flagReserveCPU, "Reserve CPUs")
	flags.Var(&opts.resources.resMemBytes, flagReserveMemory, "Reserve Memory")

	flags.Var(&opts.stopGrace, flagStopGracePeriod, flagDesc(flagStopGracePeriod, "Time to wait before force killing a container (ns|us|ms|s|m|h)"))
	flags.Var(&opts.replicas, flagReplicas, "Number of tasks")

	flags.StringVar(&opts.restartPolicy.condition, flagRestartCondition, "", flagDesc(flagRestartCondition, `Restart when condition is met ("none"|"on-failure"|"any")`))
	flags.Var(&opts.restartPolicy.delay, flagRestartDelay, flagDesc(flagRestartDelay, "Delay between restart attempts (ns|us|ms|s|m|h)"))
	flags.Var(&opts.restartPolicy.maxAttempts, flagRestartMaxAttempts, flagDesc(flagRestartMaxAttempts, "Maximum number of restarts before giving up"))

	flags.Var(&opts.restartPolicy.window, flagRestartWindow, flagDesc(flagRestartWindow, "Window used to evaluate the restart policy (ns|us|ms|s|m|h)"))

	flags.Uint64Var(&opts.update.parallelism, flagUpdateParallelism, defaultFlagValues.getUint64(flagUpdateParallelism), "Maximum number of tasks updated simultaneously (0 to update all at once)")
	flags.DurationVar(&opts.update.delay, flagUpdateDelay, 0, flagDesc(flagUpdateDelay, "Delay between updates (ns|us|ms|s|m|h)"))
	flags.DurationVar(&opts.update.monitor, flagUpdateMonitor, 0, flagDesc(flagUpdateMonitor, "Duration after each task update to monitor for failure (ns|us|ms|s|m|h)"))
	flags.SetAnnotation(flagUpdateMonitor, "version", []string{"1.25"})
	flags.StringVar(&opts.update.onFailure, flagUpdateFailureAction, "", flagDesc(flagUpdateFailureAction, `Action on update failure ("pause"|"continue"|"rollback")`))
	flags.Var(&opts.update.maxFailureRatio, flagUpdateMaxFailureRatio, flagDesc(flagUpdateMaxFailureRatio, "Failure rate to tolerate during an update"))
	flags.SetAnnotation(flagUpdateMaxFailureRatio, "version", []string{"1.25"})
	flags.StringVar(&opts.update.order, flagUpdateOrder, "", flagDesc(flagUpdateOrder, `Update order ("start-first"|"stop-first")`))
	flags.SetAnnotation(flagUpdateOrder, "version", []string{"1.29"})

	flags.Uint64Var(&opts.rollback.parallelism, flagRollbackParallelism, defaultFlagValues.getUint64(flagRollbackParallelism), "Maximum number of tasks rolled back simultaneously (0 to roll back all at once)")
	flags.SetAnnotation(flagRollbackParallelism, "version", []string{"1.28"})
	flags.DurationVar(&opts.rollback.delay, flagRollbackDelay, 0, flagDesc(flagRollbackDelay, "Delay between task rollbacks (ns|us|ms|s|m|h)"))
	flags.SetAnnotation(flagRollbackDelay, "version", []string{"1.28"})
	flags.DurationVar(&opts.rollback.monitor, flagRollbackMonitor, 0, flagDesc(flagRollbackMonitor, "Duration after each task rollback to monitor for failure (ns|us|ms|s|m|h)"))
	flags.SetAnnotation(flagRollbackMonitor, "version", []string{"1.28"})
	flags.StringVar(&opts.rollback.onFailure, flagRollbackFailureAction, "", flagDesc(flagRollbackFailureAction, `Action on rollback failure ("pause"|"continue")`))
	flags.SetAnnotation(flagRollbackFailureAction, "version", []string{"1.28"})
	flags.Var(&opts.rollback.maxFailureRatio, flagRollbackMaxFailureRatio, flagDesc(flagRollbackMaxFailureRatio, "Failure rate to tolerate during a rollback"))
	flags.SetAnnotation(flagRollbackMaxFailureRatio, "version", []string{"1.28"})
	flags.StringVar(&opts.rollback.order, flagRollbackOrder, "", flagDesc(flagRollbackOrder, `Rollback order ("start-first"|"stop-first")`))
	flags.SetAnnotation(flagRollbackOrder, "version", []string{"1.29"})

	flags.StringVar(&opts.endpoint.mode, flagEndpointMode, defaultFlagValues.getString(flagEndpointMode), "Endpoint mode (vip or dnsrr)")

	flags.BoolVar(&opts.registryAuth, flagRegistryAuth, false, "Send registry authentication details to swarm agents")

	flags.StringVar(&opts.logDriver.name, flagLogDriver, "", "Logging driver for service")
	flags.Var(&opts.logDriver.opts, flagLogOpt, "Logging driver options")

	flags.StringVar(&opts.healthcheck.cmd, flagHealthCmd, "", "Command to run to check health")
	flags.SetAnnotation(flagHealthCmd, "version", []string{"1.25"})
	flags.Var(&opts.healthcheck.interval, flagHealthInterval, "Time between running the check (ns|us|ms|s|m|h)")
	flags.SetAnnotation(flagHealthInterval, "version", []string{"1.25"})
	flags.Var(&opts.healthcheck.timeout, flagHealthTimeout, "Maximum time to allow one check to run (ns|us|ms|s|m|h)")
	flags.SetAnnotation(flagHealthTimeout, "version", []string{"1.25"})
	flags.IntVar(&opts.healthcheck.retries, flagHealthRetries, 0, "Consecutive failures needed to report unhealthy")
	flags.SetAnnotation(flagHealthRetries, "version", []string{"1.25"})
	flags.Var(&opts.healthcheck.startPeriod, flagHealthStartPeriod, "Start period for the container to initialize before counting retries towards unstable (ns|us|ms|s|m|h)")
	flags.SetAnnotation(flagHealthStartPeriod, "version", []string{"1.29"})
	flags.BoolVar(&opts.healthcheck.noHealthcheck, flagNoHealthcheck, false, "Disable any container-specified HEALTHCHECK")
	flags.SetAnnotation(flagNoHealthcheck, "version", []string{"1.25"})

	flags.BoolVarP(&opts.tty, flagTTY, "t", false, "Allocate a pseudo-TTY")
	flags.SetAnnotation(flagTTY, "version", []string{"1.25"})

	flags.BoolVar(&opts.readOnly, flagReadOnly, false, "Mount the container's root filesystem as read only")
	flags.SetAnnotation(flagReadOnly, "version", []string{"1.28"})

	flags.StringVar(&opts.stopSignal, flagStopSignal, "", "Signal to stop the container")
	flags.SetAnnotation(flagStopSignal, "version", []string{"1.28"})
}

const (
	flagCredentialSpec          = "credential-spec"
	flagPlacementPref           = "placement-pref"
	flagPlacementPrefAdd        = "placement-pref-add"
	flagPlacementPrefRemove     = "placement-pref-rm"
	flagConstraint              = "constraint"
	flagConstraintRemove        = "constraint-rm"
	flagConstraintAdd           = "constraint-add"
	flagContainerLabel          = "container-label"
	flagContainerLabelRemove    = "container-label-rm"
	flagContainerLabelAdd       = "container-label-add"
	flagDNS                     = "dns"
	flagDNSRemove               = "dns-rm"
	flagDNSAdd                  = "dns-add"
	flagDNSOption               = "dns-option"
	flagDNSOptionRemove         = "dns-option-rm"
	flagDNSOptionAdd            = "dns-option-add"
	flagDNSSearch               = "dns-search"
	flagDNSSearchRemove         = "dns-search-rm"
	flagDNSSearchAdd            = "dns-search-add"
	flagEndpointMode            = "endpoint-mode"
	flagEntrypoint              = "entrypoint"
	flagHost                    = "host"
	flagHostAdd                 = "host-add"
	flagHostRemove              = "host-rm"
	flagHostname                = "hostname"
	flagEnv                     = "env"
	flagEnvFile                 = "env-file"
	flagEnvRemove               = "env-rm"
	flagEnvAdd                  = "env-add"
	flagGroup                   = "group"
	flagGroupAdd                = "group-add"
	flagGroupRemove             = "group-rm"
	flagLabel                   = "label"
	flagLabelRemove             = "label-rm"
	flagLabelAdd                = "label-add"
	flagLimitCPU                = "limit-cpu"
	flagLimitMemory             = "limit-memory"
	flagMode                    = "mode"
	flagMount                   = "mount"
	flagMountRemove             = "mount-rm"
	flagMountAdd                = "mount-add"
	flagName                    = "name"
	flagNetwork                 = "network"
	flagNetworkAdd              = "network-add"
	flagNetworkRemove           = "network-rm"
	flagPublish                 = "publish"
	flagPublishRemove           = "publish-rm"
	flagPublishAdd              = "publish-add"
	flagReadOnly                = "read-only"
	flagReplicas                = "replicas"
	flagReserveCPU              = "reserve-cpu"
	flagReserveMemory           = "reserve-memory"
	flagRestartCondition        = "restart-condition"
	flagRestartDelay            = "restart-delay"
	flagRestartMaxAttempts      = "restart-max-attempts"
	flagRestartWindow           = "restart-window"
	flagRollbackDelay           = "rollback-delay"
	flagRollbackFailureAction   = "rollback-failure-action"
	flagRollbackMaxFailureRatio = "rollback-max-failure-ratio"
	flagRollbackMonitor         = "rollback-monitor"
	flagRollbackOrder           = "rollback-order"
	flagRollbackParallelism     = "rollback-parallelism"
	flagStopGracePeriod         = "stop-grace-period"
	flagStopSignal              = "stop-signal"
	flagTTY                     = "tty"
	flagUpdateDelay             = "update-delay"
	flagUpdateFailureAction     = "update-failure-action"
	flagUpdateMaxFailureRatio   = "update-max-failure-ratio"
	flagUpdateMonitor           = "update-monitor"
	flagUpdateOrder             = "update-order"
	flagUpdateParallelism       = "update-parallelism"
	flagUser                    = "user"
	flagWorkdir                 = "workdir"
	flagRegistryAuth            = "with-registry-auth"
	flagLogDriver               = "log-driver"
	flagLogOpt                  = "log-opt"
	flagHealthCmd               = "health-cmd"
	flagHealthInterval          = "health-interval"
	flagHealthRetries           = "health-retries"
	flagHealthTimeout           = "health-timeout"
	flagHealthStartPeriod       = "health-start-period"
	flagNoHealthcheck           = "no-healthcheck"
	flagSecret                  = "secret"
	flagSecretAdd               = "secret-add"
	flagSecretRemove            = "secret-rm"
)
