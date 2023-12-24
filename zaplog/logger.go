package zaplog

import (
	"github.com/Chendemo12/functools/python"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"strconv"
	"time"
)

const timeformat = time.DateTime

const (
	DEBUG    = zapcore.DebugLevel
	INFO     = zapcore.InfoLevel
	WARNING  = zapcore.WarnLevel
	ERROR    = zapcore.ErrorLevel
	TRACE    = zapcore.DPanicLevel
	CRITICAL = zapcore.FatalLevel
)

const (
	End   = "\u001B[0m"
	EndLn = "\u001B[0m\n"
	Ero   = "\u001B[31m"
	Suc   = "\u001B[32m"
	War   = "\u001B[33m"
	Inf   = "\u001B[34m"
	Deb   = "\u001B[35m" // 紫红色
)

var (
	defaultConfig = &Config{
		Filename:   "runtime",
		Level:      WARNING,
		Rotation:   3,
		Retention:  7,
		MaxBackups: 10,
		Compress:   true,
	}
	instances     []*Config   // 存储全部日志句柄
	defaultLogger *zap.Logger = nil
)

func init() {
	defaultLogger = NewLogger(defaultConfig)
	zap.ReplaceGlobals(defaultLogger) // 配置 zap 包的全局变量
}

type Config struct {
	Filename   string        `json:"filename,omitempty"` // 日志文件名，默认runtime
	Level      zapcore.Level `json:"level,omitempty"`    // 日志级别，默认warning
	Rotation   int           `json:"rotation"`           // 单个日志文件的最大大小，单位MB
	Retention  int           `json:"retention"`          // 单个日志的最大保存时间，单位Day
	MaxBackups int           `json:"max_backups"`        // 日志文件最大保留数量
	Compress   bool          `json:"compress"`           // 是否启用压缩，默认true
	logger     *zap.Logger
}

func (l *Config) String() string { return l.Filename + strconv.Itoa(int(l.Level)) }

func (l *Config) Logger() *zap.Logger { return l.logger }

func (l *Config) Sugar() *zap.SugaredLogger { return l.Logger().Sugar() }

func (l *Config) ResetLevel(level zapcore.Level) *Config {
	return l
}

// NewLogger 创建一个新的日志句柄
func NewLogger(c ...*Config) *zap.Logger {
	var conf *Config
	if len(c) < 1 {
		conf = defaultConfig
	} else {
		conf = c[0]
	}

	// 首先判断是否已经存在此配置下的日志句柄
	for i := 0; i < len(instances); i++ {
		if instances[i].String() == conf.String() {
			return instances[i].logger
		}
	}

	logger := newLogger(conf)
	conf.logger = logger
	instances = append(instances, conf)

	return logger
}

// GetLogger 依据文件名查询日志句柄
//
//	@param	filename	string	日志文件名
//	@param	deft		[]bool	是否在未找到日志句柄时返回默认的日志句柄
func GetLogger(filename string, deft ...bool) *zap.SugaredLogger {
	// 首先判断是否已经存在此配置下的日志句柄
	for i := 0; i < len(instances); i++ {
		if instances[i].Filename == filename {
			return instances[i].logger.Sugar()
		}
	}

	if python.Any(deft...) {
		return defaultLogger.Sugar()
	}

	return nil
}

func newLogger(conf *Config) *zap.Logger {
	// 设置日志级别
	zapLevel := zap.NewAtomicLevelAt(conf.Level)
	// 文件writeSyncer
	fileWriteSyncer := zapcore.AddSync(
		&lumberjack.Logger{
			Filename:   "./logs/" + conf.Filename + ".log", // 日志文件存放目录
			MaxSize:    conf.Rotation,                      // 文件大小限制,单位MB
			MaxBackups: conf.MaxBackups,                    // 最大保留日志文件数量
			MaxAge:     conf.Retention,                     // 日志文件保留天数
			Compress:   conf.Compress,                      // 是否压缩处理
		},
	)

	var fileCore zapcore.Core
	if conf.Level < WARNING { // 开发模式的调试日志, console文本模式输出
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(timeformat) // 指定时间格式
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder       // 设置彩色输出

		fileCore = zapcore.NewCore(
			// 获取编码器,NewJSONEncoder()输出json格式，NewConsoleEncoder()输出普通文本格式
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(fileWriteSyncer, zapcore.AddSync(os.Stdout)),
			// 第三个及之后的参数为写入文件的日志级别,ErrorLevel模式只记录error级别的日志
			zapLevel,
		)
	} else { // 设置生产环境，json模式输出
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(timeformat) // 指定时间格式
		fileCore = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(fileWriteSyncer, zapcore.AddSync(os.Stdout)),
			zapLevel,
		)
	}

	return zap.New(fileCore, zap.AddCaller()) // AddCaller()为显示文件名和行号
}
