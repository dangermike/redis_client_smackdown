# Redis Client Smackdown

This test assumes that you are running a "real" Redis server on your local machine at the default port (6379). It calls `FLUSH_DB`, so be careful.

## General usage

```shell
$ go test -v -bench . -benchtime 2s -benchmem
goos: darwin
goarch: amd64
pkg: github.com/dangermike/redis_client_smackdown
BenchmarkGoRedis-4          2511        877486 ns/op      139266 B/op        1025 allocs/op
BenchmarkGoRedisDo-4        2131        965186 ns/op      126839 B/op        813 allocs/op
BenchmarkRadix-4            2227       1045498 ns/op      105836 B/op        206 allocs/op
BenchmarkRedigo-4           2737        952412 ns/op      222023 B/op        907 allocs/op
BenchmarkRedispipe-4        2295       1002247 ns/op      223036 B/op        912 allocs/op
PASS
ok      github.com/dangermike/redis_client_smackdown    17.884s
```

## Testing a specific client

It is likely you're going to want to look at a specific client to see how it did. The listing below will show the results for a single client.

```shell
$ go test -v -bench BenchmarkGoRedisDo -benchtime 2s -benchmem -memprofile mem.pprof
goos: darwin
goarch: amd64
pkg: github.com/dangermike/redis_client_smackdown
BenchmarkGoRedisDo-4        2925        942208 ns/op      126841 B/op        813 allocs/op
PASS
ok      github.com/dangermike/redis_client_smackdown    3.501s
```

You can then view the allocation graph in your browser.

```shell
$ go tool pprof mem.pprof
Type: alloc_space
Time: Nov 12, 2019 at 11:31am (EST)
Entering interactive mode (type "help" for commands, "o" for options)
(pprof) web
```
