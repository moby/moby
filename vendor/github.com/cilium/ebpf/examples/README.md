# eBPF Examples

- [kprobe](kprobe/) - Attach a program to the entry or exit of an arbitrary kernel symbol (function).
- [uretprobe](uretprobe/) - Like a kprobe, but for symbols in userspace binaries (e.g. `bash`).
- [tracepoint](tracepoint/) - Attach a program to predetermined kernel tracepoints.
- Add your use case(s) here!

## How to run

```bash
cd ebpf/examples/
go run -exec sudo [./kprobe, ./uretprobe, ./tracepoint, ...]
```
