package main

import (
	"collector-agent/db"
	"collector-agent/models/msg"
	"collector-agent/pkg/rabbitmq"
	"collector-agent/util"
	"context"
	"log"

	"github.com/go-redis/redis/v8"
)

func main() {
	run()
}

func run() {
	client := db.GetRedisConnection()
	amqpUrl, err := client.Get(context.Background(), "RabbitmqUrl").Result()
	if err == redis.Nil {
		log.Fatal("RabbitmqUrl not found")
	} else if err != nil {
		log.Fatal(err)
	}
	slaveID, err := client.Get(context.Background(), "SlaveID").Result()
	if err == redis.Nil {
		log.Fatal("SlaveID not found")
	} else if err != nil {
		log.Fatal(err)
	}
	SSLCaCrtPem, _ := client.Get(context.Background(), "SSLCaCrtPem").Result()
	SSLClientCrtPem, _ := client.Get(context.Background(), "SSLClientCrtPem").Result()
	SSLClientKeyPem, _ := client.Get(context.Background(), "SSLClientKeyPem").Result()

	url := "amqps://guest:guest@" + amqpUrl + ":5671/"
	log.Println("amqp url: ", url)

	config := rabbitmq.Config{
		Url:             url,
		SSLCACrtPem:     SSLCaCrtPem,
		SSLClientCrtPem: SSLClientCrtPem,
		SSLClientKeyPem: SSLClientKeyPem,
	}
	// conn, err := rabbitmq.NewConnection(config)
	conn, err := rabbitmq.NewConnectionWithTLS(config)
	util.LogIfErr(err)
	defer conn.Conn.Close()

	returnChan := make(chan msg.Msg, 1000)

	mainName := "collector-main"
	mainCtrl := rabbitmq.NewCtrl(mainName, &returnChan)
	mainCtrl.SlaveID = slaveID
	mainCtrl.SetupChannelAndQueue(mainName, conn.Conn)
	defer mainCtrl.Channel.Close()

	retryName := "collector-retry"
	retryCtrl := rabbitmq.NewCtrl(retryName, &returnChan)
	retryCtrl.SlaveID = slaveID
	retryCtrl.SetupChannelAndQueue(retryName, conn.Conn)
	defer retryCtrl.Channel.Close()

	returnName := "collector-return"
	returnCtrl := rabbitmq.NewCtrl(returnName, &returnChan)
	retryCtrl.SlaveID = slaveID
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
