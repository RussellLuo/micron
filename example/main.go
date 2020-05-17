package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RussellLuo/micron/cron"
	redislocker "github.com/RussellLuo/micron/locker/redis"
	"github.com/go-redis/redis"
)

func main() {
	addr := flag.String("addr", "localhost:6379", "The address of the Redis server.")
	flag.Parse()

	c := cron.New(
		redislocker.New(redis.NewClient(&redis.Options{
			Addr: *addr,
		})),
		&cron.Options{
			Timezone: "Asia/Shanghai",
			LockTTL:  2 * time.Second, // Assume the maximal clock error is 2s.
			ErrHandler: func(err error) {
				log.Printf("err: %v", err)
			},
		},
	)

	// The job will be executed at every 5th second.
	c.Add("test", "*/5 * * * * * *", func() { // nolint:errcheck
		log.Printf("hello")
	})
	c.Start()
	log.Println("Cron started successfully")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	c.Stop()
}
