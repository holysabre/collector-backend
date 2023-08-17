package main

import (
	"collector-agent/db"
	"collector-agent/models/msg"
	"collector-agent/pkg/logger"
	"collector-agent/pkg/rabbitmq"
	"context"

	"github.com/go-redis/redis/v8"
)

func main() {
	run()
}

func run() {
	redisConn := db.NewRedisConnection()
	client := redisConn.GetClient()
	amqpUrl, err := client.Get(context.Background(), "RabbitmqUrl").Result()
	logger.ExitIfErr(err, "Fail To Get Data From Redis")
	if err == redis.Nil {
		logger.Fatal("RabbitmqUrl not found")
	}
	slaveID, err := client.Get(context.Background(), "SlaveID").Result()
	logger.ExitIfErr(err, "Fail To Get Data From Redis")
	if err == redis.Nil {
		logger.Fatal("SlaveID not found")
	}
	datacenterID, err := client.Get(context.Background(), "DatacenterID").Result()
	logger.ExitIfErr(err, "Fail To Get Data From Redis")
	if err == redis.Nil {
		logger.Fatal("DatacenterID not found")
	}
	SSLCaCrtPem, err := client.Get(context.Background(), "SSLCaCrtPem").Result()
	logger.ExitIfErr(err, "Fail To Get Data From Redis")
	if err == redis.Nil {
		logger.Fatal("SSLCaCrtPem not found")
	}
	SSLClientCrtPem, _ := client.Get(context.Background(), "SSLClientCrtPem").Result()
	SSLClientKeyPem, _ := client.Get(context.Background(), "SSLClientKeyPem").Result()

	redisConn.CloseClient(client)

	url := "amqps://guest:guest@" + amqpUrl + ":5671/"
	logger.Printf("amqp url: %s", url)

	config := rabbitmq.Config{
		Url:             url,
		SSLCACrtPem:     SSLCaCrtPem,
		SSLClientCrtPem: SSLClientCrtPem,
		SSLClientKeyPem: SSLClientKeyPem,
	}
	// conn, err := rabbitmq.NewConnection(config)
	conn, err := rabbitmq.NewConnectionWithTLS(config)
	logger.LogIfErr(err)
	defer conn.Conn.Close()

	returnChan := make(chan msg.Msg, 1000)

	mainName := "collector-main-" + datacenterID
	mainCtrl := rabbitmq.NewCtrl(mainName, &returnChan)
	mainCtrl.SlaveID = slaveID
	mainCtrl.SetupChannelAndQueue(mainName, conn.Conn)
	defer mainCtrl.Channel.Close()

	retryName := "collector-retry-" + datacenterID
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
