# Redis Client Smackdown

This test assumes that you are running a "real" Redis server on your local machine at the default port (6379). It calls `FLUSH_DB`, so be careful.

## General usage
```
$ go test -v -bench . -benchtime 2s -benchmem

goos: darwin
goarch: amd64
pkg: github.com/dangermike/redis_client_smackdown
BenchmarkGoRedis-4          2251        896257 ns/op      139271 B/op       1025 allocs/op
BenchmarkGoRedisDo-4        2230        920441 ns/op      130264 B/op        915 allocs/op
BenchmarkRadix-4            2655        984269 ns/op      105825 B/op        206 allocs/op
BenchmarkRedigo-4           2522        803750 ns/op      222025 B/op        907 allocs/op
BenchmarkRedispipe-4        2301        967403 ns/op      223027 B/op        912 allocs/op
PASS
ok      github.com/dangermike/redis_client_smackdown    19.508s

```

## Testing a specific client
It is likely you're going to want to look at a specific client to see how it did. The listing below will show the results for a single client.

```
go test -v -bench BenchmarkGoRedisDo -benchtime 2s -benchmem -memprofile mem.pprof
goos: darwin
goarch: amd64
pkg: github.com/dangermike/redis_client_smackdown
BenchmarkGoRedisDo-4        2032       1172792 ns/op      130265 B/op        915 allocs/op
PASS
ok      github.com/dangermike/redis_client_smackdown    4.777s
```

You can then view the allocation graph in your browser.
```
$ go tool pprof mem.pprof
Type: alloc_space
Time: Nov 12, 2019 at 11:31am (EST)
Entering interactive mode (type "help" for commands, "o" for options)
(pprof) web
```
