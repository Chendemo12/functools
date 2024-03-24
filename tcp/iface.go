package tcp

const tcpByteOrder = "big"
const headerLength = 2         // 消息头长度
const defaultMaxOpenConn = 100 //

// HandlerFunc 消息处理程序
type HandlerFunc interface {
	OnAccepted(r *Remote) error // 当客户端连接时触发的操作，如果此操作耗时较长,则应手动创建一个协程进行处理
	OnClosed(r *Remote) error   // 当客户端断开连接时触发的操作, 此操作执行完成之后才会清空远端地址
	Handler(r *Remote) error    // 处理接收到的消息, 当 OnAccepted 处理完成时,会循环执行 Read -> Handler, 返回nil则表示处理成功，会调用 Remote.Drain 方法发送数据（如果有）,当然也可以自行调用 Drain 发送数据
}
