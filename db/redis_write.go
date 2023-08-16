package db

import (
	"collector-agent/pkg/logger"
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
)

const redisWriteClientCap = 20

type RedisWriteConnection struct {
	RedisWriteClientChan chan *redis.Client
}

var write_once sync.Once
var internalRedisWriteClient *RedisWriteConnection

func NewRedisWriteConnection() *RedisWriteConnection {
	write_once.Do(func() {
		internalRedisWriteClient = &RedisWriteConnection{}
		internalRedisWriteClient.RedisWriteClientChan = make(chan *redis.Client, redisWriteClientCap)

		options := redis.Options{
			Network: "unix",
			Addr:    "/app/run/redis.sock",
			DB:      0,
		}

		for i := 0; i < redisWriteClientCap; i++ {
			c := redis.NewClient(&options)

			_, err := c.Ping(context.Background()).Result()
			logger.ExitIfErr(err, "Unable To Connect To Redis")
			internalRedisWriteClient.RedisWriteClientChan <- c
		}
		logger.Printf("RedisWriteClientChan len: %d \n", len(internalRedisWriteClient.RedisWriteClientChan))
	})

	return internalRedisWriteClient
}

func (rrc RedisWriteConnection) GetClient() *redis.Client {
	return <-rrc.RedisWriteClientChan
}

func (rrc RedisWriteConnection) CloseClient(c *redis.Client) {
	rrc.RedisWriteClientChan <- c
}
