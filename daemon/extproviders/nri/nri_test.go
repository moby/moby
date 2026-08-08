package nri

import (
	"testing"

	"github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestApplyAdjustmentEnvAndMounts(t *testing.T) {
	spec := &specs.Spec{
		Process: &specs.Process{Env: []string{"PATH=/bin", "FOO=old"}},
		Mounts:  []specs.Mount{{Destination: "/data"}},
	}
	adj := &adaptation.ContainerAdjustment{
		Env: []*api.KeyValue{
			{Key: "FOO", Value: "new"}, // replaces the existing FOO by key
			{Key: "BAR", Value: "baz"}, // appended
		},
		Mounts: []*api.Mount{
			{Destination: "/extra", Source: "/host/extra", Type: "bind"},
		},
	}

	assert.NilError(t, applyAdjustment(spec, adj))
	assert.DeepEqual(t, spec.Process.Env, []string{"PATH=/bin", "FOO=new", "BAR=baz"})
	assert.Check(t, is.Len(spec.Mounts, 2))
	assert.Equal(t, spec.Mounts[1].Destination, "/extra")
	assert.Equal(t, spec.Mounts[1].Source, "/host/extra")
}

func TestApplyAdjustmentArgsAnnotationsRlimits(t *testing.T) {
	spec := &specs.Spec{
		Process:     &specs.Process{Args: []string{"sh"}},
		Annotations: map[string]string{"keep": "yes"},
	}
	adj := &adaptation.ContainerAdjustment{
		Args:        []string{"sh", "-c", "true"},
		Annotations: map[string]string{"added": "1"},
		Rlimits:     []*api.POSIXRlimit{{Type: "RLIMIT_NOFILE", Hard: 1024, Soft: 512}},
	}

	assert.NilError(t, applyAdjustment(spec, adj))
	assert.DeepEqual(t, spec.Process.Args, []string{"sh", "-c", "true"})
	assert.Equal(t, spec.Annotations["keep"], "yes")
	assert.Equal(t, spec.Annotations["added"], "1")
	assert.Check(t, is.Len(spec.Process.Rlimits, 1))
	assert.Equal(t, spec.Process.Rlimits[0].Type, "RLIMIT_NOFILE")
	assert.Equal(t, spec.Process.Rlimits[0].Hard, uint64(1024))
}

func TestApplyAdjustmentResourcesOverlay(t *testing.T) {
	existingShares := uint64(100)
	spec := &specs.Spec{
		Linux: &specs.Linux{Resources: &specs.LinuxResources{
			CPU: &specs.LinuxCPU{Shares: &existingShares},
		}},
	}
	adj := &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{
		Resources: &adaptation.LinuxResources{
			Cpu:    &adaptation.LinuxCPU{Quota: api.Int64(50000)},
			Memory: &adaptation.LinuxMemory{Limit: api.Int64(1 << 30)},
		},
	}}

	assert.NilError(t, applyAdjustment(spec, adj))
	cpu := spec.Linux.Resources.CPU
	// the plugin set only Quota; Shares from the spec is preserved (sparse overlay)
	assert.Equal(t, *cpu.Shares, uint64(100))
	assert.Equal(t, *cpu.Quota, int64(50000))
	assert.Equal(t, *spec.Linux.Resources.Memory.Limit, int64(1<<30))
}

func TestApplyAdjustmentRejectsUnsupported(t *testing.T) {
	spec := &specs.Spec{Process: &specs.Process{}}
	// OCI hooks have no mapping yet, so they are rejected, not silently dropped.
	err := applyAdjustment(spec, &adaptation.ContainerAdjustment{
		Hooks: &api.Hooks{CreateRuntime: []*api.Hook{{Path: "/bin/true"}}},
	})
	assert.ErrorContains(t, err, "unsupported")
}

func TestApplyResourcesRejectsUnmappedKind(t *testing.T) {
	spec := &specs.Spec{}
	// pids limits have no overlay yet, so a pids adjustment is rejected.
	err := applyAdjustment(spec, &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{
		Resources: &adaptation.LinuxResources{Pids: &api.LinuxPids{Limit: 128}},
	}})
	assert.ErrorContains(t, err, "unsupported resource adjustments")
}

// TestRejectUnsupportedFields locks down that rejectUnsupported is exhaustive
// over every ContainerAdjustment/LinuxContainerAdjustment field without an OCI
// mapping: each such field must be rejected (naming the field) rather than
// silently dropped, while a plain env/mount/args adjustment is accepted.
func TestRejectUnsupportedFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		adj   *adaptation.ContainerAdjustment
		field string
	}{
		{
			name:  "hooks",
			adj:   &adaptation.ContainerAdjustment{Hooks: &api.Hooks{CreateRuntime: []*api.Hook{{Path: "/bin/true"}}}},
			field: "hooks",
		},
		{
			name:  "CDIDevices",
			adj:   &adaptation.ContainerAdjustment{CDIDevices: []*api.CDIDevice{{Name: "vendor.com/dev=0"}}},
			field: "CDI",
		},
		{
			name:  "cgroupsPath",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{CgroupsPath: "/foo"}},
			field: "cgroupsPath",
		},
		{
			name:  "oomScoreAdj",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{OomScoreAdj: api.Int(-500)}},
			field: "oomScoreAdj",
		},
		{
			name:  "ioPriority",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{IoPriority: &api.LinuxIOPriority{}}},
			field: "ioPriority",
		},
		{
			name:  "seccompPolicy",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{SeccompPolicy: &api.LinuxSeccomp{}}},
			field: "seccompPolicy",
		},
		{
			name:  "namespaces",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{Namespaces: []*api.LinuxNamespace{{Type: "network"}}}},
			field: "namespaces",
		},
		{
			name:  "sysctl",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{Sysctl: map[string]string{"net.ipv4.ip_forward": "1"}}},
			field: "sysctl",
		},
		{
			name:  "netDevices",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{NetDevices: map[string]*api.LinuxNetDevice{"eth0": {}}}},
			field: "netDevices",
		},
		{
			name:  "scheduler",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{Scheduler: &api.LinuxScheduler{}}},
			field: "scheduler",
		},
		{
			name:  "rdt",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{Rdt: &api.LinuxRdt{}}},
			field: "rdt",
		},
		{
			name:  "memoryPolicy",
			adj:   &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{MemoryPolicy: &api.LinuxMemoryPolicy{}}},
			field: "memoryPolicy",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := rejectUnsupported(tc.adj)
			assert.ErrorContains(t, err, "unsupported")
			assert.ErrorContains(t, err, tc.field)
		})
	}

	t.Run("plain env/mount/args accepted", func(t *testing.T) {
		adj := &adaptation.ContainerAdjustment{
			Env:    []*api.KeyValue{{Key: "FOO", Value: "bar"}},
			Mounts: []*api.Mount{{Destination: "/data", Source: "/host/data", Type: "bind"}},
			Args:   []string{"sh", "-c", "true"},
		}
		assert.NilError(t, rejectUnsupported(adj))
	})
}

// TestRejectRemovals locks down that an adjustment with a removal-marked entry
// (key/dest/path prefixed with '-') is rejected -- since the bridge only adds
// and replaces, applying it verbatim would corrupt the spec -- while a normal
// (non-removal) entry is accepted.
func TestRejectRemovals(t *testing.T) {
	for _, tc := range []struct {
		name string
		adj  *adaptation.ContainerAdjustment
	}{
		{
			name: "env",
			adj:  &adaptation.ContainerAdjustment{Env: []*api.KeyValue{{Key: api.MarkForRemoval("FOO")}}},
		},
		{
			name: "mount",
			adj:  &adaptation.ContainerAdjustment{Mounts: []*api.Mount{{Destination: api.MarkForRemoval("/data")}}},
		},
		{
			name: "device",
			adj:  &adaptation.ContainerAdjustment{Linux: &adaptation.LinuxContainerAdjustment{Devices: []*api.LinuxDevice{{Path: api.MarkForRemoval("/dev/null")}}}},
		},
		{
			name: "annotation",
			adj:  &adaptation.ContainerAdjustment{Annotations: map[string]string{api.MarkForRemoval("k"): ""}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := rejectRemovals(tc.adj)
			assert.ErrorContains(t, err, "removal")
			assert.ErrorContains(t, err, tc.name)
		})
	}

	t.Run("non-removal accepted", func(t *testing.T) {
		adj := &adaptation.ContainerAdjustment{
			Env:         []*api.KeyValue{{Key: "FOO", Value: "bar"}},
			Mounts:      []*api.Mount{{Destination: "/data"}},
			Annotations: map[string]string{"k": "v"},
			Linux:       &adaptation.LinuxContainerAdjustment{Devices: []*api.LinuxDevice{{Path: "/dev/null"}}},
		}
		assert.NilError(t, rejectRemovals(adj))
	})
}

// TestApplyResourcesRejectsClasses locks down that BlockioClass and RdtClass are
// rejected: ToOCI() drops them, so they must be checked on the NRI resources
// before conversion or the plugin's request would be silently lost.
func TestApplyResourcesRejectsClasses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		res   *adaptation.LinuxResources
		field string
	}{
		{
			name:  "blockioClass",
			res:   &adaptation.LinuxResources{BlockioClass: api.String("slow")},
			field: "blockioClass",
		},
		{
			name:  "rdtClass",
			res:   &adaptation.LinuxResources{RdtClass: api.String("group-a")},
			field: "rdtClass",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := applyResources(&specs.Spec{}, tc.res)
			assert.ErrorContains(t, err, "unsupported resource adjustments")
			assert.ErrorContains(t, err, tc.field)
		})
	}
}
