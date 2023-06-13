package main

import (
	"collector-agent/models/msg"
	"collector-agent/pkg/rabbitmq"
	"collector-agent/pkg/server"
	"collector-agent/util"
)

func main() {
	sc := server.NewServerCollector()
	sc.Collect()
}

func run() {
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
