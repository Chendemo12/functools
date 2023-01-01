package example

import (
	"fmt"
	"github.com/Chendemo12/functools/tcp"
	"time"
)

type TCPHandler struct {
	tcp.MessageHandler
}

// HandlerFunc 处理方法，若未实现此方法，则采用默认方法TCPMessageHandler.HandlerFunc()
func (h *TCPHandler) HandlerFunc(msg *tcp.Frame) *tcp.Frame {
	fmt.Println(msg.Hex())
	return nil // 不返回任何数据
}

func Example_NewTcpServer_1() {
	s := tcp.NewAsyncTcpServer(
		&tcp.TcpsConfig{
			Host:           "0.0.0.0",
			Port:           8090,
			ByteOrder:      "big",
			MaxOpenConn:    5,
			MessageHandler: &TCPHandler{},
			Logger:         nil, // TODO：必需设置
		},
	)
	time.Sleep(100 * time.Second)
	s.Stop()
}

func Example_NewTcpClient_1() {
	c := tcp.NewAsyncTcpClient(&tcp.TcpcConfig{
		Host:      "127.0.0.1",
		Port:      8090,
		ByteOrder: "big",
		Logger:    nil, // TODO：必需设置
	})
	for msg := range c.Messages() {
		fmt.Println(msg)
	}
}
