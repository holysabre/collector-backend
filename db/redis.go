package db

import (
	"collector-agent/util"
	"context"

	"github.com/go-redis/redis/v8"
)

func GetRedisConnection() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Network: "unix",
		Addr:    "/app/run/redis.sock",
		DB:      0,
	})

	// 使用 Ping() 方法检查是否成功连接到 Redis
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		util.FailOnError(err, "连接Redis失败")
	}

	return client
}
