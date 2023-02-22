package logger

import (
	"io"
	"log"
	"os"
)

// Iface 自定义logger接口，log及zap等均已实现此接口
type Iface interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
}

// FIface 适用于 fmt.Errorf 风格的日志接口
type FIface interface {
	Errorf(format string, args ...any)
	Warnf(format string, args ...any)
	Debugf(format string, args ...any)
}

type DefaultLogger struct {
	logger *log.Logger
}

func (l *DefaultLogger) Debug(args ...any) { l.logger.Println("\u001B[35mDEBUG\u001B[0m", args) }
func (l *DefaultLogger) Info(args ...any)  { l.logger.Println("\u001B[34mINFO\u001B[0m", args) }
func (l *DefaultLogger) Warn(args ...any)  { l.logger.Println("\u001B[33mWARN\u001B[0m", args) }
func (l *DefaultLogger) Error(args ...any) { l.logger.Println("\u001B[31mERROR\u001B[0m", args) }

func NewLogger(out io.Writer, prefix string, flag int) *DefaultLogger {
	return &DefaultLogger{logger: log.New(out, prefix, flag)}
}

func NewDefaultLogger() *DefaultLogger {
	return NewLogger(os.Stderr, "", log.LstdFlags|log.Lshortfile)
}
