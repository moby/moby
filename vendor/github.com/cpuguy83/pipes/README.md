# Pipes

A library to work with OS pipes which takes advantage of OS-level optimizations.

## Why pipes?

The idea behind this library is to enable copying between different file
descriptors without the normal overhead of `read(bytes) && write(bytes)`, which
is 2 system calls and 2 copies between userspace and kernel space.

In the implementation here we use the splice(2) system call to move bytes from
the read side to the write side. This means there is only 1 system call to
perform and no data copied into userspace. In many cases there is no copying
even in kernel space as the pointer to the data is just shifted between the two
fd's.

The important thing to note here is that the only benefit that this library
brings is if you are copying between to another file descriptor (such as, but
not limited to, a regular file or a tcp socket).

### Benchmarks

This compares against using io.Copy directly on the underlying *os.File vs the
optimized ReadFrom implementation here.

A note about these benchmarks, as it turns out it is pretty difficult to test
throughput accurately. For instance the tests currently just dumbly drain data
out of the pipe with `io.Copy(ioutil.Discard, pipe)` while the benchmark is in
progress so we can test how write speed. This can in and of itself be a
bottleneck. However, the benchmarks do let us compare throughput across
different implementations with the same bottlenecks.

```
benchmark                                  old ns/op     new ns/op     delta
BenchmarkReadFrom/regular_file/16K-4       32933         16940         -48.56%
BenchmarkReadFrom/regular_file/32K-4       43407         24365         -43.87%
BenchmarkReadFrom/regular_file/64K-4       65497         39097         -40.31%
BenchmarkReadFrom/regular_file/128K-4      102643        61719         -39.87%
BenchmarkReadFrom/regular_file/256K-4      172160        99404         -42.26%
BenchmarkReadFrom/regular_file/512K-4      299951        178221        -40.58%
BenchmarkReadFrom/regular_file/1MB-4       552199        322710        -41.56%
BenchmarkReadFrom/regular_file/10MB-4      5256567       3632938       -30.89%
BenchmarkReadFrom/regular_file/100MB-4     54652792      35899298      -34.31%
BenchmarkReadFrom/regular_file/1GB-4       552312826     383047329     -30.65%

benchmark                                  old MB/s     new MB/s     speedup
BenchmarkReadFrom/regular_file/16K-4       497.49       967.17       1.94x
BenchmarkReadFrom/regular_file/32K-4       754.89       1344.89      1.78x
BenchmarkReadFrom/regular_file/64K-4       1000.60      1676.24      1.68x
BenchmarkReadFrom/regular_file/128K-4      1276.96      2123.68      1.66x
BenchmarkReadFrom/regular_file/256K-4      1522.67      2637.17      1.73x
BenchmarkReadFrom/regular_file/512K-4      1747.91      2941.78      1.68x
BenchmarkReadFrom/regular_file/1MB-4       1898.91      3249.28      1.71x
BenchmarkReadFrom/regular_file/10MB-4      1994.79      2886.30      1.45x
BenchmarkReadFrom/regular_file/100MB-4     1918.61      2920.88      1.52x
BenchmarkReadFrom/regular_file/1GB-4       1944.08      2803.16      1.44x
```
