package db

import (
	"collector-agent/pkg/logger"
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
)

const redisReadClientCap = 20

type RedisReadConnection struct {
	RedisReadClientChan chan *redis.Client
}

var read_once sync.Once
var internalRedisReadClient *RedisReadConnection

func NewRedisReadConnection() *RedisReadConnection {
	read_once.Do(func() {
		internalRedisReadClient = &RedisReadConnection{}
		internalRedisReadClient.RedisReadClientChan = make(chan *redis.Client, redisReadClientCap)

		options := redis.Options{
			Network: "unix",
			Addr:    "/app/run/redis.sock",
			DB:      0,
		}

		for i := 0; i < redisReadClientCap; i++ {
			c := redis.NewClient(&options)

			_, err := c.Ping(context.Background()).Result()
			logger.ExitIfErr(err, "Unable To Connect To Redis")
			internalRedisReadClient.RedisReadClientChan <- c
		}
		logger.Printf("RedisReadClientChan len: %d \n", len(internalRedisReadClient.RedisReadClientChan))
	})

	return internalRedisReadClient
}

func (rrc RedisReadConnection) GetClient() *redis.Client {
	return <-rrc.RedisReadClientChan
}

func (rrc RedisReadConnection) CloseClient(c *redis.Client) {
	rrc.RedisReadClientChan <- c
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
