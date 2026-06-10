[![PkgGoDev](https://img.shields.io/badge/go.dev-docs-007d9c?style=flat-square&logo=go&logoColor=white)](https://pkg.go.dev/github.com/tonistiigi/fsutil)
[![CI Status](https://img.shields.io/github/actions/workflow/status/tonistiigi/fsutil/ci.yml?label=ci&logo=github&style=flat-square)](https://github.com/tonistiigi/fsutil/actions?query=workflow%3Aci)
[![Go Report Card](https://goreportcard.com/badge/github.com/tonistiigi/fsutil?style=flat-square)](https://goreportcard.com/report/github.com/tonistiigi/fsutil)
[![Codecov](https://img.shields.io/codecov/c/github/tonistiigi/fsutil?logo=codecov&style=flat-square)](https://codecov.io/gh/tonistiigi/fsutil)

Incremental file directory sync tools in golang.

```
BENCH_FILE_SIZE=10000 docker buildx bake bench-root
...
#17 0.303 + CGO_ENABLED=0 xx-go test -benchmem '-bench=.' '-run=^$' .
#17 0.303 + tee /tmp/fsutil.log
#17 1.527 BenchmarkWalker/depth_1_target-32                28356             42258 ns/op            9234 B/op        174 allocs/op
#17 3.166 BenchmarkWalker/depth_1_doublestar_target-32             28647             42038 ns/op            9282 B/op        175 allocs/op
#17 4.865 BenchmarkWalker/depth_2_star_target-32                    1184           1009371 ns/op          200654 B/op       3971 allocs/op
#17 6.342 BenchmarkWalker/depth_2_doublestar_target-32              1148           1007115 ns/op          195891 B/op       3908 allocs/op
#17 9.339 BenchmarkWalker/depth_3_star_star_target-32                 39          28146516 ns/op         5221363 B/op     100915 allocs/op
#17 13.54 BenchmarkWalker/depth_3_doublestar_target-32                40          28496829 ns/op         5206464 B/op     100828 allocs/op
#17 17.99 BenchmarkWalker/depth_4_star_star_star_target-32            26          48224854 ns/op         6571213 B/op     119421 allocs/op
#17 25.32 BenchmarkWalker/depth_4_doublestar_target-32                24          45061931 ns/op         6488522 B/op     119315 allocs/op
#17 28.67 BenchmarkWalker/depth_5_star_star_star_star_target-32                       54          22124864 ns/op         2476377 B/op      42818 allocs/op
#17 32.59 BenchmarkWalker/depth_5_doublestar_target-32                                49          21479412 ns/op         2460690 B/op      42699 allocs/op
#17 35.09 BenchmarkWalker/depth_6_star_star_star_star_star_target-32                  28          38307776 ns/op         3998884 B/op      67772 allocs/op
#17 38.05 BenchmarkWalker/depth_6_doublestar_target-32                                31          38242074 ns/op         3980841 B/op      67634 allocs/op
#17 42.92 BenchmarkWalker/depth_6_doublestar_exclude_star_star_doublestar-32                2925            393602 ns/op           47439 B/op       1006 allocs/op
#17 45.99 + cd bench
#17 45.99 + CGO_ENABLED=0 xx-go test -benchmem '-bench=.' '-run=^$' .
#17 45.99 + tee /tmp/bench.log
#17 46.84 BenchmarkCopyWithTar10-32                          283           4291776 ns/op          906824 B/op        843 allocs/op
#17 50.05 BenchmarkCopyWithTar50-32                           50          24077499 ns/op         4999874 B/op       4514 allocs/op
#17 54.00 BenchmarkCopyWithTar200-32                          15          74350687 ns/op        18757006 B/op      15347 allocs/op
#17 56.31 BenchmarkCopyWithTar1000-32                          5         259188166 ns/op        72393427 B/op      55045 allocs/op
#17 61.31 BenchmarkCPA10-32                                  339           3496169 ns/op            7102 B/op         77 allocs/op
#17 64.68 BenchmarkCPA50-32                                   74          15808000 ns/op            7102 B/op         77 allocs/op
#17 67.26 BenchmarkCPA200-32                                  26          45892546 ns/op            7101 B/op         77 allocs/op
#17 70.08 BenchmarkCPA1000-32                                  8         147602854 ns/op            7103 B/op         77 allocs/op
#17 72.60 BenchmarkDiffCopy10-32                             392           3045072 ns/op          219789 B/op       1123 allocs/op
#17 76.15 BenchmarkDiffCopy50-32                              82          14024073 ns/op         1253831 B/op       5460 allocs/op
#17 78.83 BenchmarkDiffCopy200-32                             27          41259003 ns/op         4683591 B/op      18640 allocs/op
#17 81.60 BenchmarkDiffCopy1000-32                             8         125244042 ns/op        17285147 B/op      67649 allocs/op
#17 83.90 BenchmarkDiffCopyProto10-32                        412           2934103 ns/op          232184 B/op       1143 allocs/op
#17 87.46 BenchmarkDiffCopyProto50-32                         80          13809955 ns/op         1273311 B/op       5565 allocs/op
#17 90.02 BenchmarkDiffCopyProto200-32                        30          41169665 ns/op         4697476 B/op      18995 allocs/op
#17 93.05 BenchmarkDiffCopyProto1000-32                        8         127126319 ns/op        17334920 B/op      68883 allocs/op
#17 95.37 BenchmarkIncrementalDiffCopy10-32                 1540            779568 ns/op          119068 B/op       1015 allocs/op
#17 97.84 BenchmarkIncrementalDiffCopy50-32                  782           1513121 ns/op          455740 B/op       4285 allocs/op
#17 99.97 BenchmarkIncrementalDiffCopy200-32                 271           4248983 ns/op         1329875 B/op      13603 allocs/op
#17 102.3 BenchmarkIncrementalDiffCopy1000-32                 84          14027390 ns/op         4552790 B/op      46849 allocs/op
#17 105.4 BenchmarkIncrementalDiffCopy5000-32                 14          73269136 ns/op        24500669 B/op     266772 allocs/op
#17 110.6 BenchmarkIncrementalDiffCopy10000-32                 9         128400623 ns/op        43706410 B/op     472982 allocs/op
#17 114.4 BenchmarkIncrementalCopyWithTar10-32               396           2946217 ns/op          915096 B/op        826 allocs/op
#17 116.3 BenchmarkIncrementalCopyWithTar50-32                69          16767967 ns/op         5093048 B/op       4472 allocs/op
#17 117.7 BenchmarkIncrementalCopyWithTar200-32               19          62520307 ns/op        19081658 B/op      15270 allocs/op
#17 119.7 BenchmarkIncrementalCopyWithTar1000-32               5         239712851 ns/op        73113704 B/op      54938 allocs/op
#17 122.8 BenchmarkIncrementalRsync10-32                      26          43773727 ns/op            6608 B/op         69 allocs/op
#17 124.1 BenchmarkIncrementalRsync50-32                      25          45985011 ns/op            6608 B/op         69 allocs/op
#17 125.6 BenchmarkIncrementalRsync200-32                     22          49976819 ns/op            6608 B/op         69 allocs/op
#17 127.3 BenchmarkIncrementalRsync1000-32                    19          63618139 ns/op            6600 B/op         69 allocs/op
#17 130.6 BenchmarkIncrementalRsync5000-32                     8         132002745 ns/op            6608 B/op         69 allocs/op
#17 136.3 BenchmarkIncrementalRsync10000-32                    6         187247351 ns/op            6608 B/op         69 allocs/op
#17 140.3 BenchmarkRsync10-32                                 25          46054741 ns/op            6606 B/op         69 allocs/op
#17 141.6 BenchmarkRsync50-32                                 19          58922835 ns/op            6605 B/op         69 allocs/op
#17 143.2 BenchmarkRsync200-32                                13          91176938 ns/op            6606 B/op         69 allocs/op
#17 145.5 BenchmarkRsync1000-32                                6         198319527 ns/op            6606 B/op         69 allocs/op
#17 147.6 BenchmarkGnuTar10-32                               268           4489528 ns/op           14192 B/op        151 allocs/op
#17 150.8 BenchmarkGnuTar50-32                                54          20528041 ns/op           14192 B/op        151 allocs/op
#17 152.9 BenchmarkGnuTar200-32                               19          60394926 ns/op           14192 B/op        151 allocs/op
#17 155.4 BenchmarkGnuTar1000-32                               6         198630328 ns/op           14192 B/op        151 allocs/op
```
