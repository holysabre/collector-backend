package db

import (
	"collector-agent/pkg/logger"
	"context"
	"log"
	"sync"

	"github.com/go-redis/redis/v8"
)

const redisClientCap = 30

type RedisConnection struct {
	RedisClient *redis.Client
}

var read_once sync.Once
var internalRedisClient *RedisConnection

func NewRedisConnection() *RedisConnection {
	read_once.Do(func() {
		// internalRedisClient.RedisClient = make(chan *redis.Client, redisReadClientCap)

		options := redis.Options{
			Network:  "unix",
			Addr:     "/app/run/redis.sock",
			DB:       0,
			PoolSize: redisClientCap,
		}

		c := redis.NewClient(&options)

		// for i := 0; i < redisReadClientCap; i++ {
		// 	c := redis.NewClient(&options)

		_, err := c.Ping(context.Background()).Result()
		logger.ExitIfErr(err, "Unable To Connect To Redis")
		internalRedisClient.RedisClient = c
		// }
		// logger.Printf("RedisReadClientChan len: %d \n", len(internalRedisReadClient.RedisReadClientChan))
	})

	return internalRedisClient
}

func (rrc RedisConnection) GetClient() *redis.Client {
	log.Println(rrc.RedisClient.PoolStats())
	return rrc.RedisClient
}

func (rrc RedisConnection) CloseClient(c *redis.Client) {
	rrc.RedisClient.Close()
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
