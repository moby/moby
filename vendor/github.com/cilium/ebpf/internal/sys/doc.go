// Package sys contains bindings for the BPF syscall.
package sys

//go:generate go run github.com/cilium/ebpf/internal/cmd/gentypes ../btf/testdata/vmlinux-btf.gz
