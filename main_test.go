package main

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	goredis "github.com/go-redis/redis/v7"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/mediocregopher/radix/v3"

	redispipe "github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/redisconn"
)

const (
	defaultNetwork  = "tcp"
	defaultAddress  = "127.0.0.1:6379"
	defaultPoolSize = 10
	numKeys         = 100
)

func BenchmarkGoRedis(b *testing.B) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     "localhost:6379",
		PoolSize: 10,
	})
	defer client.Close()
	pipeline := client.Pipeline()

	mset := func(keyVals []string) {
		pipeline.MSet(keyVals)
		for j := 0; j < len(keyVals); j += 2 {
			pipeline.Expire(keyVals[j], 10*time.Second)
		}
		cerrs, err := pipeline.Exec()
		for eix, cerr := range cerrs {
			if cerr != nil && cerr.Err() != nil {
				log.Printf("Command error on #%d: %s", eix, cerr.Err().Error())
			}
		}
		if err != nil {
			log.Printf("Pipeline error: %s", err.Error())
		}
	}

	get := func(keys, values []string) ([]string, error) {
		vis, err := client.MGet(keys...).Result()
		if err != nil {
			return values, err
		}
		for _, vi := range vis {
			values = append(values, vi.(string))
		}

		return values, nil
	}

	doTest(b, mset, get)
}

func BenchmarkGoRedisDo(b *testing.B) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     "localhost:6379",
		PoolSize: 10,
	})
	defer client.Close()
	pipeline := client.Pipeline()

	msetArgs := make([]interface{}, (2*numKeys)+1)

	mset := func(keyVals []string) {
		msetArgs = append(msetArgs[:0], "mset")
		for _, kv := range keyVals {
			msetArgs = append(msetArgs, kv)
		}
		_ = pipeline.Process(goredis.NewStatusCmd(msetArgs...))
		for j := 0; j < len(keyVals); j += 2 {
			_ = pipeline.Process(goredis.NewBoolCmd("expire", keyVals[j], "10"))
		}
		cerrs, err := pipeline.Exec()
		for eix, cerr := range cerrs {
			if cerr != nil && cerr.Err() != nil {
				log.Printf("Command error on #%d: %s", eix, cerr.Err().Error())
			}
		}
		if err != nil {
			log.Printf("Pipeline error: %s", err.Error())
		}
	}

	mgetArgs := make([]interface{}, 0)
	mgetArgs = append(mgetArgs, "mget")
	get := func(keys, values []string) ([]string, error) {
		mgetArgs = mgetArgs[:1]
		for _, k := range keys {
			mgetArgs = append(mgetArgs, k)
		}
		cmd := goredis.NewStringSliceCmd(mgetArgs...)
		if err := client.Process(cmd); err != nil {
			return values, err
		}
		vis := cmd.Val()
		values = append(values, vis...)
		return values, nil
	}

	doTest(b, mset, get)
}

func BenchmarkRadix(b *testing.B) {
	pool, err := radix.NewPool(defaultNetwork, defaultAddress, defaultPoolSize)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	// if err := pool.Do(radix.Cmd(nil, "FLUSHDB")); err != nil {
	// 	panic(err)
	// }

	commands := make([]radix.CmdAction, 1+numKeys)
	mset := func(keyVals []string) {
		for i := 0; i < len(keyVals)/2; i++ {
			commands[1+i] = radix.Cmd(nil, "EXPIRE", keyVals[i*2], "10")
		}
		commands[0] = radix.Cmd(nil, "MSET", keyVals...)
		if err := pool.Do(radix.Pipeline(commands...)); err != nil {
			log.Printf("Pipeline failed: %s", err.Error())
		}
	}

	mget := func(keys, values []string) ([]string, error) {
		err := pool.Do(radix.Cmd(&values, "MGET", keys...))
		return values, err
	}

	doTest(b, mset, mget)
}

func BenchmarkRedigo(b *testing.B) {
	pool := &redigo.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redigo.Conn, error) {
			return redigo.Dial(defaultNetwork, defaultAddress)
		},
	}
	defer pool.Close()

	kvs := make([]interface{}, 2*numKeys)
	mset := func(keyVals []string) {
		conn := pool.Get()
		defer conn.Close()
		for ix, k := range keyVals {
			kvs[ix] = k
		}
		if err := conn.Send("MSET", kvs...); err != nil {
			log.Printf("Pipeline MSET: %s", err.Error())
			return
		}
		for i := 0; i < len(keyVals); i += 2 {
			if err := conn.Send("EXPIRE", keyVals[i], "10"); err != nil {
				log.Printf("Pipeline EXPIRE: %s", err.Error())
				return
			}
		}
		if err := conn.Flush(); err != nil {
			log.Printf("Pipeline flush: %s", err.Error())
		}
		if _, err := conn.Receive(); err != nil {
			log.Printf("Pipeline receive: %s", err.Error())
		}
	}

	keyInts := make([]interface{}, numKeys)
	mget := func(keys, values []string) ([]string, error) {
		conn := pool.Get()
		defer conn.Close()
		for i, k := range keys {
			keyInts[i] = k
		}
		reply, err := redigo.Values(conn.Do("MGET", keyInts...))
		if err != nil {
			return values, err
		}

		for _, item := range reply {
			values = append(values, string(item.([]byte)))
		}

		return values, nil
	}

	doTest(b, mset, mget)
}

func BenchmarkRedispipe(b *testing.B) {
	ctx := context.Background()

	connFactory := func(ctx context.Context) (redispipe.Sender, error) {
		opts := redisconn.Opts{
			Logger: redisconn.NoopLogger{}, // shut up logging. Could be your custom implementation.
			Handle: nil,
		}
		conn, err := redisconn.Connect(ctx, defaultAddress, opts)
		return conn, err
	}

	writeconn, err := connFactory(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer writeconn.Close()
	writesync := redispipe.SyncCtx{S: writeconn} // wrapper for synchronous api

	readconn, err := connFactory(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer readconn.Close()
	readsync := redispipe.SyncCtx{S: readconn} // wrapper for synchronous api

	kvs := make([]interface{}, 2*numKeys)
	requests := make([]redispipe.Request, 1+numKeys)
	mset := func(keyVals []string) {
		for i, x := range keyVals {
			kvs[i] = x

		}
		requests[0] = redispipe.Req("MSET", kvs...)
		for i := 0; i < len(keyVals)/2; i++ {
			requests[1+i] = redispipe.Req("EXPIRE", keyVals[i*2], "10")
		}

		if err := redispipe.AsError(writesync.SendMany(ctx, requests)); err != nil {
			log.Printf("Failed pipeline: %s", err.Error())
		}
	}

	keyInts := make([]interface{}, numKeys)
	mget := func(keys, values []string) ([]string, error) {
		for ix, k := range keys {
			keyInts[ix] = k
		}

		response := readsync.Send(ctx, redispipe.Req("MGET", keyInts...))
		if err := redispipe.AsError(response); err != nil {
			return values, err
		}
		for _, v := range response.([]interface{}) {
			values = append(values, string(v.([]byte)))
		}

		return values, nil
	}

	doTest(b, mset, mget)
}

type KeyValue [2]string

func (kv KeyValue) Key() string {
	return kv[0]
}

func (kv KeyValue) Value() string {
	return kv[1]
}

var keyValues = func() []KeyValue {
	numKeys := 50000
	keyvalues := make([]KeyValue, numKeys)
	for i := 0; i < numKeys; i++ {
		k := randomString(3)
		keyvalues[i] = KeyValue{k, k + "-" + randomString(97)}
	}
	return keyvalues
}()

func doTest(b *testing.B, mset func([]string), mget func([]string, []string) ([]string, error)) {
	items := make(chan []string, 1000)
	slicePool := sync.Pool{
		New: func() interface{} {
			item := make([]string, 0, numKeys)
			return item
		},
	}

	b.ResetTimer()
	go func() {
		keyVals := make([]string, 0, 2*numKeys)
		for i := 0; i < b.N; i++ {
			keys := slicePool.Get().([]string)
			keyVals = keyVals[:0]

			for j := 0; j < numKeys; j++ {
				kv := keyValues[((i*numKeys)+j)%len(keyValues)]
				keyVals = append(keyVals, kv[0], kv[1])
				keys = append(keys, kv[0])
			}
			mset(keyVals)
			// rand.Shuffle(len(keys), func(i int, j int) {
			// 	keys[i], keys[j] = keys[j], keys[i]
			// })
			items <- keys
		}
		close(items)
	}()

	values := make([]string, numKeys)
	cnt := 0
	for keys := range items {
		cnt++
		values, err := mget(keys, values[:0])
		if err != nil {
			b.Fatalf("GET failed: %s", err.Error())
		} else if len(keys) != len(values) {
			b.Fatalf("LENGTH MISMATCH: %d != %d", len(keys), len(values))
		} else {
			for i := 0; i < len(keys); i++ {
				if len(values[i]) < len(keys[i]) || keys[i] != values[i][0:len(keys[i])] {
					b.Fatalf("%d:%d MISMATCH: %s is not a prefix of %s", cnt, i, keys[i], values[i])
				}
			}
		}
		keys = keys[:0]
		slicePool.Put(keys)
	}
}
