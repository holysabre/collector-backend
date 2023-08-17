package db

import (
	"collector-agent/pkg/logger"
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
)

const redisClientCap = 30

var (
	once   sync.Once
	client *redis.Client
)

func GetRedisClient() *redis.Client {
	once.Do(func() {
		options := redis.Options{
			Network:  "unix",
			Addr:     "/app/run/redis.sock",
			DB:       0,
			PoolSize: redisClientCap,
		}

		client = redis.NewClient(&options)
	})

	return client
}

// func (rrc RedisConnection) GetClient() *redis.Client {
// 	log.Println(rrc.RedisClient.PoolStats())
// 	return rrc.RedisClient
// }

// func (rrc RedisConnection) CloseClient(c *redis.Client) {
// 	rrc.RedisClient.Close()
// }

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
