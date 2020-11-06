# zerorpc
A lightweight, bidirectional RPC (WIP!)

```
[mehmet@zero zerorpc]$ go test ./core ./benchmark -bench='RPC' -benchtime=3s -cpu=1,2,4
goos: linux
goarch: amd64
pkg: github.com/mgurevin/zerorpc/core
BenchmarkZeroRPCCall/Call           	  253867	     14727 ns/op	     67900 call/sec	       0 B/op	       0 allocs/op
BenchmarkZeroRPCCall/Call-2         	  354513	     10604 ns/op	     94307 call/sec	       0 B/op	       0 allocs/op
BenchmarkZeroRPCCall/Call-4         	  401074	      7631 ns/op	    131046 call/sec	       0 B/op	       0 allocs/op
BenchmarkZeroRPCPush/Push           	  516235	      6100 ns/op	    163924 push/sec	       0 B/op	       0 allocs/op
BenchmarkZeroRPCPush/Push-2         	  847354	      4591 ns/op	    217820 push/sec	       0 B/op	       0 allocs/op
BenchmarkZeroRPCPush/Push-4         	  986324	      3800 ns/op	    263133 push/sec	       0 B/op	       0 allocs/op
PASS
ok  	github.com/mgurevin/zerorpc/core	27.332s
goos: linux
goarch: amd64
pkg: github.com/mgurevin/zerorpc/benchmark
BenchmarkGRPCCall/Call           	  102626	     35113 ns/op	     28480 call/sec	    8496 B/op	     167 allocs/op
BenchmarkGRPCCall/Call-2         	  169965	     22603 ns/op	     44242 call/sec	    8290 B/op	     157 allocs/op
BenchmarkGRPCCall/Call-4         	  253670	     14462 ns/op	     69147 call/sec	    8250 B/op	     154 allocs/op
PASS
ok  	github.com/mgurevin/zerorpc/benchmark	11.846s
```

