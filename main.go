package main

import (
	"collector-agent/db"
	"collector-agent/models/msg"
	"collector-agent/pkg/rabbitmq"
	"collector-agent/util"
	"context"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
)

func main() {
	run()
}

func run() {
	// 访问环境变量
	amqpUsername := os.Getenv("RABBITMQ_USERNAME")
	amqpPassowd := os.Getenv("RABBITMQ_PASSWORD")
	// amqpUrl := os.Getenv("RABBITMQ_URL")

	client := db.GetRedisConnection()
	amqpUrl, err := client.Get(context.Background(), "RabbitmqUrl").Result()
	if err == redis.Nil {
		log.Fatal("RabbitmqUrl not found")
	} else if err != nil {
		log.Fatal(err)
	}

	url := "amqp://" + amqpUsername + ":" + amqpPassowd + "@" + amqpUrl
	log.Println("amqp url: ", url)

	config := rabbitmq.Config{Url: url}
	conn, err := rabbitmq.NewConnection(config)
	util.LogIfErr(err)
	defer conn.Conn.Close()

	returnChan := make(chan msg.Msg, 1000)

	mainName := "collector-main"
	mainCtrl := rabbitmq.NewCtrl(mainName, &returnChan)
	mainCtrl.SetupChannelAndQueue(mainName, conn.Conn)
	defer mainCtrl.Channel.Close()

	retryName := "collector-retry"
	retryCtrl := rabbitmq.NewCtrl(retryName, &returnChan)
	retryCtrl.SetupChannelAndQueue(retryName, conn.Conn)
	defer retryCtrl.Channel.Close()

	returnName := "collector-return"
	returnCtrl := rabbitmq.NewCtrl(returnName, &returnChan)
	returnCtrl.SetupChannelAndQueue(returnName, conn.Conn)
	defer returnCtrl.Channel.Close()

	mainCtrl.RetryChannel = retryCtrl.Channel
	mainCtrl.RetryQueue = retryCtrl.Queue

	retryCtrl.RetryChannel = retryCtrl.Channel
	retryCtrl.RetryQueue = retryCtrl.Queue

	forever := make(chan bool)

	go mainCtrl.ListenQueue()
	go retryCtrl.ListenQueue()
	go returnCtrl.ListenReturnQueue()

	<-forever
}
