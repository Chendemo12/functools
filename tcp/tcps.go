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
	"sync/atomic"
)

var bufLength = int(math.Pow(2, 16) + headerLength) // 缓冲区大小

var empty = make([]byte, 0)
var welcome = []byte("i received your message")
var defaultsConfig = &TcpsConfig{
	MessageHandler:   &MessageHandler{},
	Logger:           nil,
	Host:             "0.0.0.0",
	Port:             "8090",
	ByteOrder:        tcpByteOrder,
	MaxOpenConn:      100,
	IptableLimit:     false,
	LazyInitDisabled: false,
}

// MessageHandler 消息处理方法，同样适用于客户端和服务端
type MessageHandler struct{}

// Handler 处理接收到的消息
func (h *MessageHandler) Handler(r *Remote) error {
	_, err := r.Write(welcome)
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
	num        int    `description:"最大连接数量"`
	proto      string `description:"协议类型"`
	dport      string `description:"目的端口"`
	dst        string `description:"目标地址"`
	src        string `description:"源地址"`
	isIptables bool   `description:"是否开启IPTABLE限制"`
}

func (l *ConnLimit) String() string {
	if l.src != "" {
		return l.InputWithSrc()
	} else {
		return l.Input()
	}
}

// Num 最大连接数
func (l *ConnLimit) Num() int { return l.num }

// Input INPUT 限制命令
func (l *ConnLimit) Input() string {
	return fmt.Sprintf(
		"iptables -I INPUT -p %s --dport %s -m connlimit --connlimit-above %d -m state --state NEW -j DROP",
		l.proto, l.dport, l.num,
	)
}

// Output OUTPUT 限制命令
func (l *ConnLimit) Output() string {
	// 插入到队头
	return fmt.Sprintf(
		"iptables -I OUTPUT -p %s --dport %s -m connlimit --connlimit-above %d -j DROP",
		l.proto, l.dport, l.num,
	)
}

func (l *ConnLimit) InputWithSrc() string {
	return fmt.Sprintf(
		"iptables -I INPUT -p %s -s %s --dport %s -m connlimit --connlimit-above %d -j DROP",
		l.proto, l.src, l.dport, l.num,
	)
}

func (l *ConnLimit) OutputWithSrc() string {
	return fmt.Sprintf(
		"iptables -I OUTPUT -p %s -s %s --dport %s -m connlimit --connlimit-above %d -j DROP",
		l.proto, l.src, l.dport, l.num,
	)
}

// Cmd 构建规则
func (l *ConnLimit) Cmd() *exec.Cmd {
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
func (l *ConnLimit) Execute() error {
	if !l.isIptables {
		return nil
	}
	if runtime.GOOS != "windows" {
		_, err := l.Cmd().Output()
		return err
	} else {
		return errors.New("OS not supported")
	}
}

// ============================================================================

// Server tcp 服务端实现
type Server struct {
	handler          HandlerFunc     `description:"消息处理方法"`
	logger           logger.Iface    `description:"日志"`
	listener         net.Listener    `description:"listener"`
	lock             *sync.Mutex     `description:"连接建立和释放时加锁"`
	wg               *sync.WaitGroup `description:"广播任务"`
	addr             string          `description:"工作地址"`
	byteOrder        string          `description:"消息长度字节序"`
	remotes          []*Remote       `description:"客户端连接"`
	limit            *ConnLimit      `description:"连接限制"`
	isRunning        *atomic.Bool    `description:"是否正在运行"`
	lazyInitDisabled bool            `description:"是否禁用延迟初始化"`
}

func (s *Server) newRemote(index int, initBuf bool) *Remote {
	r := &Remote{
		index:     index,
		conn:      nil,
		addr:      "",
		logger:    s.logger,
		byteOrder: s.byteOrder,
		rxEnd:     headerLength,
		txEnd:     headerLength,
		lock:      &sync.Mutex{},
	}

	if initBuf {
		r.rx = make([]byte, bufLength/4, bufLength)
		r.tx = make([]byte, bufLength/4, bufLength)
	}

	return r
}

func (s *Server) init() error {
	if s.handler == nil {
		return errors.New("server MessageHandler is not define")
	}
	if s.isRunning.Load() {
		return errors.New("server already running")
	}

	// 初始化连接记录池, 默认先分配100个连接
	s.remotes = make([]*Remote, defaultMaxOpenConn)
	for i := 0; i < defaultMaxOpenConn; i++ {
		s.remotes[i] = s.newRemote(i, false)
	}

	if s.lazyInitDisabled && s.limit.num > 0 { // 禁用延迟初始化，立即分配连接内存
		if s.limit.num > defaultMaxOpenConn {
			for i := 0; i < defaultMaxOpenConn; i++ {
				s.remotes[i].rx = make([]byte, bufLength/4, bufLength)
				s.remotes[i].tx = make([]byte, bufLength/4, bufLength)
			}

			for i := 0; i < s.limit.num-defaultMaxOpenConn; i++ {
				// 设置的最大连接数超过了默认值
				s.remotes = append(s.remotes, s.newRemote(defaultMaxOpenConn+i, true))
			}
		} else {
			for i := 0; i < s.limit.num; i++ {
				s.remotes[i].rx = make([]byte, bufLength/4, bufLength)
				s.remotes[i].tx = make([]byte, bufLength/4, bufLength)
			}
		}
	}

	// 设置连接限制
	if err := s.limit.Execute(); err != nil {
		s.logger.Warn("iptables executed failed, err: " + err.Error())
	}

	// 使用 net.Listen 监听连接的地址与端口
	listener, err := net.Listen(s.limit.proto, s.addr)
	if err != nil {
		return err
	}

	s.listener = listener // 修改TCP运行状态
	s.isRunning.Store(true)

	s.logger.Info(s.String())

	return nil
}

// 查找一个合适的连接槽位，此时一定有一个可用的槽位
func (s *Server) findRemote() int {
	index := -1
	for i := 0; i < len(s.remotes); i++ {
		if s.remotes[i].conn == nil && len(s.remotes[i].rx) > headerLength {
			// 发现空闲槽位且缓冲区已经分配了内存空间
			index = i
			break
		}
	}

	if index == -1 {
		// 没有找到分配了空间的空闲槽位
		for i := 0; i < len(s.remotes); i++ {
			if s.remotes[i].conn == nil {
				// 发现空闲槽位
				index = i
				break
			}
		}

		// 为槽位分配空间
		s.remotes[index].rx = make([]byte, bufLength/4, bufLength)
		s.remotes[index].tx = make([]byte, bufLength/4, bufLength)
	}
	return index
}

// 阻塞处理通信中任务
func (s *Server) proc() {
	for s.isRunning.Load() {
		conn, err := s.listener.Accept() // 等待连接
		if err != nil {
			s.logger.Warn("accept conn failed, err: " + err.Error())
			continue
		}

		s.lock.Lock() // 建立连接时禁止并发
		nextOpenConnNum := s.GetOpenConnNums() + 1
		openConnLimit := s.MaxOpenConnNums()
		if openConnLimit > 0 && nextOpenConnNum > openConnLimit {
			// 限制连接数，判断是否达到最大连接数, 当达到最大连接数, 不接受新的连接
			s.logger.Warn("reached the upper limit, conn will be closed: " + conn.RemoteAddr().String())
			_ = conn.Close()
			continue
		}

		// === 未达到最大连接数, 接受客户端连接
		if nextOpenConnNum > len(s.remotes) {
			// 当前的连接数大于连接池大小, 创建新的连接对象
			s.remotes = append(s.remotes, s.newRemote(nextOpenConnNum, true))
		}

		// 查找一个合适的连接槽位
		connIndex := s.findRemote()
		s.remotes[connIndex].conn = conn
		s.remotes[connIndex].addr = conn.RemoteAddr().String()
		go s.handle(s.remotes[connIndex])

		s.lock.Unlock()
	}
}

// 处理TCP连接
func (s *Server) handle(r *Remote) {
	defer func() {
		_ = s.handler.OnClosed(r) // 处理关闭时任务
		_ = r.Close()             // 关闭连接, OnClosed 内部可能还会发送消息，因此处理完成后再关闭连接
		r.reset()
	}()

	// 处理连接时任务
	err := s.handler.OnAccepted(r)
	if err != nil {
		s.logger.Error(r.addr+": error on accept, ", err.Error())
	}

	for s.isRunning.Load() { // 处理通信中任务
		err = r.readMessage()
		if err != nil {
			// 读取失败，处理连接断开事件后主动关闭连接
			break
		}

		//
		// ******************** 处理过程 ********************
		//
		err = s.handler.Handler(r)
		if err != nil {
			s.logger.Warnf("handler message of '%s' failed, %s", r.addr, err.Error())
		} else {
			err = r.Drain() // 如果不存在数据则会直接返回nil
			if err != nil {
				s.logger.Warnf("write message to '%s' failed, %s", r.addr, err.Error())
			}
		}
	}
}

func (s *Server) Addr() string { return s.addr }

func (s *Server) String() string {
	return fmt.Sprintf(
		"server linsten on: %s, running: %t, byteOrder: %s, maxOpenConns: %d, openConns: %d",
		s.addr, s.IsRunning(), s.byteOrder, s.MaxOpenConnNums(), s.GetOpenConnNums(),
	)
}

func (s *Server) IsRunning() bool { return s.isRunning.Load() }

func (s *Server) ByteOrder() string { return s.byteOrder }

func (s *Server) MaxOpenConnNums() int { return s.limit.Num() }

// GetOpenConnNums 获取当前TCP的连接数量
//
//	@return	int 打开的连接数量
func (s *Server) GetOpenConnNums() (v int) {
	v = 0
	for i := 0; i < len(s.remotes); i++ {
		if s.remotes[i].conn != nil {
			v++
		}
	}

	return
}

// Broadcast 将数据广播到所有客户端连接
//
//	@return	int 发送成功的客户端数量
func (s *Server) Broadcast(msg []byte) int {
	num := 0
	for i := 0; i < len(s.remotes); i++ {
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

// Close 依据连接地址关闭一个远端连接;
// 若连接已关闭，则不做任何操作，若连接处于活动状态下，此操作执行完成总后会触发 OnClosed 回调
func (s *Server) Close(addr string) error {
	if addr == "" { // do nothing
		return nil
	}

	for i := 0; i < len(s.remotes); i++ {
		r := s.remotes[i]
		if r.addr == addr {
			// 当连接被关闭后，process 函数将遇错退出，OnClosed回调将被自动触发
			return r.conn.Close()
		}
	}

	return nil
}

// Stop 停止并关闭全部TCP连接
func (s *Server) Stop() {
	if !s.IsRunning() { // 服务未启动
		return
	}
	// 此操作会触发handle任务的退出，并关闭连接
	s.isRunning.Store(false)

	// 关闭服务器句柄
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

// Start 异步非阻塞运行服务，若服务已在运行则返回错误信息
func (s *Server) Start() error {
	err := s.init()
	if err != nil {
		return err
	}

	go s.proc() // 异步非阻塞运行
	return nil
}

// Serve 阻塞式启动TCP服务，若服务已在运行则返回错误信息
func (s *Server) Serve() error {
	err := s.init()
	if err != nil {
		return err
	}

	s.proc() // 阻塞运行
	return nil
}

type TcpsConfig struct {
	MessageHandler   HandlerFunc  `json:"-"`
	Logger           logger.Iface `json:"-"`
	Host             string       `json:"host,omitempty"`
	Port             string       `json:"port,omitempty"`
	ByteOrder        string       `json:"byteOrder,omitempty"`
	MaxOpenConn      int          `json:"maxOpenConn,omitempty"`      // 最大连接数量，为0则不限制
	IptableLimit     bool         `json:"iptableLimit,omitempty"`     // 是否开启IPTABLE限制
	LazyInitDisabled bool         `json:"lazyInitDisabled,omitempty"` // 是否禁用延迟初始化，禁用延迟初始化后，程序将在启动时一次性的分配完连接内存（有最大数量限制）
}

// NewTcpServer 创建一个TCP server，此处未主动运行，需手动 Server.Start 启动服务
func NewTcpServer(c ...*TcpsConfig) *Server {
	s := &Server{
		handler:          defaultsConfig.MessageHandler,
		logger:           defaultsConfig.Logger,
		addr:             net.JoinHostPort(defaultsConfig.Host, defaultsConfig.Port),
		lazyInitDisabled: defaultsConfig.LazyInitDisabled,
		limit: &ConnLimit{
			num:        defaultsConfig.MaxOpenConn,
			proto:      "tcp",
			dport:      defaultsConfig.Port,
			dst:        "",
			src:        "",
			isIptables: defaultsConfig.IptableLimit,
		},
	}

	if len(c) > 0 {
		s.byteOrder = c[0].ByteOrder
		s.handler = c[0].MessageHandler
		s.logger = c[0].Logger
		s.addr = net.JoinHostPort(c[0].Host, c[0].Port)
		s.lazyInitDisabled = c[0].LazyInitDisabled
		s.limit.num = c[0].MaxOpenConn
		s.limit.dport = c[0].Port
		s.limit.isIptables = c[0].IptableLimit
	}
	s.lock, s.wg = &sync.Mutex{}, &sync.WaitGroup{}
	s.isRunning = &atomic.Bool{}

	if s.byteOrder == "" { // 默认大端字节序
		s.byteOrder = tcpByteOrder
	}

	if s.logger == nil {
		s.logger = logger.NewDefaultLogger()
	}

	return s
}

// NewAsyncTcpServer 创建一个新的TCP server
//
// TCP server 此处已做消息处理的解构，提供一个默认的消息处理方法；
// 在实际的业务处理中，需实现 HandlerFunc 接口，
//
// 当服务端接收到来自客户端的数据时，会首先执行 HandlerFunc.OnAccepted 进行连接时任务处理,
// 此任务执行完毕后才会读取并处理数据；
//
// 收到客户端消息时自动调用 HandlerFunc.Handler 方法来处理数据;
//
// 若需返回数据到客户端，则可直接在 HandlerFunc.Handler 内通过 Remote.Write 方法实现。
//
// # Usage
//
//	// 1. 首先创建一个自定义struct，并实现 HandlerFunc 接口
//	type TCPHandler struct {
//		MessageHandler
//	}
//
//	// 2. 重写 Handler 方法：
//	func (h *TCPHandler) Handler(r *Remote) error {
//		r.Read(buf)		// 复制数据到buf
//		r.Write(buf)	// 返回数据
//		r.Drain()		// 立即发送数据，并阻塞直到发送完成，如果不调用此方法，则在 HandlerFunc.Handler 处理完毕后再发送数据
//		return nil		// 返回nil则表示处理成功，会调用 Remote.Drain 方法发送数据（如果有）
//	}
//
//	// 3. 创建并启动一个新服务
//	ts := NewAsyncTcpServer(
//		&TcpsConfig{
//			Host:           "0.0.0.0",
//			Port:           8090,
//			maxOpenConn:    5,
//			MessageHandler: &TCPHandler{},
//			byteOrder:      "big",
//			logger:         nil,
//		},
//	)
func NewAsyncTcpServer(c ...*TcpsConfig) *Server {
	var s *Server
	s = NewTcpServer(c...)

	err := s.Start()
	if err != nil {
		s.logger.Errorf("server started on '%s' failed: %s", s.addr, err.Error())
	}

	return s
}
