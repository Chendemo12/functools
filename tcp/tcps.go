package tcp

import (
	"errors"
	"gitlab.cowave.com/gogo/functools/cprint"
	"io"
	"net"
	"strconv"
	"sync"
)

var (
	lock           = &sync.Mutex{}
	welcome        = []byte("i received your message")
	defaultsConfig = &TcpsConfig{
		MessageHandler: &MessageHandler{},
		Logger:         nil,
		Host:           "0.0.0.0",
		Port:           8090,
		MaxOpenConn:    5,
		ByteOrder:      tcpByteOrder,
	}
)

type MessageHandler struct{}

// HandlerFunc 处理接收到的消息
func (t *MessageHandler) HandlerFunc(msg *Frame) *Frame {
	msg.Content = welcome

	return msg
}

// AcceptHandlerFunc 当客户端连接时触发的操作
func (t *MessageHandler) AcceptHandlerFunc(conn net.Conn) {
	cprint.Green("welcome to the world: " + conn.RemoteAddr().String())
}

// ReadMessage 自定义读取方法，初始情况下，Content 为长度为2的数组
// @param   msg  *Frame  TCP消息帧
// @return  error 错误原因
func (t *MessageHandler) ReadMessage(msg *Frame) error {
	// 接收数据
	n, err := msg.Conn.Read(msg.Content[:2])
	if err != nil {
		return err
	}
	if n <= 2 {
		return errors.New("the message header is incomplete")
	}
	// 获取消息长度
	msg.Length = MessageHeaderUnpack(msg.ByteOrder, msg.Content[:2])
	msg.Content = make([]byte, msg.Length) // 修改消息体长度
	if _, err = io.ReadFull(msg.Conn, msg.Content); err != nil {
		return err
	} // ReadFull 会把填充contentBuf填满为止

	return nil
}

// SendMessage 主动向客户端发送消息，进行TCPMessage消息封装
// @param   msg  *Frame  TCP消息帧
// @return  error 错误原因
func (t *MessageHandler) SendMessage(msg *Frame) error {
	if _, err := msg.Conn.Write(msg.Pack()); err != nil {
		return err
	}
	return nil
}

type remote struct {
	index int
	conn  net.Conn
	addr  string
	s     *Server
}

func (r *remote) Logger() LoggerIface    { return r.s.logger }
func (r *remote) Handler() ServerHandler { return r.s.messageHandler }
func (r *remote) Close() error           { return r.conn.Close() }

type Server struct {
	messageHandler ServerHandler //
	logger         LoggerIface   //
	listener       net.Listener  //
	remotes        []*remote     // 客户端连接
	isRunning      bool          // 是否正在运行
	Host           string        // TCPs host, default="0.0.0.0"
	Port           int           // TCPs port, default=8090
	MaxOpenConn    int           // TCP最大连接数量
	ByteOrder      string        // 消息长度字节序
}

// IsRunning 是否正在运行
func (s *Server) IsRunning() bool { return s.isRunning }

// SetPort 修改TCP服务工作端口
// @param  port  int  端口
func (s *Server) SetPort(port int) *Server {
	s.Port = port
	return s
}

// SetHost 修改TCP服务工作地址
// @param  host  string  工作地址
func (s *Server) SetHost(host string) *Server {
	s.Host = host
	return s
}

// SetMaxOpenConn 修改TCP的最大连接数量
// @param  cons  int  连接数量
func (s *Server) SetMaxOpenConn(conn int) *Server {
	s.MaxOpenConn = conn
	return s
}

// SetMessageHandlerFunc 设置消息处理钩子函数
func (s *Server) SetMessageHandlerFunc(handler ServerHandler) *Server {
	s.messageHandler = handler
	return s
}

func (s *Server) Disconnect(r *remote) *Server {
	lock.Lock()
	defer lock.Unlock()

	if s.remotes[r.index] != nil {
		_ = s.remotes[r.index].Close()
		s.remotes[r.index] = nil
	}
	return s
}

// GetOpenConnNums 获取当前TCP的连接数量
// @return  int 打开的连接数量
func (s *Server) GetOpenConnNums() (v int) {
	v = 0
	if s != nil {
		for i := 0; i < s.MaxOpenConn; i++ {
			if s.remotes[i] != nil {
				v++
			}
		}
	}

	return
}

// Stop 停止并关闭全部TCP连接
func (s *Server) Stop() {
	if !s.IsRunning() { // 服务未启动
		return
	}

	// 逐个关闭客户端连接
	for i := 0; i < s.MaxOpenConn; i++ {
		if s.remotes[i] != nil {
			_ = s.remotes[i].Close()
		}
	}

	// 关闭服务器句柄
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return
		}
	}
}

// Serve 阻塞式启动TCP服务，若服务已在运行，则返回错误信息
func (s *Server) Serve() error {
	if s.isRunning {
		return errors.New("TCP server already running")
	}
	// 初始化连接记录池
	s.remotes = make([]*remote, s.MaxOpenConn)
	if s.messageHandler == nil {
		return errors.New("TCP server MessageHandler is not define")
	}
	if s.ByteOrder == "" { // 默认大端字节序
		s.ByteOrder = tcpByteOrder
	}

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	// 使用 net.Listen 监听连接的地址与端口
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.logger.Error("listen fail, ", err.Error())
		return err
	}
	s.logger.Info("TCP server listening on ", addr)

	s.listener = listener
	s.isRunning = true // 修改TCP运行状态

	for {
		conn, err := listener.Accept() // 等待连接
		if err != nil {
			s.logger.Warn("accept fail, ", err.Error())
			continue
		}

		if s.GetOpenConnNums() >= s.MaxOpenConn { // 达到最大连接数量限制
			s.logger.Warn("the TCP connection number reached the upper limit.")
			_ = conn.Close()
			continue
		}

		// 为客户端建立连接, 此时一定存在空闲槽位
		//
		lock.Lock() // 建立连接时禁止并发
		for i := 0; i < s.MaxOpenConn; i++ {
			if s.remotes[i] == nil { // 发现空闲槽位，接受客户端连接
				r := &remote{index: i, conn: conn, s: s, addr: conn.RemoteAddr().String()}
				s.remotes[i] = r
				go process(r) // 对每个新连接创建一个协程进行连接处理
				break
			} else {
				continue
			}
		}
		lock.Unlock()
	}
}

// process 处理TCP连接
func process(r *remote) {
	// 获取客户端的网络地址信息
	r.Logger().Info(r.addr + ": Connected!")
	go r.Handler().AcceptHandlerFunc(r.conn) // 处理连接时任务

	defer func() {
		r.s.Disconnect(r)
		r.Logger().Info(r.addr + ": Closed!")
	}()

	msg := &Frame{Conn: r.conn, ByteOrder: r.s.ByteOrder}
	for { // 处理通信中任务
		msg.Content = make([]byte, 2)
		err := r.Handler().ReadMessage(msg)
		if err != nil {
			break
		}

		//
		// ******************** 处理过程 ********************
		//
		resp := r.Handler().HandlerFunc(msg)
		if resp != nil { // 需要向客户端返回消息
			if err = r.Handler().SendMessage(msg); err != nil {
				r.Logger().Warn("write to client failed, ", err.Error())
				break
			}
		}
	}
}

type TcpsConfig struct {
	MessageHandler ServerHandler
	Logger         LoggerIface
	Host           string `json:"tcps_host"`
	Port           int    `json:"tcps_port"`
	MaxOpenConn    int    `json:"max_open_conn"`
	ByteOrder      string `json:"byte_order"`
}

// NewAsyncTcpServer 创建一个新的TCP server
//
// TCP server 此处已做消息处理的解构，默认提供了一个没啥卵用的消息处理方法：TcpMessageHandler；
// 在实际的业务处理中，需要自定义一个struct，并实现IpServerMessageHandler{}的全部接口，
// 当服务端接收到来自客户端的数据时，会首先创建IpServerMessageHandler.AcceptHandlerFunc()协程，进行连接时任务处理；
// 并在收到客户端消息时自动调用IpServerMessageHandler.HandlerFunc()方法来处理数据;
//
// 若需返回数据到客户端，则IpServerMessageHandler.HandlerFunc()方法实现的返回值必须是TCPMessage{}的指针，若为nil则不发送数据。
// 因此为保证数据处理的完整性，建议将TCPMessageHandler{}作为自定义struct的第一个匿名字段，并重写HandlerFunc()方法；
//
// # Usage
//
//	// 1. 首先创建一个自定义struct，并将TCPMessageHandler{}作为第一个匿名字段：
//	type TCPHandler struct {
//		TcpMessageHandler
//	}
//
//	// 2. 重写HandlerFunc(msg *Frame) *Frame()方法：
//	func (h *TCPHandler) HandlerFunc(msg *Frame) *Frame {
//		fmt.Println(msg.Hex())
//		return nil // 不返回任何数据
//	}
//
//	// 3. 启动一个新服务
//	ts := NewAsyncTcpServer(
//		&TcpsConfig{
//			Host:           "0.0.0.0",
//			Port:           8090,
//			MaxOpenConn:    5,
//			MessageHandler: &TCPHandler{},
//			ByteOrder:      "big",
//			logger:         logger.ConsoleLogger{},
//		},
//	)
func NewAsyncTcpServer(c ...*TcpsConfig) *Server {
	var s *Server

	if len(c) == 0 {
		s = &Server{
			Host:           defaultsConfig.Host,
			Port:           defaultsConfig.Port,
			MaxOpenConn:    defaultsConfig.MaxOpenConn,
			ByteOrder:      defaultsConfig.ByteOrder,
			messageHandler: defaultsConfig.MessageHandler,
			logger:         defaultsConfig.Logger,
			isRunning:      false,
		}
	} else {
		s = &Server{
			Host:           c[0].Host,
			Port:           c[0].Port,
			MaxOpenConn:    c[0].MaxOpenConn,
			ByteOrder:      c[0].ByteOrder,
			messageHandler: c[0].MessageHandler,
			logger:         c[0].Logger,
			isRunning:      false,
		}
	}

	go func() {
		if err := s.Serve(); err != nil {
			s.logger.Error("TCP Server started failed: ", err.Error())
		} else {
			s.logger.Info("TCP Server Listen on: ", s.listener.Addr().String())
		}
	}()

	return s
}
