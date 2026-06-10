package linuxresources

import (
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/cpuset"
)

type Metadata struct {
	LinuxResources *pb.LinuxResources
}

func (m *Metadata) Merge(other solver.VertexMetadata) solver.VertexMetadata {
	if other == nil {
		return m
	}
	o, ok := other.(*Metadata)
	if !ok {
		return m
	}
	return &Metadata{
		LinuxResources: mergeRelaxed(m.LinuxResources, o.LinuxResources),
	}
}

// mergeRelaxed returns the most relaxed of a and b, so a shared
// vertex is never throttled by a stricter sibling job.
func mergeRelaxed(a, b *pb.LinuxResources) *pb.LinuxResources {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		return b.CloneVT()
	}
	if b == nil {
		return a.CloneVT()
	}
	period, quota := relaxedCPUBandwidth(a.CpuPeriod, a.CpuQuota, b.CpuPeriod, b.CpuQuota)
	return &pb.LinuxResources{
		Memory:     relaxedMemory(a.Memory, b.Memory),
		MemorySwap: relaxedMemorySwap(a.MemorySwap, b.MemorySwap),
		CpuShares:  relaxedCPUShares(a.CpuShares, b.CpuShares),
		CpuPeriod:  period,
		CpuQuota:   quota,
		CpusetCpus: relaxedCpuset(a.CpusetCpus, b.CpusetCpus),
		CpusetMems: relaxedCpuset(a.CpusetMems, b.CpusetMems),
	}
}

// 0 means unlimited (most relaxed); otherwise higher = more relaxed.
func relaxedMemory(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	return max(a, b)
}

// Docker memory-swap semantics: 0 = unset, -1 = unlimited (most relaxed),
// positive values are limits where higher = more relaxed.
func relaxedMemorySwap(a, b int64) int64 {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a == -1 || b == -1 {
		return -1
	}
	return max(a, b)
}

// 0 = unset (kernel default), not unlimited; otherwise higher = more relaxed.
func relaxedCPUShares(a, b uint64) uint64 {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	return max(a, b)
}

// relaxedCPUBandwidth picks the (period, quota) pair with the higher cap
// (quota/period) without mixing them. Quota 0 = unlimited. On a tie, shorter
// period wins (same cap executes more frequently).
func relaxedCPUBandwidth(periodA uint64, quotaA int64, periodB uint64, quotaB int64) (uint64, int64) {
	if quotaA == 0 && quotaB == 0 {
		return 0, 0
	}
	if quotaA == 0 {
		return periodA, 0
	}
	if quotaB == 0 {
		return periodB, 0
	}
	// Compare quota/period via cross-multiplication when both periods are set;
	// otherwise fall back to comparing raw quotas.
	if periodA != 0 && periodB != 0 {
		capA := quotaA * int64(periodB)
		capB := quotaB * int64(periodA)
		if capA > capB {
			return periodA, quotaA
		}
		if capB > capA {
			return periodB, quotaB
		}
		// equal caps: shorter period is more relaxed
		if periodA <= periodB {
			return periodA, quotaA
		}
		return periodB, quotaB
	}
	// At least one period is 0: compare quotas; on a tie, prefer the non-zero period.
	if quotaA > quotaB {
		return periodA, quotaA
	}
	if quotaB > quotaA {
		return periodB, quotaB
	}
	if periodA == 0 {
		return periodB, quotaB
	}
	return periodA, quotaA
}

// Empty = unset (most relaxed); otherwise the union of both sides, re-emitted
// in canonical form. On parse error, falls back to the side that parsed cleanly.
func relaxedCpuset(a, b string) string {
	if a == "" || b == "" {
		return ""
	}
	setA, errA := cpuset.Parse(a)
	setB, errB := cpuset.Parse(b)
	switch {
	case errA != nil && errB != nil:
		return a
	case errA != nil:
		return b
	case errB != nil:
		return a
	}
	for v := range setB {
		setA[v] = struct{}{}
	}
	return cpuset.Format(setA)
}
