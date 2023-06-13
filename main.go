package main

import (
	"collector-agent/models/msg"
	"collector-agent/pkg/rabbitmq"
	"collector-agent/util"
)

func main() {
	config := rabbitmq.Config{Url: "amqp://root:password@192.168.88.112:5672/"}
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

// func run() {
// 	// 连接到RabbitMQ服务器
// 	conn, err := amqp.Dial("amqp://root:password@192.168.88.112:5672/")
// 	util.FailOnError(err, "Failed to connect to RabbitMQ")
// 	defer conn.Close()

// 	var collector lib.Collector

// 	mainCh, retryCh, returnCh := collector.Init(conn)
// 	defer mainCh.Close()
// 	defer retryCh.Close()
// 	defer returnCh.Close()

// 	collector.PublishChan = make(chan lib.Msg)

// 	// 处理接收到的消息
// 	forever := make(chan bool)

// 	go collector.ListenQ(collector.MainCh, collector.MainQ)

// 	go collector.ListenQ(collector.RetryCh, collector.RetryQ)

// 	go collector.ListenPublishQ()

// 	log.Printf(" [*] Waiting for messages. To exit, press CTRL+C")
// 	<-forever
// }
