package zaplog

import (
	"fmt"
	"github.com/Chendemo12/functools/cprint"
	"github.com/Chendemo12/functools/helper"
	"github.com/Chendemo12/functools/python"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

const timeformat = "2006/01/02 15:04:05"

var console *zap.Logger // 用于仅控制台输出的日志

func init() {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(timeformat) //指定时间格式
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder       //设置彩色输出

	fileCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
		zap.DebugLevel, // 设置日志级别
	)

	console = zap.New(fileCore, zap.AddCaller(), zap.AddCallerSkip(1))
}

// ConsoleLogger 控制台日志
/*
此自定义日志仅输出到控制台，用于解决生产环境下无法输出一些调试信息，自定义日志需实现CustomLoggerIface接口
	SDebug()方法仅支持输入字符串，速度更好
	FDebug()格式化输出，类似于python.__repr__() 输出格式，内部对struct,map,array,string,error实现了string的格式化转换
	Debug()方式支持多种类型，内部采用fmt.Sprint(args...)方法进行转换

FDebug()仅支持2个必须参数：消息描述，打印对象，对于其他类型可能引发异常
	array -> "0d 56 f6";
	map -> json;
	struct, pointer -> Configuration({"Base":{"host":"0.0.0.0","port":8081,"title":"flying"}});
	error -> error.Error();

Usage:
	logger := ConsoleLogger{}
	logger.SSucc("Started")
	err := errors.New("connection closed")
	logger.Debug("err, ", err)
	logger.FDebug("config", config)
*/
type ConsoleLogger struct{}

func (l ConsoleLogger) Sync() error { return nil }

func (l ConsoleLogger) Debug(args ...any) {
	console.Debug(helper.CombineStrings(cprint.Deb, fmt.Sprint(args...), cprint.End))
}

func (l ConsoleLogger) Info(args ...any) {
	console.Info(helper.CombineStrings(cprint.Inf, fmt.Sprint(args...), cprint.End))
}

func (l ConsoleLogger) Warn(args ...any) {
	console.Warn(helper.CombineStrings(cprint.War, fmt.Sprint(args...), cprint.End))
}

func (l ConsoleLogger) Error(args ...any) {
	console.Error(helper.CombineStrings(cprint.Ero, fmt.Sprint(args...), cprint.End))
}

func (l ConsoleLogger) Succ(args ...any) {
	console.Debug(helper.CombineStrings(cprint.Suc, fmt.Sprint(args...), cprint.End))
}

func (l ConsoleLogger) Errorf(format string, v ...any) { l.SError(fmt.Errorf(format, v...).Error()) }

func (l ConsoleLogger) Warnf(format string, v ...any) { l.SWarn(fmt.Errorf(format, v...).Error()) }

func (l ConsoleLogger) Debugf(format string, v ...any) { l.SDebug(fmt.Errorf(format, v...).Error()) }

// SDebug == only string is accepted, so it is faster than Debug

func (l ConsoleLogger) SDebug(msg string) {
	console.Debug(helper.CombineStrings(cprint.Deb, msg, cprint.End))
}

func (l ConsoleLogger) SInfo(msg string) {
	console.Info(helper.CombineStrings(cprint.Inf, msg, cprint.End))
}

func (l ConsoleLogger) SWarn(msg string) {
	console.Warn(helper.CombineStrings(cprint.War, msg, cprint.End))
}

func (l ConsoleLogger) SError(msg string) {
	console.Error(helper.CombineStrings(cprint.Ero, msg, cprint.End))
}

func (l ConsoleLogger) SSucc(msg string) {
	console.Debug(helper.CombineStrings(cprint.Suc, msg, cprint.End))
}

func (l ConsoleLogger) FDebug(message string, object any) {
	console.Debug(helper.CombineStrings(cprint.Deb, message, python.Repr(object), cprint.End))
}

func (l ConsoleLogger) FInfo(message string, object any) {
	console.Info(helper.CombineStrings(cprint.Inf, message, python.Repr(object), cprint.End))
}

func (l ConsoleLogger) FWarn(message string, object any) {
	console.Warn(helper.CombineStrings(cprint.War, message, python.Repr(object), cprint.End))
}

func (l ConsoleLogger) FError(message string, object any) {
	console.Error(helper.CombineStrings(cprint.Ero, message, python.Repr(object), cprint.End))
}

func (l ConsoleLogger) FSucc(message string, object any) {
	console.Debug(helper.CombineStrings(cprint.Suc, message, python.Repr(object), cprint.End))
}
