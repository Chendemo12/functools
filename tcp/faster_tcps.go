package tcp

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/Chendemo12/functools/cprint"
	"io"
	"net"
	"sync"
)

const defaultHeaderLength = 2        // 默认消息头的长度
const defaultMemoryCap = 65536       // 客户端默认的内存大小
const preDistributionRemoteNums = 10 // 预分配内存的客户端最大数量
const tcpByteOrder = "big"

type HeaderLength int

const (
	ZeroBytes  HeaderLength = -1
	TwoBytes   HeaderLength = 2
	FourBytes  HeaderLength = 4
	EightBytes HeaderLength = 8
)

var (
	defaultFasterTcpsConfig = &FasterTcpsConfig{
		ServerHandler: &FasterMessageHandler{},
		Logger:        nil,
		Host:          "0.0.0.0",
		Port:          "8090",
		MaxOpenConn:   5,
		ByteOrder:     tcpByteOrder,
	}
)

// ---------------------- 服务端配置 ----------------------

type FasterTcpsConfig struct {
	ServerHandler *FasterMessageHandler
	Logger        LoggerIface
	Host          string       `json:"tcps_host"`
	Port          string       `json:"tcps_port"`
	MaxOpenConn   int          `json:"max_open_conn"`
	HeaderLength  HeaderLength `json:"header_length"` // 消息头长度，默认为2,若需禁用消息头则设置为-1，而非0，且最大值为8
	ByteOrder     string       `json:"byte_order"`
}

// ---------------------- 自定义处理方法 ----------------------

type FasterServerHandler interface {
	AcceptHandlerFunc(r *Remote)               // 当客户端连接时触发的操作,此协程应在连接关闭时主动退出，如需在此协程内向client发送消息，则必须通过调用Write/Drain实现
	Read(conn net.Conn, p []byte) (int, error) // 读取数据，允许自定义读取方法,
	HandlerFunc(r *Remote)                     // 处理接收到的消息
	ClosedHandlerFunc(r *Remote)               // 当连接断开时执行的方法
}
type FasterMessageHandler struct{}

// AcceptHandlerFunc 当客户端连接时触发的操作
func (t *FasterMessageHandler) AcceptHandlerFunc(r *Remote) {
	cprint.Green("welcome to the world: " + r.conn.RemoteAddr().String())
	<-r.Ctx().Done()
	return
}

// HandlerFunc 处理接收到的消息
func (t *FasterMessageHandler) HandlerFunc(r *Remote) {
	_, _ = r.Write(welcome)
}

// Read 自定义读取方法
func (t *FasterMessageHandler) Read(conn net.Conn, p []byte) (int, error) {
	// 读取数据
	n, err := io.ReadFull(conn, p[:1024])
	return n, err
}

// ClosedHandlerFunc 连接关闭时方法
func (t *FasterMessageHandler) ClosedHandlerFunc(r *Remote) {
	cprint.Green(r.conn.RemoteAddr().String() + "closed connection")
}

// ---------------------- 远端连接 ----------------------

type Remote struct {
	s        *FasterServer      // FasterServer
	si       int                // 记录当前连接所处 FasterServer 连接池的下标
	conn     net.Conn           // 远端链接对象，当此为nil时则表示此槽位空闲
	readC    chan struct{}      // 交替执行数据收发，支持在发送数据的同时读取下一包数据
	procC    chan struct{}      // 上一包数据是否处理完成
	ri       int                // 当前缓冲区数据结束的下标
	wi       int                // 当前未发送数据的结束下标
	lastRead int                // 上一次读取的数据下标
	rBuf     []byte             // 存放从TCP读到的数据,容量= defaultMemoryLength
	wBuf     []byte             // 待发送的数据
	rwLock   *sync.RWMutex      // 适用于发送数据时的读写锁
	ctx      context.Context    //
	cancel   context.CancelFunc //
}

func (r *Remote) init(s *FasterServer, index int) {
	r.s, r.si = s, index
	r.ri, r.wi = r.HeaderLen(), r.HeaderLen() // 预留消息头位置
	r.rwLock = &sync.RWMutex{}
	r.ctx, r.cancel = context.WithCancel(s.ctx)

	r.rBuf = make([]byte, r.Cap()) // header: 2 + content:65536
	r.wBuf = make([]byte, r.Cap())
	r.readC, r.procC = make(chan struct{}, 1), make(chan struct{}, 1)
	r.procC <- struct{}{} // 第一步为读操作
}

func (r *Remote) reset() *Remote {
	r.ri, r.wi = r.HeaderLen(), r.HeaderLen() // 预留消息头位置
	r.wBuf, r.rBuf = r.wBuf[:0], r.rBuf[:0]   // 清空数据但保持内存空间不变
	if len(r.procC) == 0 {
		r.procC <- struct{}{} // 第一步为读操作
	}
	if len(r.readC) != 0 {
		<-r.readC
	}

	return r
}

// close 主动关闭与客户端的连接
func (r *Remote) close() error {
	r.reset() // 重置此连接
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *Remote) Ctx() context.Context { return r.ctx }

// RWLock 适用于发送数据时的读写锁
func (r *Remote) RWLock() *sync.RWMutex { return r.rwLock }

// HeaderLen 消息头长度
func (r *Remote) HeaderLen() int { return int(r.s.headerLength) }

// HeaderByteOrder 消息头字节序
func (r *Remote) HeaderByteOrder() string { return r.s.headerByteOrder }

// Conn 获取远端连接对象
// @return net.Conn 远端连接对象
func (r *Remote) Conn() net.Conn { return r.conn }

// RemoteAddr 获取远端链接地址
// @return net.Addr 远端链接地址
func (r *Remote) RemoteAddr() net.Addr { return r.conn.RemoteAddr() }

// Logger 获取日志配置
func (r *Remote) Logger() LoggerIface { return r.s.logger }

func (r *Remote) String() string {
	if r.ri == r.HeaderLen() {
		return "<nil>"
	}
	return string(r.rBuf[r.HeaderLen():r.ri])
}

func (r *Remote) Cap() int { return defaultMemoryCap + r.HeaderLen() }

func (r *Remote) Len() int { return r.ReadLen() }

// ReadLen 获取接收数据的长度
func (r *Remote) ReadLen() int {
	if r.ri >= r.Cap() {
		return 0
	}
	return r.ri - r.HeaderLen() // 当前数据结束下标 - 消息头长度
}

// UnreadLen 获取缓冲区内尚未被读取的数据长度
func (r *Remote) UnreadLen() int {
	if r.lastRead >= r.ri {
		return 0
	}
	return r.ri - r.lastRead
}

// Read 将缓冲区的数据读取到切片p内，并返回实际读取的数据长度，其效果等同于copy，不建议使用
func (r *Remote) Read(p []byte) (int, error) {
	if r.UnreadLen() == 0 { // 没有数据可以读了
		return 0, io.EOF
	}

	i := copy(p, r.rBuf[r.lastRead:r.ri])
	r.lastRead += i // 标记被读取的数据量
	return i, nil
}

// ReadTo 将从TCP中读取到的数据转写到w中
func (r *Remote) ReadTo(w io.Writer) (int, error) {
	if r.UnreadLen() == 0 {
		return 0, io.EOF
	}
	n, err := w.Write(r.rBuf[r.lastRead:r.ri])
	if err != nil {
		return 0, err
	}
	r.lastRead += n
	return n, nil
}

// Copy 将缓冲区的数据拷贝到切片p内，并返回实际读取的数据长度
func (r *Remote) Copy(p []byte) (int, error) { return r.Read(p) }

// Content 获取读到的字节流
func (r *Remote) Content() []byte { return r.rBuf[r.lastRead:r.ri] }

// Write 将切片p中的内容追加到发数据缓冲区内，并返回追加的数据长度;
// 若缓冲区大小不足以写入全部数据，则返回实际写入的数据长度，
// 返回值err恒为nil
func (r *Remote) Write(p []byte) (int, error) {
	r.rwLock.Lock() // 避免 ServerHandler.AcceptHandlerFunc 与 ServerHandler.HandlerFunc 并发操作
	defer r.rwLock.Unlock()

	i := 0
	if len(p)+r.wi > r.Cap() {
		i = r.Cap() - r.wi // 计算可以写入的最大字节数
	} else {
		i = len(p) // 有足够的空间可以全部写入数据
	}
	r.wi += i // 更新未发送数据的结束下标
	return copy(r.wBuf[r.wi:], p[:i]), nil
}

// WriteFrom 从 io.Reader 中读取数据并直接写入发数据缓冲区
func (r *Remote) WriteFrom(re io.Reader) (int, error) {
	r.rwLock.Lock()
	defer r.rwLock.Unlock()

	if defaultMemoryCap-r.wi-r.HeaderLen() > 0 { // 存在剩余空位
		n, err := re.Read(r.wBuf[r.wi:])
		if err != nil {
			return 0, err
		}
		r.wi += n
		return n, nil
	}
	return 0, io.EOF // 缓冲区已满
}

// WriteSeek 设置下一次写的位置
// @param offset int64 相对偏移量
// @param whence int 相对位置,取值有：0相对起始位置;1相对当前位置;2相对结束位置;
// @return int64, error 修正后的相对偏移量，可能的错误
func (r *Remote) WriteSeek(offset int64, whence int) (int64, error) {
	r.rwLock.Lock()
	defer r.rwLock.Unlock()

	switch whence {
	case io.SeekStart:
		r.wi = int(offset)
	case io.SeekCurrent:
		r.wi += int(offset)
	case io.SeekEnd:
		r.wi = defaultMemoryCap + int(offset)
	default:
		return 0, errors.New("tcps.Remote.WriteSeek: invalid whence")
	}

	if r.wi <= r.HeaderLen() {
		r.wi = r.HeaderLen()
	} else if r.wi >= r.Cap() {
		r.wi = r.Cap()
	}

	return int64(r.wi), nil
}

// Drain 提交缓冲区内的数据到TCP客户端
func (r *Remote) Drain() (int, error) {
	r.rwLock.Lock()
	defer r.rwLock.Unlock()

	if r.UnreadLen() == 0 {
		return 0, nil
	}

	// 构造消息头
	if r.HeaderLen() > 0 {
		r.s.packHeaderFunc(r.wBuf[:r.HeaderLen()], uint64(r.wi-r.HeaderLen()))
	}
	n, err := r.conn.Write(r.wBuf[:r.wi])

	if err != nil {
		r.wi += n
		return n, err
	}
	r.wi = r.HeaderLen() // 消息发送完毕

	return n, nil
}

// ---------------------- 服务端 ----------------------

type FasterServer struct {
	Host                string               // TCPs host, default="0.0.0.0"
	Port                string               // TCPs port, default=8090
	MaxOpenConn         int                  // TCP最大连接数量
	headerLength        HeaderLength         // TCP消息头长度，若为0则不包含消息头
	headerByteOrder     string               // TCP消息头字节序，默认大端序，若 headerLength =0则无效
	FasterServerHandler FasterServerHandler  //
	logger              LoggerIface          //
	listener            net.Listener         //
	remotes             []*Remote            // 客户端连接
	isRunning           bool                 // 是否正在运行
	lock                *sync.Mutex          // 连接建立和断开时加锁
	ctx                 context.Context      //
	packHeaderFunc      func([]byte, uint64) //
	unpackHeaderFunc    func([]byte) int     //
}

func (s *FasterServer) init() *FasterServer {
	s.lock = &sync.Mutex{} // 连接建立和断开时加锁
	s.ctx = context.Background()

	// 初始化连接记录池
	s.remotes = make([]*Remote, s.MaxOpenConn)
	// 初始化内存，以避免运行中的内存分配
	small := preDistributionRemoteNums
	if small > s.MaxOpenConn {
		small = s.MaxOpenConn
	}
	for i := 0; i < small; i++ {
		s.remotes[i] = &Remote{}
		s.remotes[i].init(s, i)
	}

	return s
}

func (s *FasterServer) read(r *Remote) error {
	if s.headerLength != 0 {
		_, err := r.conn.Read(r.rBuf[:s.headerLength])
		if err != nil {
			return err
		}
		// ReadFull 会把填充contentBuf填满为止
		n, err := io.ReadFull(r.conn, r.rBuf[s.headerLength:s.unpackHeaderFunc(r.rBuf[:s.headerLength])])
		r.ri += n
		return err
	} else {
		n, err := r.s.FasterServerHandler.Read(r.conn, r.rBuf)
		r.ri += n
		return err
	}
}

// IsRunning 是否正在运行
func (s *FasterServer) IsRunning() bool { return s.isRunning }

// SetMessageHandlerFunc 设置消息处理钩子函数
func (s *FasterServer) SetMessageHandlerFunc(handler FasterServerHandler) *FasterServer {
	s.FasterServerHandler = handler
	return s
}

// GetOpenConnNums 获取当前TCP的连接数量
// @return  int 打开的连接数量
func (s *FasterServer) GetOpenConnNums() (v int) {
	v = 0
	for i := 0; i < s.MaxOpenConn; i++ {
		if s.remotes[i].conn != nil {
			v++
		}
	}

	return
}

// Stop 停止并关闭全部TCP连接
func (s *FasterServer) Stop() {
	if !s.IsRunning() { // 服务未启动
		return
	}

	// 逐个关闭客户端连接
	for i := 0; i < s.MaxOpenConn; i++ {
		if s.remotes[i] != nil {
			_ = s.remotes[i].close()
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
func (s *FasterServer) Serve() error {
	if s.isRunning {
		return errors.New("s is running")
	}
	if s.FasterServerHandler == nil {
		return errors.New("s FasterServerHandler is not define")
	}

	s.init()

	addr := net.JoinHostPort(s.Host, s.Port)
	// 使用 net.Listen 监听连接的地址与端口
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.logger.Error("tcp FasterServer listen fail, ", err.Error())
		return err
	}

	s.listener, s.isRunning = listener, true // 修改TCP运行状态
	s.logger.Info("tcp FasterServer listening on ", addr)

	for {
		conn, err := listener.Accept() // 等待连接
		if err != nil {
			s.logger.Warn("connection accept fail, ", err.Error())
			continue
		}

		if s.GetOpenConnNums() >= s.MaxOpenConn { // 达到最大连接数量限制
			s.logger.Warn("the connection number reached the upper limit.")
			_ = conn.Close()
			continue
		}

		// 为客户端建立连接, 此时一定存在空闲槽位
		//
		s.lock.Lock() // 建立连接时禁止并发
		for i := 0; i < s.MaxOpenConn; i++ {
			if s.remotes[i] == nil { // 此时连接槽未初始化
				s.remotes[i] = &Remote{}
				s.remotes[i].init(s, i)
			}

			if s.remotes[i].conn != nil { // 连接已占用
				continue
			}

			// 发现空闲槽位，接受客户端连接
			s.remotes[i].conn = conn
			go fasterProcess(s.remotes[i]) // 对每个新连接创建一个协程进行连接处理
			break
		}
		s.lock.Unlock()
	}
}

// fasterProcess 处理TCP连接
func fasterProcess(r *Remote) {
	defer func() {
		_ = r.close()
		r.s.FasterServerHandler.ClosedHandlerFunc(r)
		r.cancel()
		r.conn = nil // 必须先执行钩子函数后再删除客户端连接，避免在 ClosedHandlerFunc 中无法获取远端地址
	}()

	// 处理连接时任务
	go r.s.FasterServerHandler.AcceptHandlerFunc(r)

	// 读取数据
	go func() {
		for {
			select {
			case <-r.ctx.Done():
				break
			case <-r.procC: // 上一包消息已处理完
				err := r.s.read(r)
				if err != nil {
					r.s.logger.Warn(
						"read message from: " + r.conn.RemoteAddr().String() + "failed, " + err.Error(),
					)
				}
				r.readC <- struct{}{}
			}
		}
	}()

	for { // 处理通信中任务
		//
		// ******************** 处理过程 ********************
		//
		<-r.readC
		r.s.FasterServerHandler.HandlerFunc(r)
		r.procC <- struct{}{} // 立刻开始读取下一包数据
		_, err := r.Drain()   // 发送数据
		if err != nil {       // 消息发送失败，断开连接
			break
		}
	}
}

// NewAsyncFasterTcpServer 创建一个新的TCP FasterServer
//
// TCP s 此处已做消息处理的解构，默认提供了一个没啥卵用的消息处理方法：TcpMessageHandler；
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
//			FasterServerHandler: &TCPHandler{},
//			ByteOrder:      "big",
//			logger:         logger.ConsoleLogger{},
//		},
//	)
func NewAsyncFasterTcpServer(c ...*FasterTcpsConfig) *FasterServer {
	var s *FasterServer

	if len(c) == 0 {
		s = &FasterServer{
			Host:                defaultFasterTcpsConfig.Host,
			Port:                defaultFasterTcpsConfig.Port,
			MaxOpenConn:         defaultFasterTcpsConfig.MaxOpenConn,
			headerLength:        defaultFasterTcpsConfig.HeaderLength,
			headerByteOrder:     defaultFasterTcpsConfig.ByteOrder,
			FasterServerHandler: defaultFasterTcpsConfig.ServerHandler,
			logger:              defaultFasterTcpsConfig.Logger,
			isRunning:           false,
		}
	} else {
		s = &FasterServer{
			Host:                c[0].Host,
			Port:                c[0].Port,
			MaxOpenConn:         c[0].MaxOpenConn,
			headerLength:        c[0].HeaderLength,
			headerByteOrder:     c[0].ByteOrder,
			FasterServerHandler: c[0].ServerHandler,
			logger:              c[0].Logger,
			isRunning:           false,
		}
	}

	if s.headerByteOrder == "" {
		s.headerByteOrder = tcpByteOrder
	}

	switch s.headerLength {
	case 0:
		s.headerLength = defaultHeaderLength
	case ZeroBytes:
		s.headerLength = 0 // 禁用消息头
	default:
		if s.headerLength > EightBytes { // 超出最大限制
			s.headerLength = EightBytes
		}
	}

	if s.headerByteOrder == tcpByteOrder {
		switch s.headerLength {
		case ZeroBytes:
			s.packHeaderFunc = func([]byte, uint64) {}
			s.unpackHeaderFunc = func([]byte) int { return 0 }
		case TwoBytes:
			s.packHeaderFunc = func(b []byte, v uint64) {
				binary.BigEndian.PutUint16(b, uint16(v))
			}
			s.unpackHeaderFunc = func(p []byte) int {
				return int(binary.BigEndian.Uint16(p))
			}
		case FourBytes:
			s.packHeaderFunc = func(b []byte, v uint64) {
				binary.BigEndian.PutUint32(b, uint32(v))
			}
			s.unpackHeaderFunc = func(p []byte) int {
				return int(binary.BigEndian.Uint32(p))
			}
		case EightBytes:
			s.packHeaderFunc = binary.BigEndian.PutUint64
			s.unpackHeaderFunc = func(p []byte) int {
				return int(binary.BigEndian.Uint64(p))
			}
		}
	} else {
		switch s.headerLength {
		case ZeroBytes:
			s.packHeaderFunc = func([]byte, uint64) {}
			s.unpackHeaderFunc = func([]byte) int { return 0 }
		case TwoBytes:
			s.packHeaderFunc = func(b []byte, v uint64) {
				binary.LittleEndian.PutUint16(b, uint16(v))
			}
			s.unpackHeaderFunc = func(p []byte) int {
				return int(binary.LittleEndian.Uint16(p))
			}
		case FourBytes:
			s.packHeaderFunc = func(b []byte, v uint64) {
				binary.LittleEndian.PutUint32(b, uint32(v))
			}
			s.unpackHeaderFunc = func(p []byte) int {
				return int(binary.LittleEndian.Uint32(p))
			}
		case EightBytes:
			s.packHeaderFunc = binary.LittleEndian.PutUint64
			s.unpackHeaderFunc = func(p []byte) int {
				return int(binary.LittleEndian.Uint64(p))
			}
		}
	}

	go func() {
		if err := s.Serve(); err != nil {
			s.logger.Error("TCP FasterServer started failed: ", err.Error())
		} else {
			s.logger.Info("TCP FasterServer Listen on: ", s.listener.Addr().String())
		}
	}()

	return s
}
