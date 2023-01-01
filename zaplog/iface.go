package zaplog

// Iface 自定义logger接口，log及zap等均已实现此接口
type Iface interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Sync() error
}

// FIface 适用于 fmt.Errorf 风格的日志接口
type FIface interface {
	Errorf(format string, args ...any)
	Warnf(format string, args ...any)
	Debugf(format string, args ...any)
}

type AllIface interface {
	Iface
	FIface
}
