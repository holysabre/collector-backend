package db

import (
	"collector-agent/pkg/logger"
	"context"
	"runtime"
	"sync"

	"github.com/go-redis/redis/v8"
)

const PoolCapPreCoreNum = 10

var (
	once   sync.Once
	client *redis.Client
)

func GetRedisClient() *redis.Client {
	once.Do(func() {
		numCPU := runtime.NumCPU()
		poolCap := PoolCapPreCoreNum * numCPU
		logger.Printf("redis pool size: %d", poolCap)
		options := redis.Options{
			Network:  "unix",
			Addr:     "/app/run/redis.sock",
			DB:       0,
			PoolSize: int(poolCap),
		}

		client = redis.NewClient(&options)
	})

	// 获取连接池的统计信息
	stats := client.PoolStats()

	// 打印连接池的统计信息
	logger.Printf("Redis Pool Stats, TotalConns: %d, IdleConns: %d, StaleConns: %d", stats.TotalConns, stats.IdleConns, stats.StaleConns)

	return client
}

func GetRedisConnection() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Network: "unix",
		Addr:    "/app/run/redis.sock",
		DB:      0,
	})

	// 使用 Ping() 方法检查是否成功连接到 Redis
	_, err := client.Ping(context.Background()).Result()
	logger.ExitIfErr(err, "Unable To Connect To Redis")

	return client
}
