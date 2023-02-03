package tcp

import (
	"log"
)

// LoggerIface 自定义logger接口，log及zap等均已实现此接口
type LoggerIface interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
}

// ServerHandler 服务端处理程序
type ServerHandler interface {
	OnAccepted(r *Remote) error // 当客户端连接时触发的操作，如果此操作耗时较长,则应手动创建一个协程进行处理
	OnClosed(r *Remote) error   // 当客户端断开连接时触发的操作, 此操作执行完成之后才会释放与 Remote 的连接
	Handler(r *Remote) error    // 处理接收到的消息, 当 OnAccepted 处理完成时,会循环执行 readMessage -> Handler
}

// ClientHandler 客户端处理程序
type ClientHandler interface {
	ServerHandler
}

type DefaultLogger struct {
	logger *log.Logger
}

func (l *DefaultLogger) Debug(args ...any) { l.logger.Panicln(args...) }
func (l *DefaultLogger) Info(args ...any)  { l.logger.Panicln(args...) }
func (l *DefaultLogger) Warn(args ...any)  { l.logger.Panicln(args...) }
func (l *DefaultLogger) Error(args ...any) { l.logger.Panicln(args...) }

func NewDefaultLogger() *DefaultLogger { return &DefaultLogger{logger: log.Default()} }
