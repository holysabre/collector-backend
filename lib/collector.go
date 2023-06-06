package lib

import (
	"collector-agent/device/network_switch"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/streadway/amqp"
)

const max_try_times = 5
const default_coroutine_nums = 10
const max_coroutine_nums = 30

type Msg struct {
	Type     string `json:"type"`
	Time     int64  `json:"time"`
	TryTimes int8   `json:"try_times"`
}

type Collector struct {
	MainCh  *amqp.Channel
	MainQ   amqp.Queue
	RetryCh *amqp.Channel
	RetryQ  amqp.Queue
}

func (c *Collector) Init(conn *amqp.Connection) (*amqp.Channel, *amqp.Channel) {
	c.MainCh, c.MainQ = c.getChAndQ("collector-main", conn)

	c.RetryCh, c.RetryQ = c.getChAndQ("collector-retry", conn)
	return c.MainCh, c.RetryCh
}

func handleCollect(msg Msg, body []byte) {
	log.Println("Type: ", msg.Type)
	switch msg.Type {
	case "switch":
		network_switch.Collect(body)
	}
}

func publishMsg(ch *amqp.Channel, q amqp.Queue, msg Msg) {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("无法编码为JSON格式: %v", err)
	}
	// 发布消息到队列
	err = ch.Publish(
		"",     // 交换机名称
		q.Name, // 队列名称
		false,  // 是否强制
		false,  // 是否立即发送
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(jsonData),
		},
	)
	if err != nil {
		log.Fatalf("无法发布消息: %v", err)
	}

	fmt.Println("消息已发送到队列！")
}

func (c *Collector) getChAndQ(name string, conn *amqp.Connection) (*amqp.Channel, amqp.Queue) {
	// 创建一个通道
	ch, err := conn.Channel()
	FailOnError(err, "Failed to open a channel")

	// 声明一个主队列
	q, err := ch.QueueDeclare(
		name,  // 队列名称
		false, // 是否持久化
		false, // 是否自动删除
		false, // 是否具有排他性
		false, // 是否阻塞等待
		nil,   // 额外的属性
	)
	FailOnError(err, "Failed to declare a queue")

	log.Printf("%s channel & queue declared", name)

	return ch, q
}

func (c *Collector) ListenQ(ch *amqp.Channel, q amqp.Queue) {
	// 接收消息从队列
	msgs, err := ch.Consume(
		q.Name, // 队列名称
		"",     // 消费者标签
		true,   // 是否自动回复
		false,  // 是否独占
		false,  // 是否阻塞等待
		false,  // 额外的属性
		nil,    // 消费者取消回调函数
	)
	FailOnError(err, "Failed to register a consumer")

	var wg sync.WaitGroup

	coroutine_nums := default_coroutine_nums
	if len(msgs) > default_coroutine_nums*2 {
		coroutine_nums = default_coroutine_nums * 2
		if coroutine_nums > max_coroutine_nums {
			coroutine_nums = max_coroutine_nums
		}
	}

	wg.Add(coroutine_nums)
	// 处理接收到的消息
	for d := range msgs {
		go func(d amqp.Delivery, wg *sync.WaitGroup) {
			// log.Printf("Received a message: %s", d.Body)
			var msg Msg
			err := json.Unmarshal(d.Body, &msg)
			if err != nil {
				fmt.Printf("无法解析JSON数据: %v", err)
				return
			}
			if msg.TryTimes >= max_try_times {
				fmt.Printf("%s try timeout", msg.Type)
				return
			}
			msg.TryTimes++
			if msg.Type == "" {
				publishMsg(c.RetryCh, c.RetryQ, msg)
				return
			}
			handleCollect(msg, d.Body)
			wg.Done()
		}(d, &wg)
	}
	wg.Wait()
}
