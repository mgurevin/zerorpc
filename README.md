# zerorpc
A lightweight, bidirectional RPC (WIP!)

```goos: linux
goarch: amd64
pkg: github.com/mgurevin/zerorpc/core
BenchmarkRPCCall/1         	   60192	     18306 ns/op	     54626 call/sec	       0 B/op	       0 allocs/op
BenchmarkRPCCall/2         	   58224	     19277 ns/op	     51877 call/sec	       0 B/op	       0 allocs/op
BenchmarkRPCCall/4         	   62991	     18885 ns/op	     52952 call/sec	       0 B/op	       0 allocs/op
BenchmarkRPCCall/8         	   61892	     18759 ns/op	     53308 call/sec	       0 B/op	       0 allocs/op
BenchmarkRPCPush/1         	  140817	      8169 ns/op	    122410 push/sec	       0 B/op	       0 allocs/op
BenchmarkRPCPush/2         	  142762	      8279 ns/op	    120787 push/sec	       0 B/op	       0 allocs/op
BenchmarkRPCPush/4         	  142092	      8285 ns/op	    120705 push/sec	       0 B/op	       0 allocs/op
BenchmarkRPCPush/8         	  138418	      8289 ns/op	    120649 push/sec	       0 B/op	       0 allocs/op
PASS
ok  	github.com/mgurevin/zerorpc/core	10.439s```

