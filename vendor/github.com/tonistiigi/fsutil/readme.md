Incremental file directory sync tools in golang.

```
BENCH_FILE_SIZE=10000 ./bench.test --test.bench .
BenchmarkCopyWithTar10-4                	    2000	    995242 ns/op
BenchmarkCopyWithTar50-4                	     300	   4710021 ns/op
BenchmarkCopyWithTar200-4               	     100	  16627260 ns/op
BenchmarkCopyWithTar1000-4              	      20	  60031459 ns/op
BenchmarkCPA10-4                        	    1000	   1678367 ns/op
BenchmarkCPA50-4                        	     500	   3690306 ns/op
BenchmarkCPA200-4                       	     200	   9495066 ns/op
BenchmarkCPA1000-4                      	      50	  29769289 ns/op
BenchmarkDiffCopy10-4                   	    2000	    943889 ns/op
BenchmarkDiffCopy50-4                   	     500	   3285950 ns/op
BenchmarkDiffCopy200-4                  	     200	   8563792 ns/op
BenchmarkDiffCopy1000-4                 	      50	  29511340 ns/op
BenchmarkDiffCopyProto10-4              	    2000	    944615 ns/op
BenchmarkDiffCopyProto50-4              	     500	   3334940 ns/op
BenchmarkDiffCopyProto200-4             	     200	   9420038 ns/op
BenchmarkDiffCopyProto1000-4            	      50	  30632429 ns/op
BenchmarkIncrementalDiffCopy10-4        	    2000	    691993 ns/op
BenchmarkIncrementalDiffCopy50-4        	    1000	   1304253 ns/op
BenchmarkIncrementalDiffCopy200-4       	     500	   3306519 ns/op
BenchmarkIncrementalDiffCopy1000-4      	     200	  10211343 ns/op
BenchmarkIncrementalDiffCopy5000-4      	      20	  55194427 ns/op
BenchmarkIncrementalDiffCopy10000-4     	      20	  91759289 ns/op
BenchmarkIncrementalCopyWithTar10-4     	    2000	   1020258 ns/op
BenchmarkIncrementalCopyWithTar50-4     	     300	   5348786 ns/op
BenchmarkIncrementalCopyWithTar200-4    	     100	  19495000 ns/op
BenchmarkIncrementalCopyWithTar1000-4   	      20	  70338507 ns/op
BenchmarkIncrementalRsync10-4           	      30	  45215754 ns/op
BenchmarkIncrementalRsync50-4           	      30	  45837260 ns/op
BenchmarkIncrementalRsync200-4          	      30	  48780614 ns/op
BenchmarkIncrementalRsync1000-4         	      20	  54801892 ns/op
BenchmarkIncrementalRsync5000-4         	      20	  84782542 ns/op
BenchmarkIncrementalRsync10000-4        	      10	 103355108 ns/op
BenchmarkRsync10-4                      	      30	  46776470 ns/op
BenchmarkRsync50-4                      	      30	  48601555 ns/op
BenchmarkRsync200-4                     	      20	  59642691 ns/op
BenchmarkRsync1000-4                    	      20	 101343010 ns/op
BenchmarkGnuTar10-4                     	     500	   3171448 ns/op
BenchmarkGnuTar50-4                     	     300	   5030296 ns/op
BenchmarkGnuTar200-4                    	     100	  10464313 ns/op
BenchmarkGnuTar1000-4                   	      50	  30375257 ns/op
```