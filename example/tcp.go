package example

import (
	"github.com/Chendemo12/functools/tcp"
	"github.com/Chendemo12/functools/zaplog"
	"sync"
	"time"
)

var (
	//logger = zaplog.NewLogger(&zaplog.Config{
	//	Filename:   "example",
	//	Level:      zaplog.DEBUG,
	//	Rotation:   2,
	//	Retention:  3,
	//	MaxBackups: 4,
	//	Compress:   true,
	//}).Sugar()
	logger = zaplog.ConsoleLogger{}
)

type ServerHandler struct {
	tcp.MessageHandler
}

func (h *ServerHandler) OnAccepted(r *tcp.Remote) error {
	r.Logger().Info("Accepted client: ", r.String())
	return nil
}

func (h *ServerHandler) Handler(r *tcp.Remote) error {
	r.Logger().Info("receive message from: ", r.String())
	// Example 1
	//return nil

	// Example 2
	_, err := r.Write([]byte("message received"))
	err = r.Drain()
	return err

}

type ClientHandler struct {
	tcp.MessageHandler
}

func (h *ClientHandler) OnAccepted(r *tcp.Remote) error {
	r.Logger().Info("connect success")

	// Example 2
	//go func() {
	//	for r.IsConnected() {
	//		_, err := r.Write([]byte(time.Now().String()))
	//		err = r.Drain()
	//		if err != nil {
	//			return
	//		}
	//
	//		time.Sleep(2 * time.Second)
	//	}
	//}()
	//return nil

	// Example 2
	_, err := r.Write([]byte(time.Now().String()))
	err = r.Drain()
	return err
}

func (h *ClientHandler) Handler(r *tcp.Remote) error {
	r.Logger().Debug("message received from server: ", r.Content())
	_, err := r.Write([]byte(time.Now().String()))
	err = r.Drain()
	return err
	//return nil
}

func (h *ClientHandler) OnClosed(r *tcp.Remote) error {
	r.Logger().Info("Server closed")
	return nil
}

func Example_NewTcpServer() *tcp.Server {
	return tcp.NewAsyncTcpServer(
		&tcp.TcpsConfig{
			Host:           "0.0.0.0",
			Port:           "8090",
			ByteOrder:      "big",
			MaxOpenConn:    5,
			MessageHandler: &ServerHandler{},
			Logger:         logger,
		},
	)
}

func Example_NewAsyncTcpClient_1() {
	tcp.NewAsyncTcpClient(&tcp.TcpcConfig{
		Host:           "127.0.0.1",
		Port:           "8090",
		ByteOrder:      "big",
		MessageHandler: &ClientHandler{},
		Logger:         logger,
	})
}

func Example_NewAsyncTcpClient_2() {
	tcp.NewAsyncTcpClient(&tcp.TcpcConfig{
		Host:           "127.0.0.1",
		Port:           "8090",
		ByteOrder:      "big",
		MessageHandler: &ClientHandler{},
		Logger:         logger,
	})
}

func TestTcp() {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	s := Example_NewTcpServer()

	time.Sleep(1 * time.Second)

	Example_NewAsyncTcpClient_1()

	// Client 2
	//go func() {
	//	wg.Add(1)
	//	Example_NewAsyncTcpClient_2()
	//}()

	go func() {
		time.Sleep(100 * time.Second)
		s.Stop()
		wg.Done()
	}()

	wg.Wait()
}
