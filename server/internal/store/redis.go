package store

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedis 初始化 Redis 连接（含重试）
func NewRedis(addr string, db int) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	var err error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = rdb.Ping(ctx).Err()
		cancel()
		if err == nil {
			return rdb, nil
		}
		log.Printf("waiting redis... (%d/30) %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}
