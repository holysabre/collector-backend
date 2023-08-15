package rabbitmq

import (
	"bytes"
	model_msg "collector-agent/models/msg"
	model_ns "collector-agent/models/network_switch"
	model_server "collector-agent/models/server"
	model_system "collector-agent/models/system"
	"collector-agent/pkg/logger"
	"collector-agent/pkg/network_switch"
	"collector-agent/pkg/server"
	"collector-agent/pkg/system"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"runtime"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/streadway/amqp"
)

const (
	PoolCapPreCoreNum      int  = 2
	max_try_times          int8 = 3
	default_coroutine_nums int  = 10
	max_coroutine_nums     int  = 30
)

type Connection struct {
	Config Config
	Conn   *amqp.Connection
}
type Config struct {
	Url             string
	SSLCACrtPem     string
	SSLClientCrtPem string
	SSLClientKeyPem string
}

func NewConnection(config Config) (Connection, error) {
	conn := Connection{}
	amqpConn, err := amqp.Dial(config.Url)
	logger.ExitIfErr(err, "Failed to connect to RabbitMQ")
	conn.Conn = amqpConn
	return conn, nil
}

func NewConnectionWithTLS(config Config) (Connection, error) {
	cert, err := tls.X509KeyPair([]byte(config.SSLClientCrtPem), []byte(config.SSLClientKeyPem))
	logger.ExitIfErr(err, "Failed to load X509 key pair")

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(config.SSLCACrtPem))

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
	}

	conn := Connection{}
	amqpConn, err := amqp.DialTLS(config.Url, tlsConfig)
	logger.ExitIfErr(err, "Failed to connect to RabbitMQ")
	conn.Conn = amqpConn
	return conn, nil
}

type Controller struct {
	Channel      *amqp.Channel
	Queue        amqp.Queue
	Pool         gopool.Pool
	ServerPool   gopool.Pool
	NSPool       gopool.Pool
	RetryChannel *amqp.Channel
	RetryQueue   amqp.Queue
	ReturnChann  *chan model_msg.Msg
	SlaveID      string
}

func NewCtrl(poolName string, returnChan *chan model_msg.Msg) *Controller {
	numCPU := runtime.NumCPU()
	poolCap := int32(PoolCapPreCoreNum * numCPU)
	logger.Printf("Number of CPU cores: %d, poolCap: %d\n", numCPU, poolCap)
	return &Controller{
		Pool:        gopool.NewPool(poolName, poolCap, gopool.NewConfig()),
		ServerPool:  gopool.NewPool(poolName+"-server", poolCap, gopool.NewConfig()),
		NSPool:      gopool.NewPool(poolName+"-ns", poolCap, gopool.NewConfig()),
		ReturnChann: returnChan,
	}
}

func (ctrl *Controller) SetupChannelAndQueue(name string, amqpConn *amqp.Connection) error {
	ch, err := amqpConn.Channel()
	logger.ExitIfErr(err, "Failed to open a channel")

	q, err := ch.QueueDeclare(
		name,  // 队列名称
		false, // 是否持久化
		true,  // 是否自动删除
		false, // 是否具有排他性
		false, // 是否阻塞等待
		nil,   // 额外的属性
	)
	logger.ExitIfErr(err, "Failed to declare a queue")
	logger.Printf("%s channel & queue declared", name)

	ctrl.Channel = ch
	ctrl.Queue = q

	return nil
}

func (ctrl *Controller) ListenQueue() {
	consumeTag := "slave-" + ctrl.SlaveID
	// 接收消息从队列
	msgs, err := ctrl.Channel.Consume(
		ctrl.Queue.Name, // 队列名称
		consumeTag,      // 消费者标签
		true,            // 是否自动回复
		false,           // 是否独占
		false,           // 是否阻塞等待
		false,           // 额外的属性
		nil,             // 消费者取消回调函数
	)
	logger.ExitIfErr(err, "Failed to register a consumer")

	for d := range msgs {
		var msg model_msg.Msg
		splited := bytes.Split(d.Body, []byte{'|'})
		if len(splited) < 2 {
			continue
		}
		decodedBytes, err := base64.StdEncoding.DecodeString(string(splited[1]))
		if err != nil {
			logger.Printf("Unable to decode base64 data: %v", err.Error())
			continue
		}
		err = json.Unmarshal(decodedBytes, &msg)
		if err != nil {
			logger.Printf("Unable to parse json data: %v", err.Error())
			continue
		}
		// if msg.TryTimes >= max_try_times {
		// 	logger.Printf("%s try timeout", msg.Type)
		// 	continue
		// }
		// msg.TryTimes++
		// if msg.Type == "" {
		// 	if err := ctrl.publishMsg(ctrl.RetryChannel, ctrl.RetryQueue, msg); err != nil {
		// 		continue
		// 	}
		// }
		ctrl.handleCollect(msg)
	}
}

func (ctrl *Controller) handleCollect(msg model_msg.Msg) {
	// log.Println("Type: ", msg.Type)
	body := []byte(msg.Data)
	switch msg.Type {
	case "switch":
		ctrl.NSPool.Go(func() {
			var ns model_ns.NetworkSwitch
			err := json.Unmarshal(body, &ns)
			logger.LogIfErrWithMsg(err, "NetworkSwitch Unable To Parse JSON Data")
			nsc := network_switch.NewNSCollector(&ns)
			nsc.Collect()
			jsonData, err := json.Marshal(nsc.NetworkSwitch)
			logger.LogIfErrWithMsg(err, "Cannot Be Encoded In JSON Format")
			returnMsg := model_msg.Msg{Type: "switch", Time: time.Now().Unix(), Data: string(jsonData)}
			*ctrl.ReturnChann <- returnMsg
		})
	case "server":
		ctrl.ServerPool.Go(func() {
			var s model_server.Server
			err := json.Unmarshal(body, &s)
			logger.LogIfErrWithMsg(err, "Server Unable To Parse JSON Data")
			sc := server.NewServerCollector(&s)
			sc.Collect()
			jsonData, err := json.Marshal(sc.Server)
			logger.LogIfErrWithMsg(err, "Cannot Be Encoded In JSON Format")
			returnMsg := model_msg.Msg{Type: "server", Time: time.Now().Unix(), Data: string(jsonData)}
			*ctrl.ReturnChann <- returnMsg
		})
	case "system":
		ctrl.Pool.Go(func() {
			var s model_system.SystemInfo
			err := json.Unmarshal(body, &s)
			logger.LogIfErrWithMsg(err, "Systen Unable To Parse JSON Data")
			sc := system.NewSystemCollector(&s)
			sc.Collect()
			jsonData, err := json.Marshal(sc.SystemInfo)
			logger.LogIfErrWithMsg(err, "Cannot Be Encoded In JSON Format")
			returnMsg := model_msg.Msg{Type: "system", Time: time.Now().Unix(), Data: string(jsonData)}
			*ctrl.ReturnChann <- returnMsg
		})
	}
}

func (ctrl *Controller) ListenReturnQueue() {
	for {
		if len(*ctrl.ReturnChann) == 0 {
			time.Sleep(100 * time.Microsecond)
			continue
		}
		if err := ctrl.publishMsg(ctrl.Channel, ctrl.Queue, <-*ctrl.ReturnChann); err != nil {
			return
		}
	}
}

func (ctrl *Controller) publishMsg(ch *amqp.Channel, q amqp.Queue, msg model_msg.Msg) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		logger.Printf("Cannot be encoded in json format: %v", err.Error())
		return nil
	}
	encodedMsg := base64.StdEncoding.EncodeToString(jsonData)
	err = ch.Publish(
		"",     // 交换机名称
		q.Name, // 队列名称
		false,  // 是否强制
		false,  // 是否立即发送
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(encodedMsg),
		},
	)
	logger.ExitIfErr(err, "Fail To Publish Msg")

	// logger.Println("Msg Published")
	return nil
}
