//go:build ebpf_unsafe_memory_experiment

package ebpf

func init() {
	unsafeMemory = true
}
