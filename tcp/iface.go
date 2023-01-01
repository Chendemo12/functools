package tcp

import (
	"encoding/binary"
	"errors"
	"gitlab.cowave.com/gogo/functools/helper"
	"net"
	"strconv"
)

// LoggerIface 自定义logger接口，log及zap等均已实现此接口
type LoggerIface interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Sync() error
}

type ServerHandler interface {
	AcceptHandlerFunc(conn net.Conn) // 当客户端连接时触发的操作，如需在此协程内向client发送消息，则必须进行TCPMessage的封装，可通过调用SendMessageToClient()实现
	HandlerFunc(msg *Frame) *Frame   // 处理接收到的消息
	SendMessage(msg *Frame) error    // 主动向客户端发送消息，进行TCPMessage封装
	ReadMessage(msg *Frame) error    // 读取数据，允许自定义读物方法
}

type MessageIface interface {
	Pack() []byte             // 打包消息
	Unpack(data []byte) error // 消息流解包
	GetConn() net.Conn        // 获取对端连接
	GetLength() uint16        // 获取消息长度
	GetContent() []byte       // 获取消息体
	String() string           // 转换消息为string类型
}

// MessageHeaderUnpack 解析消息头
func MessageHeaderUnpack(order string, data []byte) uint16 {
	if order == "little" {
		return binary.LittleEndian.Uint16(data[:2])
	}
	return binary.BigEndian.Uint16(data[:2])
}

func MessageHeaderPack(order string, length uint16) []byte {
	buf := make([]byte, 2)
	if order == "little" {
		binary.LittleEndian.PutUint16(buf, length)
	} else {
		binary.BigEndian.PutUint16(buf, length)
	}

	return buf
}

type Frame struct {
	Conn      net.Conn // 对端连接
	Content   []byte   // 消息内容
	Length    uint16   // 消息长度
	ByteOrder string   // 消息头字节序
}

func (m *Frame) Pack() []byte {
	m.Length = uint16(len(m.Content))
	lengthBuf := MessageHeaderPack(m.ByteOrder, m.Length)
	buf := append(lengthBuf, m.Content...)
	return buf
}

func (m *Frame) Unpack(data []byte) error {
	if len(data) <= 2 {
		return errors.New("the message Length is less than 2 bytes")
	}
	m.Length = MessageHeaderUnpack(m.ByteOrder, data[:2])
	m.Content = data[2:]
	return nil
}

// String 转换消息体为字符串形式
func (m *Frame) String() string {
	return string(m.Content)
}

// Map 转换为MAP
func (m *Frame) Map() map[string]any {
	js := make(map[string]any)
	js["Length"] = m.Length
	js["Content"] = m.Content
	return js
}

// Repr 格式化输出
func (m *Frame) Repr() string {
	return helper.CombineStrings(
		"Frame",
		"({Length=", strconv.Itoa(int(m.Length)),
		", Content=", helper.HexBeautify(m.Content),
		")",
	)
}

// Hex 十六进制显示消息体
func (m *Frame) Hex() string {
	if m.Length <= 2 {
		return ""
	}
	return helper.HexBeautify(m.Content)
}

func (m *Frame) GetConn() net.Conn {
	if m == nil {
		return nil
	}
	return m.Conn
}
