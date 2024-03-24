package tcp

import (
	"errors"
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
	Logger         logger.Iface  `description:"日志接口" json:"-"`
	MessageHandler HandlerFunc   `json:"-"`
	Host           string        `description:"服务端地址" json:"host,omitempty"`
	Port           string        `description:"服务端地址" json:"port,omitempty"`
	ByteOrder      string        `description:"消息头长度字节序" json:"byteOrder,omitempty"`
	ReconnectDelay time.Duration `description:"重连的等待间隔" json:"reconnectDelay,omitempty"`
	Reconnect      bool          `description:"是否重连" json:"reconnect,omitempty"`
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
	c.r.index = -1
	c.r.addr = conn.RemoteAddr().String()

	// 处理连接时任务
	err = c.handler.OnAccepted(c.r)
	if err != nil {
		c.Logger().Errorf(
			"'%s' connected, but connection-event execute failed: %s", c.RemoteAddr(), err,
		)
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
			_ = c.r.Close()
			if !c.reconnect { // 设置了不重连，退出任务
				break
			}
			// 重连
			for c.isRunning.Load() {
				time.Sleep(c.reconnectDelay)
				err = c.connect()
				if err != nil {
					c.Logger().Errorf("'%s' reconnect failed: %s", c.RemoteAddr(), err)
				} else {
					// 重连成功，退出重连循环
					break
				}
			}
		}

		//
		// ******************** 处理过程 ********************
		//
		err = c.handler.Handler(c.r)
		if err != nil {
			c.Logger().Warn("handler failed, ", err.Error())
			continue
		} else {
			err = c.Drain() // 如果不存在数据则会直接返回nil
			if err != nil {
				c.Logger().Warnf("write message to '%s' failed, %s", c.RemoteAddr(), err.Error())
			}
		}
	}
}

// RemoteAddr 远端服务器地址
func (c *Client) RemoteAddr() string   { return c.r.addr }
func (c *Client) Logger() logger.Iface { return c.r.logger }
func (c *Client) IsRunning() bool      { return c.isRunning.Load() }

// Stop 断开并停止客户端连接
func (c *Client) Stop() error {
	err := c.r.Close()
	if err != nil {
		return err
	}

	c.isRunning.Store(false)
	return nil
}

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

// Start 异步启动客户端，如果连接失败则直接退出
func (c *Client) Start() error {
	if c.handler == nil {
		return errors.New("client MessageHandler is not define")
	}
	if c.isRunning.Load() {
		return errors.New("client is already running")
	}

	// 初始化内存
	c.r.rx = make([]byte, bufLength/4, bufLength)
	c.r.tx = make([]byte, bufLength/4, bufLength)

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
	client := &Client{
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

	if len(c) > 0 {
		client.r.addr = net.JoinHostPort(c[0].Host, c[0].Port)
		client.r.logger = c[0].Logger
		client.r.byteOrder = c[0].ByteOrder
		client.handler = c[0].MessageHandler
		client.reconnect = c[0].Reconnect
		client.reconnectDelay = c[0].ReconnectDelay
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
