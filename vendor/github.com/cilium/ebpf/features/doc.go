// Package features allows probing for BPF features available to the calling process.
//
// In general, the error return values from feature probes in this package
// all have the following semantics unless otherwise specified:
//
//	err == nil: The feature is available.
//	errors.Is(err, ebpf.ErrNotSupported): The feature is not available.
//	err != nil: Any errors encountered during probe execution, wrapped.
//
// Note that the latter case may include false negatives, and that resource
// creation may succeed despite an error being returned. For example, some
// map and program types cannot reliably be probed and will return an
// inconclusive error.
//
// As a rule, only `nil` and `ebpf.ErrNotSupported` are conclusive.
//
// Probe results are cached by the library and persist throughout any changes
// to the process' environment, like capability changes.
package features
