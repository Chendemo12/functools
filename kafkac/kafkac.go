// Package kafkac kafka客户端，包含生产者和消费者
package kafkac

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/Shopify/sarama"
)

var wg sync.WaitGroup

// LoggerIface 自定义logger接口，log及zap等均已实现此接口
type LoggerIface interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Sync() error
}

type KafkaConfig struct {
	Logger              LoggerIface                       `json:"-"`                     // 日志
	ConsumerHandler     func(msg *sarama.ConsumerMessage) `json:"-"`                     // 消费者处理程序
	GroupId             string                            `json:"group_id"`              // 消费组ID
	Ack                 string                            `json:"ack"`                   // ACK应答机制, 0, 1, all
	Addrs               []string                          `json:"addrs"`                 // 地址组
	Topics              []string                          `json:"topics"`                // 消费者主题,仅当此不为空时才创建消费者协程
	ProducerErrorSwitch bool                              `json:"producer_error_switch"` // 是否查看生产者错误消息
}

// consumerHandler 消费组处理程序
type consumerHandler struct {
	MessageHandler func(msg *sarama.ConsumerMessage)
}

func (h consumerHandler) Setup(_ sarama.ConsumerGroupSession) error { return nil }

func (h consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h consumerHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.MessageHandler(msg)
		sess.MarkMessage(msg, "")
	}
	return nil
}

// KafkaClient kafka连接客户端，此客户端提供一个带缓冲的消息通道
//
// # Usage:
//
//	// 从方法创建
//
//	conf = &KafkaConfig{
//		Addrs: []string{"10.64.5.70:30095", "10.64.5.70:30094"},
//		Topics: []string{}
//		GroupId: "KONSOLE", Ack: "all",
//		ProducerErrorSwitch: true,
//		ConsumerHandler: handler,
//	}
//	kc := NewDefaultKafka(conf)
//
//	// 实例化创建
//
//	kc2 := client.KafkaClient{
//		Addrs:   []string{"10.64.5.70:30095", "10.64.5.70:30094"},
//		GroupId: "FLYING",
//		Ack: "all"
//	}
//	kc2.SetLogger(logger.ConsoleLogger{})
type KafkaClient struct {
	logger              LoggerIface                       // logger
	producerError       chan *sarama.ProducerError        // 生产者错误消息，仅在开启了错误开关时有效
	signals             chan os.Signal                    // 监听程序关闭信号
	config              *sarama.Config                    // 配置参数
	senderChannel       chan *sarama.ProducerMessage      // 需要发送到Kafka的数据
	consumerHandler     func(msg *sarama.ConsumerMessage) // 消费者处理程序
	Ack                 string                            // ACK应答机制, 0, 1, all
	GroupId             string                            // 消费组ID
	Addrs               []string                          // 地址组
	buffSize            uint                              // 通道缓冲大小
	successes           uint64                            // 成功统计计数
	producerErrors      uint64                            // 错误统计计数
	producerErrorSwitch bool                              // 是否查看生产者错误消息
	inited              bool                              // 是否完成初始化
}

func (k *KafkaClient) makeProducerMessage(topic string, key, value []byte) *sarama.ProducerMessage {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
		Key:   sarama.ByteEncoder(key),
	}
	return msg
}

// initConfig 初始化配置参数
// 参考: https://pkg.go.dev/github.com/Shopify/sarama#example-AsyncProducer-Goroutines
func (k *KafkaClient) initConfig() {
	if k.buffSize == 0 { // 若未配置缓冲区大小，则采用全局默认大小
		k.buffSize = 20
	}

	if k.producerErrorSwitch {
		k.producerError = make(chan *sarama.ProducerError, k.buffSize)
	}

	k.config = sarama.NewConfig()
	// 初始化生产者参数
	k.senderChannel = make(chan *sarama.ProducerMessage, k.buffSize)
	// 设置ACK应答机制
	switch k.Ack {
	case "0":
		k.config.Producer.RequiredAcks = sarama.NoResponse
	case "1":
		k.config.Producer.RequiredAcks = sarama.WaitForLocal
	default:
		k.config.Producer.RequiredAcks = sarama.WaitForAll
	}

	k.config.Producer.Partitioner = sarama.NewRandomPartitioner // 发送消息到一个随机的分区
	k.config.Producer.Return.Successes = true                   // 等待成功响应
	k.config.Producer.Return.Errors = k.producerErrorSwitch     // 是否开启错误消息开关
	k.config.Producer.Timeout = 5 * time.Second                 // 生产者超时时间

	// 初始化消费者参数
	k.config.Consumer.Offsets.Initial = sarama.OffsetNewest // 初始从最新的offset开始
	k.config.Consumer.Return.Errors = false

	//
	k.signals = make(chan os.Signal, 1)
	signal.Notify(k.signals, os.Interrupt)
	k.inited = true
}

// GetSuccessCounts 获取成功发送的消息计数
func (k *KafkaClient) GetSuccessCounts() uint64 { return k.successes }

// GetErrorCounts 获取发送失败的消息计数
func (k *KafkaClient) GetErrorCounts() uint64 { return k.producerErrors }

// SetLogger 修改内部logger
// 须在消费者或生产者启动前进行修改，若未配置，则默认采用全局的zap.SugaredLogger
func (k *KafkaClient) SetLogger(logger LoggerIface) *KafkaClient {
	k.logger = logger
	return k
}

// SetBuffSize 设置收发通道的缓冲大小，若通道已满则会阻塞
func (k *KafkaClient) SetBuffSize(size uint) *KafkaClient {
	k.buffSize = size
	return k
}

// OpenProducerErrors 开启生产者错误消息, 生产者启动前开启有效
func (k *KafkaClient) OpenProducerErrors() *KafkaClient {
	k.producerErrorSwitch = true
	return k
}

// YieldMessage 生产一个Kafka消息对象，此方法会创建一个协程任务并立刻返回
// @param  topic  string  目标主题
// @param  key    []byte  键
// @param  value  []byte  值
func (k *KafkaClient) YieldMessage(topic string, key, value []byte) {
	go func() {
		k.senderChannel <- k.makeProducerMessage(topic, key, value)
	}()
}

// SendMessage 生产一个Kafka消息对象，若通道已满，则会阻塞
// @param  topic  string  目标主题
// @param  key    []byte  键
// @param  value  []byte  值
func (k *KafkaClient) SendMessage(topic string, key, value []byte) {
	k.senderChannel <- k.makeProducerMessage(topic, key, value)
}

// NewAsyncConsumer 启动一个消费者任务，此任务必须以goroutine形式运行，否则会阻塞
// @param   topic    []string                           消费者Topics
// @param   handler  func(msg *sarama.ConsumerMessage)  消息处理函数
// @return  error 启动失败则返回错误原因，阻塞则成功
func (k *KafkaClient) NewAsyncConsumer(topics []string, handler func(msg *sarama.ConsumerMessage)) error {
	if !k.inited {
		k.initConfig()
	}
	k.consumerHandler = handler
	// 创建消费者组
	group, err := sarama.NewConsumerGroup(k.Addrs, k.GroupId, k.config)
	if err != nil {
		return err
	}
	defer func() { _ = group.Close() }()

	ctx := context.Background()
	for {
		if err := group.Consume(ctx, topics, consumerHandler{MessageHandler: handler}); err != nil {
			return err
		}
	}
}

// NewAsyncProducer 启动一个异步生产者任务，此任务必须以goroutine形式运行，否则会阻塞
// @return  error 启动失败则返回错误原因，成功则未nil
func (k *KafkaClient) NewAsyncProducer() error {
	if !k.inited {
		k.initConfig()
	}

	producer, err := sarama.NewAsyncProducer(k.Addrs, k.config)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range producer.Successes() {
			k.successes++
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range producer.Errors() {
			k.producerErrors++
		}
	}()

ProducerLoop:
	for message := range k.senderChannel {
		select {
		case producer.Input() <- message:

		case <-k.signals:
			producer.AsyncClose() // Trigger a shutdown of the producer
			close(k.producerError)
			close(k.senderChannel)
			break ProducerLoop
		}
	}

	wg.Wait()
	return nil
}

// ProducerErrors 获取生产者错误消息
//
//	for msg := range kc.ProducerErrors() {
//		fmt.Println(msg)
//	}
func (k *KafkaClient) ProducerErrors() <-chan *sarama.ProducerError { return k.producerError }

// Deprecated: NewProducer 创建生产者任务, 移步至NewAsyncProducer()方法
func (k *KafkaClient) NewProducer() error {
	return k.NewAsyncProducer()
}

// Deprecated: SendProducerMessage 移步至SendMessage()方法
func (k *KafkaClient) SendProducerMessage(topic string, key, value []byte) {
	k.SendMessage(topic, key, value)
}

// Deprecated: YieldProducerMessage 移步至YieldMessage()方法
func (k *KafkaClient) YieldProducerMessage(topic string, key, value []byte) {
	k.YieldMessage(topic, key, value)
}

// NewAsyncKafka 创建一个Kafka服务, 启动生产者协程，若Topics不为空则启动消费者协程
// @return  *KafkaClient kafka客户端
func NewAsyncKafka(conf *KafkaConfig) *KafkaClient {
	kc := &KafkaClient{
		Addrs:    conf.Addrs,
		GroupId:  conf.GroupId,
		logger:   conf.Logger,
		buffSize: 20,
	}
	if len(conf.Topics) != 0 {
		go func() {
			err := kc.NewAsyncConsumer(conf.Topics, conf.ConsumerHandler)
			if err != nil {
				conf.Logger.Error("Consumer started failed: " + err.Error())
			}
		}()
	}

	go func() {
		err := kc.NewAsyncProducer()
		if err != nil {
			conf.Logger.Error("Producer started failed: " + err.Error())
		} else {
			conf.Logger.Info("Producer started")
		}
	}()
	return kc
}
