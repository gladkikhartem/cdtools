package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	_ "net/http/pprof"

	"github.com/cockroachdb/pebble"
	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v2"

	"github.com/buaazp/fasthttprouter"
	_ "github.com/mattn/go-sqlite3"

	_ "github.com/lib/pq"
)

type Config struct {
	ListenAddr string         `yaml:"ListenAddr"`
	DBPath     string         `yaml:"DBPath"`
	DBOptions  pebble.Options `yaml:"DBOptions"`
	// TODO: backups
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := Start(ctx)
	if err != nil {
		panic(err)
	}
}

var store *Store

func Start(ctx context.Context) error {
	var cfg Config
	yd, err := os.ReadFile("config.yml")
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yd, &cfg)
	if err != nil {
		return err
	}
	db, err := pebble.Open(cfg.DBPath, &cfg.DBOptions)
	if err != nil {
		return err
	}
	store = NewStore(db)
	InitFastLocks()
	go func() {
		log.Print("START ", cfg.ListenAddr)
		router := fasthttprouter.New()
		// In memory APIs. Not persistent
		router.POST("/db/:acc/lock/:id", FastLockHandler)
		router.DELETE("/db/:acc/lock/:id", FastUnlockHandler)

		// Persistent APIs
		router.GET("/db/:acc/seq/:id", GetSequenceHandler)
		router.POST("/db/:acc/seq/:id", NextSequenceHandler)
		router.DELETE("/db/:acc/seq/:id", DeleteSequenceHandler)

		router.GET("/db/:acc/counter/:id", GetCounterHandler)
		router.POST("/db/:acc/counter/:id", AddCounterHandler)
		router.DELETE("/db/:acc/counter/:id", DeleteCounterHandler)

		// router.GET("/db/:acc/kv/:id", GetKVHandler)
		// router.POST("/db/:acc/kv/:id", SetKVHandler)
		// router.DELETE("/db/:acc/kv/:id", DeleteKVHandler)

		// router.GET("/db/:acc/lock/:id", GetLockHandler)
		// router.POST("/db/:acc/lock/:id", SetLockHandler)
		// router.DELETE("/db/:acc/lock/:id", DeleteLockHandler)

		router.NotFound = func(ctx *fasthttp.RequestCtx) {
			ctx.SetStatusCode(404)
		}

		s := fasthttp.Server{
			Handler:                       router.Handler,
			Concurrency:                   100000,
			MaxConnsPerIP:                 100000,
			ReadBufferSize:                10000,
			WriteBufferSize:               10000,
			DisableHeaderNamesNormalizing: true,
			NoDefaultContentType:          true,
			NoDefaultDate:                 true,
			NoDefaultServerHeader:         true,
		}
		err := s.ListenAndServe(cfg.ListenAddr)
		if err != nil {
			panic(err)
		}
	}()

	// go func() {
	err = store.FlushLoop(ctx)
	if err != nil {
		panic(err)
	}
	// }()
	// time.Sleep(time.Second * 1)
	// start := time.Now()
	// BenchmarkSequence1000Clients1000Keys()
	// log.Print("BenchmarkSequence1000Clients1000Keys took sec: ", time.Since(start).Seconds())
	// time.Sleep(time.Second * 1)
	// start = time.Now()
	// BenchmarkFastLocks1000Clients1000RandomKeys()
	// log.Print("BenchmarkFastLocks1000Clients1000RandomKeys took sec: ", time.Since(start).Seconds())
	// time.Sleep(time.Second * 1)

	return nil
}

// func BenchmarkSequence1000Clients1000Keys() {
// 	var wg sync.WaitGroup
// 	c := fasthttp.Client{}
// 	// c.Addr = "" + addr + ""
// 	// c.MaxPendingRequests = 1000
// 	c.NoDefaultUserAgentHeader = true
// 	// c.MaxConns = 10000
// 	c.MaxConnsPerHost = 10000
// 	var urls = []*fasthttp.URI{}
// 	for i := 0; i < 10000; i++ {
// 		lockURI := fasthttp.AcquireURI()
// 		lockURI.Parse(nil, []byte(fmt.Sprintf("http://localhost"+addr+"/db/123/seq/%d", i)))
// 		urls = append(urls, lockURI)
// 	}

// 	for i := 0; i < 1000; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
// 			for j := 0; j < 1000; j++ {
// 				req := fasthttp.AcquireRequest()
// 				req.Header.SetMethod("POST")
// 				u := urls[rnd.Intn(10000)]
// 				req.SetURI(u)
// 				resp := fasthttp.AcquireResponse()
// 				err := c.Do(req, resp)
// 				if err != nil {
// 					panic(err)
// 				}
// 				if resp.StatusCode() != 200 {
// 					panic(resp.StatusCode())
// 				}
// 				fasthttp.ReleaseRequest(req)
// 				fasthttp.ReleaseResponse(resp)
// 			}
// 		}(i)
// 	}
// 	wg.Wait()

// }

// func BenchmarkFastLocks1000Clients1000RandomKeys() {
// 	var wg sync.WaitGroup
// 	for i := 0; i < 1000; i++ {
// 		wg.Add(1)
// 		go func(i int) {
// 			defer wg.Done()
// 			rnd := rand.New(rand.NewSource(time.Now().Unix()))
// 			for j := 0; j < 1000; j++ {
// 				id := fmt.Sprint(rnd.Int63())
// 				h, err := fastLockHandler("1", id, 10, 10)
// 				if err != nil {
// 					panic(err)
// 				}
// 				fastUnlockHandler("1", id, h)
// 			}
// 		}(i)
// 	}
// 	wg.Wait()
// }

// func BenchmarkFastLocks1000Clients1000RandomKeys() {
// 	c := fasthttp.Client{}
// 	c.MaxConnsPerHost = 30000
// 	c.ReadBufferSize = 10000
// 	c.WriteBufferSize = 10000
// 	c.ReadTimeout = time.Second * 5
// 	c.WriteTimeout = time.Second * 5
// 	c.NoDefaultUserAgentHeader = true
// 	c.DisableHeaderNamesNormalizing = true
// 	c.DisablePathNormalizing = true
// 	c.ConnPoolStrategy = fasthttp.LIFO
// 	var wg sync.WaitGroup

// 	for i := 0; i < 400; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
// 			req := fasthttp.AcquireRequest()
// 			resp := fasthttp.AcquireResponse()
// 			for j := 0; j < 1000000/400; j++ {
// 				req.Header.SetMethod("POST")
// 				lockURI := fasthttp.AcquireURI()
// 				lockURI.Parse(nil, []byte(fmt.Sprintf("http://localhost"+addr+"/db/123/flock/%d?dur=10&wait=10", rnd.Intn(100000))))
// 				req.SetURI(lockURI)
// 				err := c.Do(req, resp)
// 				if err != nil {
// 					panic(err)
// 				}
// 				if resp.StatusCode() != 200 {
// 					panic(string(resp.Body()))
// 				}
// 				req.SetURI(lockURI)
// 				req.Header.SetMethod("DELETE")
// 				err = c.Do(req, resp)
// 				if err != nil {
// 					panic(err)
// 				}
// 				if resp.StatusCode() != 200 {
// 					panic(string(resp.Body()))
// 				}
// 				fasthttp.ReleaseURI(lockURI)
// 			}
// 			fasthttp.ReleaseRequest(req)
// 			fasthttp.ReleaseResponse(resp)
// 		}()
// 	}
// 	wg.Wait()
// }

// func BenchmarkKV1000Clients1000000RandomKeys() {
// 	c := fasthttp.Client{}
// 	c.MaxConnsPerHost = 30000
// 	c.ReadBufferSize = 10000
// 	c.WriteBufferSize = 10000
// 	c.NoDefaultUserAgentHeader = true
// 	c.DisableHeaderNamesNormalizing = true
// 	c.DisablePathNormalizing = true
// 	c.ConnPoolStrategy = fasthttp.LIFO
// 	var wg sync.WaitGroup

// 	for i := 0; i < 1000; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			rnd := rand.New(rand.NewSource(time.Now().Unix()))
// 			req := fasthttp.AcquireRequest()
// 			resp := fasthttp.AcquireResponse()
// 			for j := 0; j < 1000; j++ {
// 				kvURI := fasthttp.AcquireURI()
// 				kvURI.Parse(nil, []byte(fmt.Sprintf("http://localhost"+addr+"/db/123/kv/%d", rnd.Int63())))
// 				req.Header.SetMethod("POST")
// 				req.SetURI(kvURI)
// 				err := c.Do(req, resp)
// 				if err != nil {
// 					panic(err)
// 				}
// 				if resp.StatusCode() != 200 {
// 					panic(string(resp.Body()))
// 				}
// 				req.SetURI(kvURI)
// 				req.Header.SetMethod("DELETE")
// 				err = c.Do(req, resp)
// 				if err != nil {
// 					panic(err)
// 				}
// 				if resp.StatusCode() != 200 {
// 					panic(string(resp.Body()))
// 				}
// 			}
// 			fasthttp.ReleaseRequest(req)
// 			fasthttp.ReleaseResponse(resp)
// 		}()
// 	}
// 	wg.Wait()
// }
