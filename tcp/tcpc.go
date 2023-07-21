package tcp

import (
	"errors"
	"fmt"
	"github.com/Chendemo12/functools/logger"
	"io"
	"net"
	"sync"
	"sync/atomic"
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
	isRunning      *atomic.Bool
}

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
		c.Logger().Error(fmt.Sprintf(
			"'%s' connected, but connection-event execute failed: %s", c.RemoteAddr(), err,
		))
	}
	return nil
}

// 阻塞处理通信中任务
func (c *Client) proc() {
	for c.isRunning.Load() {
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
}

// RemoteAddr 远端服务器地址
func (c *Client) RemoteAddr() string   { return c.r.addr }
func (c *Client) Logger() logger.Iface { return c.r.logger }
func (c *Client) IsRunning() bool      { return c.isRunning.Load() }
func (c *Client) Stop() error          { return c.r.Close() }

func (c *Client) Write(buf []byte) (int, error) { return c.r.Write(buf) }

// WriteFrom 从一个reader中读取数据并写入缓冲区
// 返回写入tcp缓冲区的字节数，而非从reader中读取的字节数
// 仅在未从 reader 中读取到数据时返回错误
func (c *Client) WriteFrom(reader io.Reader) (int, error) {
	buf := make([]byte, c.r.TxFreeSize()) // 创建内存

	i, err := reader.Read(buf)
	if err != nil && i == 0 { // 未读取到数据
		return i, err
	}
	// reader 数据太多，而写入缓冲区目前没有足够空间供其写入，正常
	// err != nil && i != 0

	return c.r.Write(buf[:i])
}

func (c *Client) Drain() error { return c.r.Drain() }

// WriteMessage 一次性写入并发送数据
func (c *Client) WriteMessage(buf []byte) error {
	_, err := c.r.Write(buf)
	if err != nil {
		return err
	}
	return c.r.Drain()
}

// Start 启动客户端，如果连接失败则直接退出
func (c *Client) Start() error { return c.AsyncStart() }

// AsyncStart 异步启动客户端，如果连接失败则直接退出
func (c *Client) AsyncStart() error {
	if c.isRunning.Load() {
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

	c.isRunning.Store(true) // service is running

	go c.proc() // 异步后台运行

	return nil
}

// NewTcpClient 创建同步客户端，此处未主动连接服务端，需手动 Client.Start 发起连接
func NewTcpClient(c ...*TcpcConfig) *Client {
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
			isRunning:      &atomic.Bool{},
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
			isRunning:      &atomic.Bool{},
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

	return client
}

// NewAsyncTcpClient 创建一个TCP客户端(非阻塞), 此处已启动数据的收发操作
//
// 此处已实现业务处理的解耦，在使用过程中无需更改此文件源码，对于TcpClient，支持创建并启动多个客户端，
// 其重点在于实现 HandlerFunc 接口
// 客户端除在连接成功时通过 Remote.Write 发送数据外,同样可以通过 Client.WriteMessage 来发送数据;
func NewAsyncTcpClient(c ...*TcpcConfig) *Client {
	client := NewTcpClient(c...)

	err := client.Start()
	if err != nil {
		client.r.logger.Error("connected failed: ", err.Error())
	}

	return client
}
