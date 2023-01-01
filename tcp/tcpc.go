package tcp

import (
	"errors"
	"io"
	"net"
	"strconv"
)

var defaultcConfig = &TcpcConfig{
	Host:      "127.0.0.1",
	Port:      8090,
	ByteOrder: tcpByteOrder,
	Logger:    nil,
}

// TcpcConfig TCP客户端配置
type TcpcConfig struct {
	Host      string
	Port      int
	ByteOrder string
	Logger    LoggerIface
}

// Client TCP 客户端
type Client struct {
	logger          LoggerIface
	client          net.Conn
	receiverChannel chan *Frame // 从TCP收到的数据
	senderChannel   chan *Frame // 需要发送到TCP的数据
	Host            string
	Port            int
	ByteOrder       string
	isRunning       bool
}

// RemoteAddr 远端服务器地址
func (c *Client) RemoteAddr() string {
	if c.isRunning {
		return c.client.RemoteAddr().String()
	} else {
		return ""
	}
}

func (c *Client) Start() error {
	if c.isRunning {
		return errors.New("client is already running")
	}

	if c.ByteOrder == "" {
		c.ByteOrder = tcpByteOrder
	}

	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	// 实例化通道
	c.receiverChannel = make(chan *Frame, 20)
	c.senderChannel = make(chan *Frame, 20)
	// 记录连接
	c.client = conn

	msg := &Frame{Conn: conn, ByteOrder: c.ByteOrder}
	go func() { // 发送消息
		for msg := range c.senderChannel {
			_, err := conn.Write(msg.Pack())
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF { // connection closed by server
					break
				} else {
					c.logger.Warn("send message failed, ", err.Error(), c.RemoteAddr())
				}
			}
		}
	}()

	go func() {
		defer func(conn net.Conn) {
			if err := conn.Close(); err != nil {
				c.logger.Warn("conn close failed: ", err.Error())
				return
			}
		}(conn)

		for {
			// 接收数据
			headerBuf := [2]byte{} // 消息头长2个字节
			// 接收数据
			if n, err := conn.Read(headerBuf[:]); err != nil {
				if err != io.EOF {
					c.logger.Warn("read from connect failed, ", err.Error())
				}
				break
			} else {
				if n < 2 {
					c.logger.Warn("the message header is incomplete, ", c.RemoteAddr())
					continue
				}
				// 获取消息长度
				msg.Length = MessageHeaderUnpack(c.ByteOrder, headerBuf[:])
			}

			contentBuf := make([]byte, msg.Length)
			if _, err := io.ReadFull(conn, contentBuf); err != nil {
				if err != io.EOF {
					c.logger.Warn("read from connect failed, ", err.Error(), c.RemoteAddr())
				}
				break
			} // ReadFull 会把填充contentBuf填满为止

			msg.Content = contentBuf // 获取消息体
			c.receiverChannel <- msg
		}
	}()

	c.isRunning = true
	return nil
}

func (c *Client) Stop() {
	if c != nil && c.client != nil {
		if err := c.client.Close(); err != nil {
			c.logger.Warn("conn close failed: ", err.Error())
			return
		}
	}
}

// Messages 获取收到的数据
// # Usage：
//
//	for msg := range c.Messages() {
//		fmt.Println(msg.String())
//	}
func (c *Client) Messages() <-chan *Frame { return c.receiverChannel }

func (c *Client) WriteMessage(data []byte) error {
	if c.isRunning {
		msg := &Frame{Content: data, ByteOrder: c.ByteOrder}
		c.senderChannel <- msg
		return nil
	}
	return errors.New("client is not running")
}

// NewAsyncTcpClient 创建一个TCP客户端(非阻塞)
//
// 此处已实现业务处理的解耦，在使用过程中无需更改此文件源码，对于TcpClient，支持创建并启动多个客户端，
// TcpClient提供了自带缓冲的收发通道，客户端Start()之后，会将收到的数据存入接收通道TcpClient.receiverChannel，可通过迭代TCPClient.Messages()来获取收到的数据；
// 通过TcpClient.WriteMessage()来发送数据，从而实现业务的解耦。
//
// # Usage:
//
//	c := client.NewAsyncTcpClient(&client.TcpcConfig{
//		Host:   "127.0.0.1",
//		Port:   8090,
//		ByteOrder: "big",
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
	var tc *Client

	if len(c) == 0 {
		tc = &Client{
			Host:      defaultcConfig.Host,
			Port:      defaultcConfig.Port,
			ByteOrder: defaultcConfig.ByteOrder,
			logger:    nil,
			isRunning: false,
		}
	} else {
		tc = &Client{
			Host:      c[0].Host,
			Port:      c[0].Port,
			ByteOrder: c[0].ByteOrder,
			logger:    c[0].Logger,
		}
	}

	go func() {
		if err := tc.Start(); err != nil {
			tc.logger.Error("connected failed: ", err.Error())
		}
	}()
	return tc
}
