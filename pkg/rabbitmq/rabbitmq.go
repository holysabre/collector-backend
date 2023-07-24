package rabbitmq

import (
	"bytes"
	model_msg "collector-agent/models/msg"
	model_ns "collector-agent/models/network_switch"
	model_server "collector-agent/models/server"
	model_system "collector-agent/models/system"
	"collector-agent/pkg/network_switch"
	"collector-agent/pkg/server"
	"collector-agent/pkg/system"
	"collector-agent/util"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/streadway/amqp"
)

const (
	PoolCap                int32 = 100
	max_try_times          int8  = 5
	default_coroutine_nums int   = 10
	max_coroutine_nums     int   = 30
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
	util.FailOnError(err, "Failed to connect to RabbitMQ")
	conn.Conn = amqpConn
	return conn, nil
}

func NewConnectionWithTLS(config Config) (Connection, error) {

	// fmt.Println(config.SSLClientCrtPem, config.SSLClientKeyPem, config.SSLCACrtPem)

	// cert, err := tls.LoadX509KeyPair(config.SSLClientCrtPem, config.SSLClientKeyPem)
	cert, err := tls.X509KeyPair([]byte(config.SSLClientCrtPem), []byte(config.SSLClientKeyPem))
	if err != nil {
		log.Fatalf("Failed to load X509 key pair: %v", err)
	}

	// caCert, err := ioutil.ReadFile(config.SSLCACrtPem)
	// if err != nil {
	// 	log.Fatalf("Failed to read CA certificate: %v", err)
	// }

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(config.SSLCACrtPem))

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
	}

	conn := Connection{}
	amqpConn, err := amqp.DialTLS(config.Url, tlsConfig)
	util.FailOnError(err, "Failed to connect to RabbitMQ")
	conn.Conn = amqpConn
	return conn, nil
}

type Controller struct {
	Channel      *amqp.Channel
	Queue        amqp.Queue
	Pool         gopool.Pool
	RetryChannel *amqp.Channel
	RetryQueue   amqp.Queue
	ReturnChann  *chan model_msg.Msg
	SlaveID      string
}

func NewCtrl(poolName string, returnChan *chan model_msg.Msg) *Controller {
	return &Controller{
		Pool:        gopool.NewPool(poolName, PoolCap, gopool.NewConfig()),
		ReturnChann: returnChan,
	}
}

func (ctrl *Controller) SetupChannelAndQueue(name string, amqpConn *amqp.Connection) error {
	ch, err := amqpConn.Channel()
	util.FailOnError(err, "Failed to open a channel")

	q, err := ch.QueueDeclare(
		name,  // 队列名称
		false, // 是否持久化
		true,  // 是否自动删除
		false, // 是否具有排他性
		false, // 是否阻塞等待
		nil,   // 额外的属性
	)
	util.FailOnError(err, "Failed to declare a queue")

	// err = ch.QueueBind(name, "agent", "dcim-collector", false, nil)
	// util.FailOnError(err, "Failed to bind queue")

	log.Printf("%s channel & queue declared", name)

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
		true,            // 是否独占
		true,            // 是否阻塞等待
		false,           // 额外的属性
		nil,             // 消费者取消回调函数
	)
	util.FailOnError(err, "Failed to register a consumer")

	for d := range msgs {
		var msg model_msg.Msg
		splited := bytes.Split(d.Body, []byte{'|'})
		if len(splited) < 2 {
			return
		}
		decodedBytes, err := base64.StdEncoding.DecodeString(string(splited[1]))
		if err != nil {
			fmt.Printf("Unable to decode base64 data: %v", err)
			return
		}
		// decryptedBody, err := crypt_util.New().DecryptViaPub(decodedBytes)
		// if err != nil {
		// 	fmt.Printf("Unable to decrypt data: %v", err)
		// 	return
		// }
		err = json.Unmarshal(decodedBytes, &msg)
		if err != nil {
			fmt.Printf("Unable to parse json data: %v", err)
			return
		}
		if msg.TryTimes >= max_try_times {
			fmt.Printf("%s try timeout", msg.Type)
			return
		}
		msg.TryTimes++
		if msg.Type == "" {
			if err := ctrl.publishMsg(ctrl.RetryChannel, ctrl.RetryQueue, msg); err != nil {
				return
			}
		}
		ctrl.Pool.Go(func() {
			ctrl.handleCollect(msg)
		})
	}
}

func (ctrl *Controller) handleCollect(msg model_msg.Msg) {
	// log.Println("Type: ", msg.Type)
	body := []byte(msg.Data)
	switch msg.Type {
	case "switch":
		var ns model_ns.NetworkSwitch
		err := json.Unmarshal(body, &ns)
		if err != nil {
			fmt.Printf("NetworkSwitch 无法解析JSON数据: %v", err)
			return
		}
		nsc := network_switch.NewNSCollector(&ns)
		nsc.Collect()
		jsonData, err := json.Marshal(nsc.NetworkSwitch)
		if err != nil {
			fmt.Printf("无法编码为JSON格式: %v", err)
		}
		returnMsg := model_msg.Msg{Type: "switch", Time: time.Now().Unix(), Data: string(jsonData)}
		*ctrl.ReturnChann <- returnMsg
	case "server":
		var s model_server.Server
		err := json.Unmarshal(body, &s)
		if err != nil {
			fmt.Printf("NetworkSwitch 无法解析JSON数据: %v", err)
			return
		}
		sc := server.NewServerCollector(&s)
		sc.Collect()
		// fmt.Println(sc.Server)
		jsonData, err := json.Marshal(sc.Server)
		if err != nil {
			fmt.Printf("无法编码为JSON格式: %v", err)
		}
		returnMsg := model_msg.Msg{Type: "server", Time: time.Now().Unix(), Data: string(jsonData)}
		*ctrl.ReturnChann <- returnMsg
	case "system":
		var s model_system.SystemInfo
		err := json.Unmarshal(body, &s)
		if err != nil {
			fmt.Printf("NetworkSwitch 无法解析JSON数据: %v", err)
			return
		}
		sc := system.NewSystemCollector(&s)
		sc.Collect()
		jsonData, err := json.Marshal(sc.SystemInfo)
		if err != nil {
			fmt.Printf("无法编码为JSON格式: %v", err)
		}
		returnMsg := model_msg.Msg{Type: "system", Time: time.Now().Unix(), Data: string(jsonData)}
		*ctrl.ReturnChann <- returnMsg
	}
}

func (ctrl *Controller) ListenReturnQueue() {
	for {
		if len(*ctrl.ReturnChann) == 0 {
			time.Sleep(100 * time.Microsecond)
			continue
		}
		if err := ctrl.publishMsg(ctrl.Channel, ctrl.Queue, <-*ctrl.ReturnChann); err != nil {
			fmt.Println("Fail to publish, err: ", err)
		}
	}
}

func (ctrl *Controller) publishMsg(ch *amqp.Channel, q amqp.Queue, msg model_msg.Msg) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Cannot be encoded in json format: %v", err)
		return err
	}
	// encryptedMsg, err := crypt_util.New().EncryptViaPub(jsonData)
	// if err != nil {
	// 	fmt.Printf("Cannot encrypted data: %v", err)
	// 	return err
	// }
	encodedMsg := base64.StdEncoding.EncodeToString(jsonData)
	// 发布消息到队列
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
	if err != nil {
		return err
	}

	// fmt.Println("Msg Published")
	return nil
}
