package tcp

const tcpByteOrder = "big"
const headerLength = 2        // 消息头长度
const defaultMaxOpenConn = 10 //

// HandlerFunc 消息处理程序
type HandlerFunc interface {
	OnAccepted(r *Remote) error // 当客户端连接时触发的操作，如果此操作耗时较长,则应手动创建一个协程进行处理
	OnClosed(r *Remote) error   // 当客户端断开连接时触发的操作, 此操作执行完成之后才会释放与 Remote 的连接
	Handler(r *Remote) error    // 处理接收到的消息, 当 OnAccepted 处理完成时,会循环执行 Read -> Handler
}
