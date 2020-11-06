# zerorpc
A lightweight, bidirectional RPC (WIP!)

```
[mehmet@zero zerorpc]$ go test ./core -bench='BenchmarkRPC(Call|Push)' -benchtime=3s -cpu=1,2,4,8 -cpuprofile=cpu.profile -memprofile=mem.profile -mutexprofile=mutex.profile 
goos: linux
goarch: amd64
pkg: github.com/mgurevin/zerorpc/core
BenchmarkRPCCall/Call           	  249330	     14458 ns/op	     69166 call/sec	       0 B/op	       0 allocs/op
BenchmarkRPCCall/Call-2         	  308313	     10532 ns/op	     94948 call/sec	       1 B/op	       0 allocs/op
BenchmarkRPCCall/Call-4         	  466051	      9067 ns/op	    110291 call/sec	       1 B/op	       0 allocs/op
BenchmarkRPCCall/Call-8         	  403508	      9147 ns/op	    109317 call/sec	       1 B/op	       0 allocs/op
BenchmarkRPCPush/Push           	  435667	      7269 ns/op	    137562 push/sec	       0 B/op	       0 allocs/op
BenchmarkRPCPush/Push-2         	  820389	      5073 ns/op	    197129 push/sec	       0 B/op	       0 allocs/op
BenchmarkRPCPush/Push-4         	  919329	      3877 ns/op	    257931 push/sec	       0 B/op	       0 allocs/op
BenchmarkRPCPush/Push-8         	  708032	      4601 ns/op	    217328 push/sec	       0 B/op	       0 allocs/op
PASS
ok  	github.com/mgurevin/zerorpc/core	32.872s
```

