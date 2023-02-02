package tcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
)

const headerLength = 2                              // 消息头长度
var bufLength = int(math.Pow(2, 16) + headerLength) //

var welcome = []byte("i received your message")
var empty = make([]byte, 0)
var (
	defaultsConfig = &TcpsConfig{
		MessageHandler: &MessageHandler{},
		Logger:         nil,
		Host:           "0.0.0.0",
		Port:           "8090",
		MaxOpenConn:    5,
		ByteOrder:      tcpByteOrder,
	}
)

type MessageHandler struct{}

// Handler 处理接收到的消息
func (t *MessageHandler) Handler(r *Remote) error {
	_, err := r.Write(welcome)
	return err
}

// OnAccepted 当客户端连接时触发的操作
func (t *MessageHandler) OnAccepted(r *Remote) error {
	fmt.Printf("welcome to the world: %v\n" + r.Addr())
	return nil
}

// OnClosed 当客户端断开连接时触发的操作
func (t *MessageHandler) OnClosed(r *Remote) error {
	fmt.Printf("remote closed the connection: %v\n" + r.Addr())
	return nil
}

// ReadMessage 自定义读取方法，初始情况下，Content 为长度为2的数组
// @param   msg  *Frame  TCP消息帧
// @return  error 错误原因
func (t *MessageHandler) Read(conn net.Conn, buf io.Writer) error {
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
func (t *MessageHandler) Write(conn net.Conn, buf []byte) error {
	if _, err := msg.Conn.Write(msg.Pack()); err != nil {
		return err
	}
	return nil
}

type Remote struct {
	s         *Server
	conn      net.Conn
	index     int
	addr      string
	byteOrder string
	rx        []byte
	tx        []byte
	rxOffset  int
	txOffset  int
	lock      *sync.RWMutex
}

func (r *Remote) Addr() string           { return r.addr }
func (r *Remote) String() string         { return r.Addr() }
func (r *Remote) Logger() LoggerIface    { return r.s.logger }
func (r *Remote) Handler() ServerHandler { return r.s.messageHandler }
func (r *Remote) Close() error           { return r.conn.Close() }
func (r *Remote) Cap() int               { return bufLength - headerLength }

// Read 将缓冲区的数据读取到切片buf内，并返回实际读取的数据长度，其效果等同于copy，不建议使用
func (r *Remote) Read(buf []byte) (int, error) {
	if s := r.Unread(); len(s) == 0 { // 没有数据可以读了
		return 0, io.EOF
	} else {
		i := copy(buf, s)
		r.rxOffset += i // 标记被读取的数据量
		return i, nil
	}
}

// Write 将切片buf中的内容追加到发数据缓冲区内，并返回追加的数据长度;
// 若缓冲区大小不足以写入全部数据，则返回实际写入的数据长度，
// 返回值err恒为nil
func (r *Remote) Write(buf []byte) (int, error) {
	r.lock.Lock() // 避免 ServerHandler.AcceptHandlerFunc 与 ServerHandler.HandlerFunc 并发操作
	defer r.lock.Unlock()

	//i := 0
	//if len(buf) <= bufLength-r.txOffset {
	//	// 有足够的空间可以全部写入数据
	//	i = len(buf)
	//} else {
	//	// 计算可以写入的最大字节数
	//	i = bufLength - r.txOffset
	//}
	//copy(r.tx[r.txOffset:], buf[:i])
	i := copy(r.tx[r.txOffset:], buf)
	r.txOffset += i // 更新未发送数据的结束下标

	return i, nil
}

// Copy 将缓冲区的数据拷贝到切片p内，并返回实际读取的数据长度
func (r *Remote) Copy(p []byte) (int, error) { return r.Read(p) }

// Content 获取读到的字节流
func (r *Remote) Content() []byte {
	if r.rxOffset == headerLength {
		return empty
	}
	return r.rx[headerLength : r.rxOffset+1]
}

// Unread 获取未读取的消息流
func (r *Remote) Unread() []byte {
	length := r.Len()
	if length == 0 {
		return empty
	}
	return r.rx[r.rxOffset : length+r.rxOffset]
}

// Len 获取接收数据的长度
func (r *Remote) Len() int {
	if r.rxOffset == headerLength {
		return 0
	}
	if r.byteOrder == "big" {
		return int(binary.BigEndian.Uint16(r.rx[:headerLength]))
	} else {
		return int(binary.LittleEndian.Uint16(r.rx[:headerLength]))
	}
}

// Send 将缓冲区的数据发生到服务端
func (r *Remote) Send() (n int, err error) {
	r.lock.Lock()
	defer r.lock.Unlock() // 读写分离

	if r.txOffset == headerLength {
		return 0, nil
	}
	r.makeHeader()
	return r.conn.Write(r.tx[:r.txOffset])
}
func (r *Remote) reset() {
	r.conn, r.addr = nil, ""
	r.rxOffset, r.txOffset = headerLength, headerLength // 重置游标
}

func (r *Remote) makeHeader() {
	if r.byteOrder == "big" {
		binary.BigEndian.PutUint16(r.tx, uint16(r.txOffset-headerLength))
	} else {
		binary.LittleEndian.PutUint16(r.tx, uint16(r.txOffset-headerLength))
	}
}

// Server tcp 服务端实现
type Server struct {
	messageHandler ServerHandler `description:"消息处理方法"`
	logger         LoggerIface   `description:"日志"`
	listener       net.Listener  `description:"listener"`
	remotes        []*Remote     `description:"客户端连接"`
	isRunning      bool          `description:"是否正在运行"`
	lock           *sync.RWMutex `description:"连接建立和释放时加锁"`
	addr           string        `description:"工作地址"`
	MaxOpenConn    int           `description:"最大连接数量"`
	ByteOrder      string        `description:"消息长度字节序"`
}

// Addr 获取工作地址
func (s *Server) Addr() string   { return s.addr }
func (s *Server) String() string { return s.Addr() }

// IsRunning 是否正在运行
func (s *Server) IsRunning() bool { return s.isRunning }

// SetMaxOpenConn 修改TCP的最大连接数量
// @param  num  int  连接数量
func (s *Server) SetMaxOpenConn(num int) *Server {
	s.MaxOpenConn = num
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

// SetMessageHandler 设置消息处理钩子函数
func (s *Server) SetMessageHandler(handler ServerHandler) *Server {
	s.messageHandler = handler
	return s
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
		_ = s.listener.Close()
	}
}

// Serve 阻塞式启动TCP服务，若服务已在运行，则返回错误信息
func (s *Server) Serve() error {
	if s.isRunning {
		return errors.New("server already running")
	}
	if s.messageHandler == nil {
		return errors.New("server MessageHandler is not define")
	}
	// 初始化连接记录池
	s.remotes = make([]*Remote, s.MaxOpenConn)
	for i := 0; i < s.MaxOpenConn; i++ {
		s.remotes[i] = &Remote{
			index:     i,
			conn:      nil,
			addr:      "",
			byteOrder: s.ByteOrder,
			s:         s,
			rxOffset:  headerLength,
			txOffset:  headerLength,
			rx:        make([]byte, bufLength, bufLength),
			tx:        make([]byte, bufLength, bufLength),
			lock:      &sync.RWMutex{},
		}
	}

	if s.ByteOrder == "" { // 默认大端字节序
		s.ByteOrder = tcpByteOrder
	}

	// 使用 net.Listen 监听连接的地址与端口
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.logger.Info("server listening on: ", s.addr)
	s.listener, s.isRunning = listener, true // 修改TCP运行状态

	for {
		conn, err := listener.Accept() // 等待连接
		if err != nil {
			continue
		}

		s.lock.Lock() // 建立连接时禁止并发

		if s.GetOpenConnNums() < s.MaxOpenConn { // 为客户端建立连接, 此时一定存在空闲槽位
			for i := 0; i < s.MaxOpenConn; i++ {
				if s.remotes[i].conn != nil {
					continue
				}
				// 发现空闲槽位，接受客户端连接
				s.remotes[i].conn = conn
				s.remotes[i].addr = conn.RemoteAddr().String()

				go process(s.remotes[i]) // 对每个新连接创建一个协程进行连接处理
				break
			}
		} else { // 达到最大连接数量限制
			s.logger.Warn("the connection number reached the upper limit, " + conn.RemoteAddr().String())
			_ = conn.Close()
		}

		s.lock.Unlock()
	}
}

// process 处理TCP连接
func process(r *Remote) {
	defer func() {
		_ = r.Handler().OnClosed(r) // 处理关闭时任务
		r.reset()
	}()

	go func() { // 处理连接时任务
		err := r.Handler().OnAccepted(r)
		if err != nil {
			r.Logger().Error(r.addr+": error on accept, ", err.Error())
		}
	}()

	for { // 处理通信中任务
		err := r.Handler().Read(r.conn, r)
		if err != nil {
			break
		}

		//
		// ******************** 处理过程 ********************
		//
		err = r.Handler().Handler(r)
		if err != nil {
			r.Logger().Warn("handler failed, ", err.Error())
			continue
		}
	}
}

type TcpsConfig struct {
	MessageHandler ServerHandler
	Logger         LoggerIface
	Host           string `json:"tcps_host"`
	Port           string `json:"tcps_port"`
	MaxOpenConn    int    `json:"max_open_conn"`
	ByteOrder      string `json:"byte_order"`
}

// NewAsyncTcpServer 创建一个新的TCP server
//
// TCP server 此处已做消息处理的解构，提供一个默认的消息处理方法；
// 在实际的业务处理中，需要自定义一个struct，并实现ServerHandler的接口，
// 当服务端接收到来自客户端的数据时，会首先创建ServerHandler.OnAccepted()协程，进行连接时任务处理；
// 并在收到客户端消息时自动调用ServerHandler.Handler()方法来处理数据;
//
// 若需返回数据到客户端，则ServerHandler.Handler()方法实现的返回值必须是TCPMessage{}的指针，若为nil则不发送数据。
// 因此为保证数据处理的完整性，建议将ServerHandler{}作为自定义struct的第一个匿名字段，并重写Handler()方法；
//
// # Usage
//
//	// 1. 首先创建一个自定义struct，并将ServerHandler作为第一个匿名字段：
//	type TCPHandler struct {
//		ServerHandler
//	}
//
//	// 2. 重写Handler(msg *Frame) *Frame()方法：
//	func (h *TCPHandler) Handler(msg *Frame) *Frame {
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
			MaxOpenConn:    defaultsConfig.MaxOpenConn,
			ByteOrder:      defaultsConfig.ByteOrder,
			messageHandler: defaultsConfig.MessageHandler,
			logger:         defaultsConfig.Logger,
			addr:           net.JoinHostPort(defaultsConfig.Host, defaultsConfig.Port),
			lock:           &sync.RWMutex{},
			isRunning:      false,
		}
	} else {
		s = &Server{
			MaxOpenConn:    c[0].MaxOpenConn,
			ByteOrder:      c[0].ByteOrder,
			messageHandler: c[0].MessageHandler,
			logger:         c[0].Logger,
			addr:           net.JoinHostPort(c[0].Host, c[0].Port),
			lock:           &sync.RWMutex{},
			isRunning:      false,
		}
	}

	go func() {
		err := s.Serve()
		if err != nil {
			s.logger.Error("Server started failed: ", err.Error())
		}
	}()

	return s
}
