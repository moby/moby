// Package nri provides com.docker.mobyextension.nri.v1, an extension that
// bridges the create-spec hook point to the NRI (Node Resource Interface)
// framework.
//
// It is a single extension to the broker. Internally it runs the containerd/nri
// adaptation, which discovers the NRI plugins, orders them by their 2-digit
// index, and merges their adjustments with per-field ownership/conflict
// detection. So NRI's own ordering and conflict handling stay inside this
// extension -- the broker only ever sees one provider.
package nri

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
	"github.com/moby/moby/v2/daemon/pkg/opts"
	"github.com/moby/moby/v2/dockerversion"
	createspecv0 "github.com/moby/moby/v2/extpoints/createspec/v0"
	"github.com/moby/moby/v2/internal/extensions"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// ExtensionID is the id of the NRI bridge extension.
const ExtensionID = "com.docker.mobyextension.nri.v1"

// Extension is the NRI bridge as an in-process extension: a [Bridge] is itself
// the extension. Its adaptation starts in Init -- configured from the config the
// host delivers -- and stops in Shutdown, so its lifecycle follows the broker's.
// The same Bridge is served out of process by ./cmd/nri, so NRI can run either
// way without changing its logic.
var Extension extensions.Extension = &Bridge{}

func (b *Bridge) Declaration() extensions.Declaration {
	return extensions.Declaration{
		ID:        ExtensionID,
		Providers: []extensions.Provider{createspecv0.Point.Provide(b)},
		Init:      b.init,
		Shutdown:  b.Stop,
	}
}

func (b *Bridge) init(ctx context.Context, cfg extensions.Config, _ extensions.Resolver) error {
	if len(cfg) > 0 {
		// Decode the config object into NRIOpts via its JSON tags.
		raw, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("nri: encode config: %w", err)
		}
		if err := json.Unmarshal(raw, &b.cfg); err != nil {
			return fmt.Errorf("nri: parse config: %w", err)
		}
	}
	return b.Start(ctx)
}

// Bridge implements the container-create hook by delegating to the NRI
// adaptation. It is the shared core behind both the in-process [Extension] and
// the out-of-process binary.
type Bridge struct {
	cfg  opts.NRIOpts
	adap *adaptation.Adaptation
}

// NewBridge returns a Bridge configured by cfg. Empty plugin paths default to
// the standard locations in Start.
func NewBridge(cfg opts.NRIOpts) *Bridge {
	return &Bridge{cfg: cfg}
}

// Start creates and starts the NRI adaptation, which discovers and launches the
// NRI plugins.
func (b *Bridge) Start(context.Context) error {
	cfg := b.cfg
	applyDefaultPaths(&cfg)
	adap, err := adaptation.New("docker", dockerversion.Version, b.syncFn, b.updateFn, nriOptions(cfg)...)
	if err != nil {
		return err
	}
	if err := adap.Start(); err != nil {
		return err
	}
	b.adap = adap
	return nil
}

// Stop stops the NRI adaptation.
func (b *Bridge) Stop(context.Context) error {
	if b.adap != nil {
		b.adap.Stop()
		b.adap = nil
	}
	return nil
}

// CreateSpec asks the NRI plugins to adjust the container's OCI runtime spec and
// returns the adjusted spec. It runs at container start on the fully-formed
// spec, so the plugins see the real container id and the actual runtime config
// (namespaces, devices, resources, mounts) -- the fidelity NRI is designed for,
// and how containerd's NRI integration works.
//
// TODO: only env and mount adjustments are applied back today; richer
// adjustments (resources, devices, rlimits, hooks, CDI) are rejected until they
// are mapped onto the spec. Container lifecycle events and state sync are the
// remaining open items.
func (b *Bridge) CreateSpec(ctx context.Context, req *createspecv0.SpecRequest) (*createspecv0.SpecAdjustment, error) {
	if b.adap == nil {
		return nil, nil
	}
	var spec specs.Spec
	if err := json.Unmarshal(req.Spec, &spec); err != nil {
		return nil, fmt.Errorf("nri: parse OCI spec: %w", err)
	}
	resp, err := b.adap.CreateContainer(ctx, &adaptation.CreateContainerRequest{
		Pod:       &adaptation.PodSandbox{Id: req.ContainerID, Name: req.Name, Labels: req.Labels},
		Container: containerFromSpec(req, &spec),
	})
	if err != nil {
		return nil, err
	}
	if resp.GetUpdate() != nil {
		return nil, errors.New("nri: adjusting other containers is not supported")
	}
	if resp.GetEvict() != nil {
		return nil, errors.New("nri: container eviction is not supported")
	}
	if err := applyAdjustment(&spec, resp.GetAdjust()); err != nil {
		return nil, err
	}
	out, err := json.Marshal(&spec)
	if err != nil {
		return nil, fmt.Errorf("nri: encode OCI spec: %w", err)
	}
	return &createspecv0.SpecAdjustment{Spec: out}, nil
}

// Validate is a no-op: NRI validates its plugins' adjustments inside
// CreateContainer.
func (b *Bridge) Validate(context.Context, *createspecv0.SpecRequest) error {
	return nil
}

func (b *Bridge) syncFn(ctx context.Context, syncCB adaptation.SyncCB) error {
	// TODO: deliver the current set of containers so a connecting plugin can
	// learn existing state. Without the container lifecycle/lister, sync is empty.
	updates, err := syncCB(ctx, nil, nil)
	if err != nil {
		return err
	}
	if len(updates) > 0 {
		return errors.New("nri: container updates during sync are not implemented")
	}
	return nil
}

func (b *Bridge) updateFn(context.Context, []*adaptation.ContainerUpdate) ([]*adaptation.ContainerUpdate, error) {
	return nil, errors.New("nri: asynchronous container updates are not implemented")
}

// containerFromSpec builds the NRI container view from the OCI spec, so the
// plugins see the real container: its id, args, env, mounts, and Linux config.
func containerFromSpec(req *createspecv0.SpecRequest, spec *specs.Spec) *adaptation.Container {
	ctr := &adaptation.Container{
		Id:           req.ContainerID,
		PodSandboxId: req.ContainerID,
		Name:         req.Name,
		State:        adaptation.ContainerState_CONTAINER_CREATED,
		Labels:       req.Labels,
		Annotations:  spec.Annotations,
		Mounts:       api.FromOCIMounts(spec.Mounts),
	}
	if spec.Process != nil {
		ctr.Args = spec.Process.Args
		ctr.Env = spec.Process.Env
	}
	if spec.Linux != nil {
		ctr.Linux = &adaptation.LinuxContainer{
			Namespaces: api.FromOCILinuxNamespaces(spec.Linux.Namespaces),
			Devices:    api.FromOCILinuxDevices(spec.Linux.Devices),
			Resources:  api.FromOCILinuxResources(spec.Linux.Resources, nil),
		}
	}
	return ctr
}

// applyAdjustment applies an NRI ContainerAdjustment onto the OCI spec. It maps
// the adjustments with an unambiguous OCI representation -- env, mounts, args,
// annotations, rlimits, device nodes, and CPU/memory resources -- and rejects
// the rest (see [rejectUnsupported] and [applyResources]) so a plugin's request
// is never silently dropped.
func applyAdjustment(spec *specs.Spec, adj *adaptation.ContainerAdjustment) error {
	if adj == nil {
		return nil
	}
	if err := rejectUnsupported(adj); err != nil {
		return err
	}
	if err := applyEnv(spec, adj.Env); err != nil {
		return err
	}
	for _, m := range adj.Mounts {
		spec.Mounts = append(spec.Mounts, m.ToOCI(nil))
	}
	if len(adj.Args) > 0 {
		ensureProcess(spec).Args = adj.Args
	}
	if len(adj.Annotations) > 0 {
		if spec.Annotations == nil {
			spec.Annotations = make(map[string]string, len(adj.Annotations))
		}
		for k, v := range adj.Annotations {
			spec.Annotations[k] = v
		}
	}
	applyRlimits(spec, adj.Rlimits)
	if adj.Linux != nil {
		applyLinuxDevices(spec, adj.Linux.Devices)
		if err := applyResources(spec, adj.Linux.Resources); err != nil {
			return err
		}
	}
	return nil
}

// applyEnv merges NRI env adjustments into spec.Process.Env, replacing a
// variable that already exists by key or appending a new one.
func applyEnv(spec *specs.Spec, kvs []*api.KeyValue) error {
	if len(kvs) == 0 {
		return nil
	}
	if spec.Process == nil {
		spec.Process = &specs.Process{}
	}
	idx := make(map[string]int, len(spec.Process.Env))
	for i, e := range spec.Process.Env {
		k, _, _ := strings.Cut(e, "=")
		idx[k] = i
	}
	for _, kv := range kvs {
		if kv.Key == "" {
			return errors.New("nri: empty environment variable key")
		}
		val := kv.ToOCI()
		if i, ok := idx[kv.Key]; ok {
			spec.Process.Env[i] = val
		} else {
			idx[kv.Key] = len(spec.Process.Env)
			spec.Process.Env = append(spec.Process.Env, val)
		}
	}
	return nil
}

// rejectUnsupported errors on any adjustment this bridge does not map onto the
// OCI spec, so a plugin's request is refused rather than silently dropped. It
// covers every ContainerAdjustment and LinuxContainerAdjustment field without a
// mapping -- checked field by field so a new NRI field cannot slip through
// unnoticed -- and any entry marked for removal (see [rejectRemovals]).
//
// TODO: implement removal (env/mounts/devices/annotations) and map the Linux
// adjustments rejected below (seccomp, namespaces, sysctls, ...) rather than
// refusing them.
func rejectUnsupported(adj *adaptation.ContainerAdjustment) error {
	if err := rejectRemovals(adj); err != nil {
		return err
	}
	var u []string
	add := func(cond bool, name string) {
		if cond {
			u = append(u, name)
		}
	}
	// ContainerAdjustment: Annotations, Mounts, Env, Rlimits, and Args are
	// applied by applyAdjustment; Hooks and CDIDevices are not.
	add(adj.Hooks != nil, "hooks")
	add(len(adj.CDIDevices) > 0, "CDI")
	if l := adj.Linux; l != nil {
		// LinuxContainerAdjustment: Devices and Resources are applied; the rest
		// have no mapping yet.
		add(l.CgroupsPath != "", "linux.cgroupsPath")
		add(l.OomScoreAdj != nil, "linux.oomScoreAdj")
		add(l.IoPriority != nil, "linux.ioPriority")
		add(l.SeccompPolicy != nil, "linux.seccompPolicy")
		add(len(l.Namespaces) > 0, "linux.namespaces")
		add(len(l.Sysctl) > 0, "linux.sysctl")
		add(len(l.NetDevices) > 0, "linux.netDevices")
		add(l.Scheduler != nil, "linux.scheduler")
		add(l.Rdt != nil, "linux.rdt")
		add(l.MemoryPolicy != nil, "linux.memoryPolicy")
	}
	if len(u) > 0 {
		return fmt.Errorf("nri: unsupported container adjustments: %s", strings.Join(u, ","))
	}
	return nil
}

// rejectRemovals refuses adjustments marked for removal. NRI signals a deletion
// by prefixing the key/destination/path with '-'; this bridge only adds and
// replaces, so applying a marked entry verbatim would corrupt the spec (append a
// literal "-KEY" env var, a mount at "-/dest", and so on) rather than delete
// anything -- exactly the silent, wrong result the reject-don't-drop rule exists
// to prevent.
func rejectRemovals(adj *adaptation.ContainerAdjustment) error {
	var removed []string
	for _, e := range adj.Env {
		if key, ok := e.IsMarkedForRemoval(); ok {
			removed = append(removed, "env "+key)
		}
	}
	for _, m := range adj.Mounts {
		if dst, ok := m.IsMarkedForRemoval(); ok {
			removed = append(removed, "mount "+dst)
		}
	}
	for k := range adj.Annotations {
		if key, ok := api.IsMarkedForRemoval(k); ok {
			removed = append(removed, "annotation "+key)
		}
	}
	if adj.Linux != nil {
		for _, d := range adj.Linux.Devices {
			if path, ok := d.IsMarkedForRemoval(); ok {
				removed = append(removed, "device "+path)
			}
		}
	}
	if len(removed) > 0 {
		return fmt.Errorf("nri: removal adjustments are not supported: %s", strings.Join(removed, ","))
	}
	return nil
}

func ensureProcess(spec *specs.Spec) *specs.Process {
	if spec.Process == nil {
		spec.Process = &specs.Process{}
	}
	return spec.Process
}

func ensureLinux(spec *specs.Spec) *specs.Linux {
	if spec.Linux == nil {
		spec.Linux = &specs.Linux{}
	}
	return spec.Linux
}

// applyRlimits sets each rlimit by type, replacing an existing limit of the same
// type or appending a new one.
func applyRlimits(spec *specs.Spec, rls []*api.POSIXRlimit) {
	if len(rls) == 0 {
		return
	}
	p := ensureProcess(spec)
	idx := make(map[string]int, len(p.Rlimits))
	for i, r := range p.Rlimits {
		idx[r.Type] = i
	}
	for _, r := range rls {
		lim := specs.POSIXRlimit{Type: r.Type, Hard: r.Hard, Soft: r.Soft}
		if i, ok := idx[r.Type]; ok {
			p.Rlimits[i] = lim
		} else {
			idx[r.Type] = len(p.Rlimits)
			p.Rlimits = append(p.Rlimits, lim)
		}
	}
}

// applyLinuxDevices adds device nodes, replacing an existing device at the same
// path or appending a new one.
func applyLinuxDevices(spec *specs.Spec, devs []*api.LinuxDevice) {
	if len(devs) == 0 {
		return
	}
	l := ensureLinux(spec)
	idx := make(map[string]int, len(l.Devices))
	for i, d := range l.Devices {
		idx[d.Path] = i
	}
	for _, d := range devs {
		dev := d.ToOCI()
		if i, ok := idx[dev.Path]; ok {
			l.Devices[i] = dev
		} else {
			idx[dev.Path] = len(l.Devices)
			l.Devices = append(l.Devices, dev)
		}
	}
}

// applyResources overlays the plugin's CPU and memory limits onto the spec's
// resources. NRI resource adjustments are sparse, so only the fields a plugin
// actually set are applied. Resource kinds without a mapping (device cgroup
// rules, pids, hugepages, unified, and the blockio/RDT classes) are rejected
// rather than dropped.
func applyResources(spec *specs.Spec, res *adaptation.LinuxResources) error {
	if res == nil {
		return nil
	}
	var u []string
	// BlockioClass and RdtClass have no OCI representation and are dropped by
	// ToOCI(), so they must be checked on the NRI resources before the conversion.
	if res.BlockioClass != nil {
		u = append(u, "blockioClass")
	}
	if res.RdtClass != nil {
		u = append(u, "rdtClass")
	}
	oci := res.ToOCI()
	if len(oci.Devices) > 0 {
		u = append(u, "device cgroup rules")
	}
	if oci.Pids != nil {
		u = append(u, "pids")
	}
	if len(oci.HugepageLimits) > 0 {
		u = append(u, "hugepages")
	}
	if len(oci.Unified) > 0 {
		u = append(u, "unified")
	}
	if len(u) > 0 {
		return fmt.Errorf("nri: unsupported resource adjustments: %s", strings.Join(u, ","))
	}
	r := ensureLinux(spec)
	if r.Resources == nil {
		r.Resources = &specs.LinuxResources{}
	}
	mergeCPU(r.Resources, oci.CPU)
	mergeMemory(r.Resources, oci.Memory)
	return nil
}

// mergeCPU overlays the set CPU fields of src onto dst, creating dst.CPU only
// if a field is actually set.
func mergeCPU(dst *specs.LinuxResources, src *specs.LinuxCPU) {
	if src == nil {
		return
	}
	cpu := func() *specs.LinuxCPU {
		if dst.CPU == nil {
			dst.CPU = &specs.LinuxCPU{}
		}
		return dst.CPU
	}
	if src.Shares != nil {
		cpu().Shares = src.Shares
	}
	if src.Quota != nil {
		cpu().Quota = src.Quota
	}
	if src.Period != nil {
		cpu().Period = src.Period
	}
	if src.RealtimeRuntime != nil {
		cpu().RealtimeRuntime = src.RealtimeRuntime
	}
	if src.RealtimePeriod != nil {
		cpu().RealtimePeriod = src.RealtimePeriod
	}
	if src.Cpus != "" {
		cpu().Cpus = src.Cpus
	}
	if src.Mems != "" {
		cpu().Mems = src.Mems
	}
}

// mergeMemory overlays the set memory fields of src onto dst, creating
// dst.Memory only if a field is actually set.
func mergeMemory(dst *specs.LinuxResources, src *specs.LinuxMemory) {
	if src == nil {
		return
	}
	mem := func() *specs.LinuxMemory {
		if dst.Memory == nil {
			dst.Memory = &specs.LinuxMemory{}
		}
		return dst.Memory
	}
	if src.Limit != nil {
		mem().Limit = src.Limit
	}
	if src.Reservation != nil {
		mem().Reservation = src.Reservation
	}
	if src.Swap != nil {
		mem().Swap = src.Swap
	}
	if src.Kernel != nil { //nolint:staticcheck // NRI still carries OCI kernel memory for runtimes that honor it.
		mem().Kernel = src.Kernel //nolint:staticcheck // NRI still carries OCI kernel memory for runtimes that honor it.
	}
	if src.KernelTCP != nil {
		mem().KernelTCP = src.KernelTCP
	}
	if src.Swappiness != nil {
		mem().Swappiness = src.Swappiness
	}
	if src.DisableOOMKiller != nil {
		mem().DisableOOMKiller = src.DisableOOMKiller
	}
	if src.UseHierarchy != nil {
		mem().UseHierarchy = src.UseHierarchy
	}
}

// applyDefaultPaths fills in the standard NRI plugin and config locations when
// unset, so the out-of-process binary can self-configure.
//
// TODO: rootless Docker uses different (homedir-relative) locations; see the
// in-tree NRI integration's setDefaultPaths.
func applyDefaultPaths(cfg *opts.NRIOpts) {
	if cfg.PluginPath == "" {
		cfg.PluginPath = "/usr/libexec/docker/nri-plugins"
	}
	if cfg.PluginConfigPath == "" {
		cfg.PluginConfigPath = "/etc/docker/nri/conf.d"
	}
}

func nriOptions(cfg opts.NRIOpts) []adaptation.Option {
	res := []adaptation.Option{
		adaptation.WithPluginPath(cfg.PluginPath),
		adaptation.WithPluginConfigPath(cfg.PluginConfigPath),
	}
	if cfg.SocketPath == "" {
		res = append(res, adaptation.WithDisabledExternalConnections())
	} else {
		res = append(res, adaptation.WithSocketPath(cfg.SocketPath))
	}
	return res
}
