package tcp

import (
	"errors"
	"fmt"
	"github.com/Chendemo12/functools/logger"
	"math"
	"net"
	"os/exec"
	"runtime"
	"sync"
)

var bufLength = int(math.Pow(2, 16) + headerLength) //

var empty = make([]byte, 0)
var welcome = []byte("i received your message")
var defaultsConfig = &TcpsConfig{
	MessageHandler: &MessageHandler{},
	Logger:         nil,
	Host:           "0.0.0.0",
	Port:           "8090",
	MaxOpenConn:    5,
	ByteOrder:      tcpByteOrder,
}

// MessageHandler 客户端消息处理方法
type MessageHandler struct{}

// Handler 处理接收到的消息
func (h *MessageHandler) Handler(r *Remote) error {
	_, err := r.Write(welcome)
	err = r.Drain()
	return err
}

// OnAccepted 当客户端连接时触发的操作
func (h *MessageHandler) OnAccepted(r *Remote) error {
	r.Logger().Info("welcome to the world: ", r.Addr())
	return nil
}

// OnClosed 当客户端断开连接时触发的操作
func (h *MessageHandler) OnClosed(r *Remote) error {
	r.Logger().Info("remote closed the connection: ", r.Addr())
	return nil
}

// ConnLimit 连接限制
type ConnLimit struct {
	num   int    `description:"最大连接数量"`
	proto string `description:"协议类型"`
	dport string `description:"目的端口"`
	dst   string `description:"目标地址"`
	src   string `description:"源地址"`
}

// Num 最大连接数
func (l ConnLimit) Num() int { return l.num }

// Input INPUT 限制命令
func (l ConnLimit) Input() string {
	return fmt.Sprintf(
		"iptables -I INPUT -p %s --dport %s -m connlimit --connlimit-above %d -m state --state NEW -j DROP",
		l.proto, l.dport, l.num,
	)
}

// Output OUTPUT 限制命令
func (l ConnLimit) Output() string {
	// 插入到队头
	return fmt.Sprintf(
		"iptables -I OUTPUT -p %s --dport %s -m connlimit --connlimit-above %d -j DROP",
		l.proto, l.dport, l.num,
	)
}

func (l ConnLimit) InputWithSrc() string {
	return fmt.Sprintf(
		"iptables -I INPUT -p %s -s %s --dport %s -m connlimit --connlimit-above %d -j DROP",
		l.proto, l.src, l.dport, l.num,
	)
}

func (l ConnLimit) OutputWithSrc() string {
	return fmt.Sprintf(
		"iptables -I OUTPUT -p %s -s %s --dport %s -m connlimit --connlimit-above %d -j DROP",
		l.proto, l.src, l.dport, l.num,
	)
}

// Cmd 构建规则
func (l ConnLimit) Cmd() *exec.Cmd {
	cmd := exec.Command(
		"iptables", "-I", "INPUT",
		"-p", l.proto,
		"--dport", l.dport,
		"-m", "connlimit", "--connlimit-above", fmt.Sprintf("%d", l.num),
		// "--connlimit-mask", "0", "-m", "state", "--state", "NEW",
		"-j", "DROP",
	)
	return cmd
}

// Execute 执行命令
func (l ConnLimit) Execute() error {
	if runtime.GOOS != "windows" {
		_, err := l.Cmd().Output()
		return err
	} else {
		return errors.New("OS not supported")
	}
}

// Server tcp 服务端实现
type Server struct {
	handler   HandlerFunc     `description:"消息处理方法"`
	logger    logger.Iface    `description:"日志"`
	listener  net.Listener    `description:"listener"`
	lock      *sync.Mutex     `description:"连接建立和释放时加锁"`
	wg        *sync.WaitGroup `description:"广播任务"`
	addr      string          `description:"工作地址"`
	byteOrder string          `description:"消息长度字节序"`
	remotes   []*Remote       `description:"客户端连接"`
	limit     *ConnLimit      `description:"连接限制"`
	isRunning bool            `description:"是否正在运行"`
}

// Addr 获取工作地址
func (s *Server) Addr() string         { return s.addr }
func (s *Server) String() string       { return s.Addr() }
func (s *Server) IsRunning() bool      { return s.isRunning }
func (s *Server) ByteOrder() string    { return s.byteOrder }
func (s *Server) MaxOpenConnNums() int { return s.limit.Num() }

// SetMaxOpenConn 修改TCP的最大连接数量
//
//	@param	num	int	连接数量
func (s *Server) SetMaxOpenConn(num int) *Server {
	s.limit.num = num
	return s
}

// GetOpenConnNums 获取当前TCP的连接数量
//
//	@return	int 打开的连接数量
func (s *Server) GetOpenConnNums() (v int) {
	v = 0
	for i := 0; i < s.MaxOpenConnNums(); i++ {
		if s.remotes[i].conn != nil {
			v++
		}
	}

	return
}

// SetMessageHandler 设置消息处理钩子函数
func (s *Server) SetMessageHandler(handler HandlerFunc) *Server {
	s.handler = handler
	return s
}

// Broadcast 将数据广播到所有客户端连接
//
//	@return int 发送成功的客户端数量
func (s *Server) Broadcast(msg []byte) int {
	s.lock.Lock()
	defer s.lock.Unlock()

	num := 0
	for i := 0; i < s.MaxOpenConnNums(); i++ {
		r := s.remotes[i]
		if !r.IsConnected() {
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			_, err := r.Write(msg)
			if err != nil {
				return
			}

			if r.Drain() != nil {
				return
			}
			num += 1
		}()

	}
	s.wg.Wait()
	return num
}

// Stop 停止并关闭全部TCP连接
func (s *Server) Stop() {
	if !s.IsRunning() { // 服务未启动
		return
	}

	// 逐个关闭客户端连接
	for i := 0; i < s.MaxOpenConnNums(); i++ {
		if s.remotes[i] != nil {
			_ = s.remotes[i].Close()
		}
	}

	// 关闭服务器句柄
	if s.listener != nil {
		_ = s.listener.Close()
	}

	s.isRunning = false
}

// Serve 阻塞式启动TCP服务，若服务已在运行，则返回错误信息
func (s *Server) Serve() error {
	if s.isRunning {
		return errors.New("server already running")
	}
	if s.handler == nil {
		return errors.New("server MessageHandler is not define")
	}
	// 初始化连接记录池
	s.remotes = make([]*Remote, s.MaxOpenConnNums())
	for i := 0; i < s.MaxOpenConnNums(); i++ {
		s.remotes[i] = &Remote{
			index:     i,
			conn:      nil,
			addr:      "",
			logger:    s.logger,
			byteOrder: s.byteOrder,
			rxEnd:     headerLength,
			txEnd:     headerLength,
			rx:        make([]byte, bufLength, bufLength),
			tx:        make([]byte, bufLength, bufLength),
			lock:      &sync.Mutex{},
		}
	}

	err := s.limit.Execute() // 设置限制
	if err != nil {
		s.logger.Warn(err.Error())
	} else {
		s.logger.Info("iptables limit exceeded")
	}
	// 使用 net.Listen 监听连接的地址与端口
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.logger.Info(fmt.Sprintf(
		"server listening on: %s, with maximum number of connections: %d", s.addr, s.MaxOpenConnNums(),
	))
	s.listener, s.isRunning = listener, true // 修改TCP运行状态

	for s.isRunning {
		conn, err := listener.Accept() // 等待连接
		if err != nil {
			continue
		}

		s.lock.Lock() // 建立连接时禁止并发

		if s.GetOpenConnNums() < s.MaxOpenConnNums() { // 为客户端建立连接, 此时一定存在空闲槽位
			for i := 0; i < s.MaxOpenConnNums(); i++ {
				remote := s.remotes[i]
				if remote.conn != nil {
					continue
				}

				// 发现空闲槽位，接受客户端连接
				remote.conn = conn
				remote.addr = conn.RemoteAddr().String()

				go s.process(remote) // 对每个新连接创建一个协程进行连接处理
				break
			}
		} else { // 达到最大连接数量限制
			s.logger.Warn("reached the upper limit, closed: " + conn.RemoteAddr().String())
			_ = conn.Close()
		}

		s.lock.Unlock()
	}
	return nil
}

// process 处理TCP连接
func (s *Server) process(r *Remote) {
	defer func() {
		_ = s.handler.OnClosed(r) // 处理关闭时任务
		r.reset()
	}()

	// 处理连接时任务
	err := s.handler.OnAccepted(r)
	if err != nil {
		s.logger.Error(r.addr+": error on accept, ", err.Error())
	}

	for s.isRunning { // 处理通信中任务
		err := r.readMessage()
		if err != nil {
			break
		}

		//
		// ******************** 处理过程 ********************
		//
		err = s.handler.Handler(r)
		if err != nil {
			s.logger.Warn("handler failed, ", err.Error())
			continue
		}
	}
}

type TcpsConfig struct {
	MessageHandler HandlerFunc
	Logger         logger.Iface
	Host           string `json:"tcps_host"`
	Port           string `json:"tcps_port"`
	ByteOrder      string `json:"byte_order"`
	MaxOpenConn    int    `json:"max_open_conn"`
}

// NewAsyncTcpServer 创建一个新的TCP server
//
// TCP server 此处已做消息处理的解构，提供一个默认的消息处理方法；
// 在实际的业务处理中，需要自定义一个struct，并实现ServerHandler的接口，
// 当服务端接收到来自客户端的数据时，会首先创建ServerHandler.OnAccepted()协程，进行连接时任务处理；
// 并在收到客户端消息时自动调用ServerHandler.HandlerFunc()方法来处理数据;
//
// 若需返回数据到客户端，则ServerHandler.HandlerFunc()方法实现的返回值必须是TCPMessage{}的指针，若为nil则不发送数据。
// 因此为保证数据处理的完整性，建议将ServerHandler{}作为自定义struct的第一个匿名字段，并重写Handler()方法；
//
// # Usage
//
//	// 1. 首先创建一个自定义struct，并将ServerHandler作为第一个匿名字段：
//	type TCPHandler struct {
//		HandlerFunc
//	}
//
//	// 2. 重写Handler(msg *Frame) *Frame()方法：
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
//			maxOpenConn:    5,
//			MessageHandler: &TCPHandler{},
//			byteOrder:      "big",
//			logger:         logger.ConsoleLogger{},
//		},
//	)
func NewAsyncTcpServer(c ...*TcpsConfig) *Server {
	var s *Server

	if len(c) == 0 {
		s = &Server{
			byteOrder: defaultsConfig.ByteOrder,
			handler:   defaultsConfig.MessageHandler,
			logger:    defaultsConfig.Logger,
			addr:      net.JoinHostPort(defaultsConfig.Host, defaultsConfig.Port),
			limit: &ConnLimit{
				num:   defaultsConfig.MaxOpenConn,
				proto: "tcp",
				dport: defaultsConfig.Port,
				dst:   "",
				src:   "",
			},
		}
	} else {
		s = &Server{
			byteOrder: c[0].ByteOrder,
			handler:   c[0].MessageHandler,
			logger:    c[0].Logger,
			addr:      net.JoinHostPort(c[0].Host, c[0].Port),
			limit: &ConnLimit{
				num:   c[0].MaxOpenConn,
				proto: "tcp",
				dport: c[0].Port,
				dst:   "",
				src:   "",
			},
		}
	}
	s.lock, s.wg = &sync.Mutex{}, &sync.WaitGroup{}

	if s.byteOrder == "" { // 默认大端字节序
		s.byteOrder = tcpByteOrder
	}

	if s.logger == nil {
		s.logger = logger.NewDefaultLogger()
	}

	if s.MaxOpenConnNums() == 0 {
		s.SetMaxOpenConn(defaultMaxOpenConn)
	}

	go func() {
		err := s.Serve()
		if err != nil {
			s.logger.Error("Server started failed: ", err.Error())
		}
	}()

	return s
}
