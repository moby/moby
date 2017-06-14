
This repository contains Go bindings and sample code for [Hyper-V sockets](https://msdn.microsoft.com/en-us/virtualization/hyperv_on_windows/develop/make_mgmt_service) and [virtio sockets](http://stefanha.github.io/virtio/)(VSOCK).

## Organisation

- `pkg/hvsock`: Go binding for Hyper-V sockets
- `pkg/vsock`: Go binding for virtio VSOCK
- `examples`: Sample Go code and stress test
- `scripts`: Miscellaneous scripts
- `c`: Sample C code (including benchmarks and stress tests)
- `data`: Data from benchmarks


## Building

By default the Go sample code is build in a container. Simply type `make`.

If you want to build binaries on a local system use `make build-binaries`.

## Testing

There are READMEs in the [./examples](./examples/README.md) and
[./c](./c/README.md) sub-directories.  There is also a top-level
`make` target to build a
[linuxkit](https://github.com/linuxkit/linuxkit) image with the test
binaries included.

## Known limitations

- `hvsock`: The Windows side does not implement `accept()` due to
  limitations on some Windows builds where a VM can not connect to the
  host via Hyper-V sockets.

- `vsock`: There is no host side implementation as the interface is
  highly hypervisor specific. `examples` contains some code used to
  interact with VSOCK implementation in
  [Hyperkit](https://github.com/docker/hyperkit).

