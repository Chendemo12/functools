package tcp

import (
	"errors"
	"github.com/Chendemo12/functools/logger"
	"net"
	"sync"
	"time"
)

var defaultcConfig = &TcpcConfig{
	Host:           "127.0.0.1",
	Port:           "8090",
	ByteOrder:      tcpByteOrder,
	Logger:         logger.NewDefaultLogger(),
	Reconnect:      true,
	ReconnectDelay: 2 * time.Second,
	MessageHandler: &MessageHandler{},
}

// TcpcConfig TCP客户端配置
type TcpcConfig struct {
	Logger         logger.Iface `description:"日志接口"`
	MessageHandler HandlerFunc
	Host           string        `description:"server host"`
	Port           string        `description:"server port"`
	ByteOrder      string        `description:"消息头长度字节序"`
	ReconnectDelay time.Duration `description:"重连的等待间隔"`
	Reconnect      bool          `description:"是否重连"`
}

// Client TCP 客户端
type Client struct {
	handler        HandlerFunc
	r              *Remote
	reconnectDelay time.Duration `description:"重连的等待间隔"`
	reconnect      bool          `description:"是否重连"`
	isRunning      bool
}

// RemoteAddr 远端服务器地址
func (c *Client) RemoteAddr() string   { return c.r.addr }
func (c *Client) Logger() logger.Iface { return c.r.logger }
func (c *Client) IsRunning() bool      { return c.isRunning }
func (c *Client) Stop() error          { return c.r.Close() }

// 连接远程服务
func (c *Client) connect() error {
	conn, err := net.Dial("tcp", c.RemoteAddr())
	if err != nil {
		return err
	}
	c.r.conn = conn

	// 处理连接时任务
	err = c.handler.OnAccepted(c.r)
	if err != nil {
		c.Logger().Error(c.RemoteAddr()+": error on accept, ", err.Error())
	}
	return nil
}

func (c *Client) Start() error {
	if c.isRunning {
		return errors.New("client is already running")
	}

	if c.handler == nil {
		return errors.New("client MessageHandler is not define")
	}

	// 初始化内存
	c.r.rx = make([]byte, bufLength, bufLength)
	c.r.tx = make([]byte, bufLength, bufLength)

	if err := c.connect(); err != nil {
		// 如果启动时就无法连接，则直接退出
		return err
	}

	c.isRunning = true // server is running

	for c.isRunning { // 处理通信中任务
		err := c.r.readMessage()
		if err != nil {
			// 消息读取失败，重连
			err = c.handler.OnClosed(c.r)
			if !c.reconnect { // 设置了不重连，退出任务
				break
			}
			// 重连
			time.Sleep(c.reconnectDelay)
			err = c.connect()
		}

		//
		// ******************** 处理过程 ********************
		//
		err = c.handler.Handler(c.r)
		if err != nil {
			c.Logger().Warn("handler failed, ", err.Error())
			continue
		}
	}

	return nil
}

// NewAsyncTcpClient 创建一个TCP客户端(非阻塞)
//
// 此处已实现业务处理的解耦，在使用过程中无需更改此文件源码，对于TcpClient，支持创建并启动多个客户端，
// TcpClient提供了自带缓冲的收发通道，客户端Start()之后，会将收到的数据存入接收通道TcpClient.receiverChannel，可通过迭代TCPClient.Messages()来获取收到的数据；
// 通过TcpClient.WriteMessage()来发送数据，从而实现业务的解耦。
//
// # Usage:
//
//	c := conn.NewAsyncTcpClient(&conn.TcpcConfig{
//		Host:   "127.0.0.1",
//		Port:   8090,
//		byteOrder: "big",
//		Logger: logger.ConsoleLogger{},
//	})
//
//	// 读取数据：
//	for msg := range c.Messages() {
//		fmt.Println(msg)
//	}
//
//	// 发送数据：
//	if err := c.WriteMessage([]byte("hello")); err == nil {
//		fmt.Println("<== message sent successfully")
//	}
func NewAsyncTcpClient(c ...*TcpcConfig) *Client {
	var client *Client

	if len(c) == 0 {
		client = &Client{
			r: &Remote{
				addr:      net.JoinHostPort(defaultcConfig.Host, defaultcConfig.Port),
				conn:      nil,
				logger:    defaultsConfig.Logger,
				byteOrder: defaultsConfig.ByteOrder,
				rxEnd:     headerLength,
				txEnd:     headerLength,
				lock:      &sync.Mutex{},
			},
			handler:        defaultcConfig.MessageHandler,
			reconnect:      defaultcConfig.Reconnect,
			reconnectDelay: defaultcConfig.ReconnectDelay,
			isRunning:      false,
		}
	} else {
		client = &Client{
			r: &Remote{
				addr:      net.JoinHostPort(c[0].Host, c[0].Port),
				conn:      nil,
				logger:    c[0].Logger,
				byteOrder: c[0].ByteOrder,
				rxEnd:     headerLength,
				txEnd:     headerLength,
				lock:      &sync.Mutex{},
			},
			handler:        c[0].MessageHandler,
			reconnect:      c[0].Reconnect,
			reconnectDelay: c[0].ReconnectDelay,
		}
	}
	if client.r.byteOrder == "" {
		client.r.byteOrder = tcpByteOrder
	}
	if client.r.logger == nil {
		client.r.logger = logger.NewDefaultLogger()
	}

	if client.reconnectDelay == 0 {
		client.reconnectDelay = 1 * time.Second
	}

	go func() {
		err := client.Start()
		if err != nil {
			client.r.logger.Error("connected failed: ", err.Error())
		}
	}()
	return client
}
