package tcp

import (
	"encoding/binary"
	"errors"
	"github.com/Chendemo12/functools/logger"
	"io"
	"net"
	"sync"
)

// Remote 对端链接
type Remote struct {
	conn      net.Conn
	logger    logger.Iface
	lock      *sync.Mutex
	addr      string
	byteOrder string
	rx        []byte `description:"接收缓冲区"`
	tx        []byte `description:"发送缓冲区"`
	index     int    `description:"当前连接在Server中的位置"`
	lastRead  int    `description:"上一次读取的结束位置"`
	rxEnd     int    `description:"接收到的数据结束位置"`
	txEnd     int    `description:"待发送的数据结束位置"`
}

func (r *Remote) Addr() string         { return r.addr }
func (r *Remote) String() string       { return r.Addr() }
func (r *Remote) Logger() logger.Iface { return r.logger }
func (r *Remote) Cap() int             { return bufLength - headerLength }

// Index 当前连接在 Server 中的位置, Client 无效
func (r *Remote) Index() int { return r.index }

// IsConnected 是否已连接
func (r *Remote) IsConnected() bool { return r.conn != nil }

// Close 关闭与对端的连接
func (r *Remote) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// Len 获取接收数据的总长度
func (r *Remote) Len() int {
	if r.rxEnd == headerLength {
		return 0
	}
	return r.parseHeader()
}

// Read 将缓冲区的数据读取到切片buf内，并返回实际读取的数据长度
func (r *Remote) Read(buf []byte) (int, error) {
	if s := r.Unread(); len(s) == 0 { // 没有数据可以读了
		return 0, io.EOF
	} else {
		i := copy(buf, s)
		r.lastRead += i // 标记被读取的数据量
		return i, nil
	}
}

// ReadN 读取N个字节的数据, 若未读取的数据不足N字节，则返回全部未读取的数据
func (r *Remote) ReadN(n int) []byte {
	if r.lastRead >= r.rxEnd { // 无数据可读
		return empty
	}
	if r.lastRead+n > r.rxEnd {
		b := r.rx[r.lastRead:r.rxEnd]
		r.lastRead = r.rxEnd
		return b
	} else {
		b := r.rx[r.lastRead : r.lastRead+n]
		r.lastRead += n
		return b
	}
}

// Copy 将缓冲区的数据拷贝到切片p内，并返回实际读取的数据长度
func (r *Remote) Copy(p []byte) (int, error) { return r.Read(p) }

// Content 获取读到的字节流, 可以重复调用
func (r *Remote) Content() []byte {
	if r.rxEnd == headerLength {
		return empty
	}
	return r.rx[headerLength:r.rxEnd]
}

// Unread 获取未读取的消息流
func (r *Remote) Unread() []byte {
	return r.rx[r.lastRead:r.rxEnd]
}

// Write 将切片buf中的内容追加到发数据缓冲区内，并返回追加的数据长度;
// 若缓冲区大小不足以写入全部数据，则返回实际写入的数据长度和错误消息
func (r *Remote) Write(buf []byte) (int, error) {
	r.lock.Lock() // 避免 HandlerFunc.OnAccepted 与 HandlerFunc.HandlerFunc 并发操作
	defer r.lock.Unlock()

	i := copy(r.tx[r.txEnd:], buf)
	r.txEnd += i // 更新未发送数据的结束下标

	// Write must return a non-nil error if it returns n < len(p).
	// Write must not modify the slice data, even temporarily.
	if i < len(buf) {
		return i, io.ErrShortWrite
	}

	return i, nil
}

// Seek Writer Seek
func (r *Remote) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset <= 0 {
			r.txEnd = headerLength
		} else {
			r.txEnd = headerLength + int(offset)
		}
	case io.SeekCurrent:
		r.txEnd += int(offset)
		if r.txEnd < headerLength {
			r.txEnd = headerLength
		}
		if r.txEnd > bufLength {
			r.txEnd = bufLength
		}
	case io.SeekEnd:
		if offset < 0 && int(offset) >= -(bufLength-headerLength) {
			r.txEnd += int(offset)
		}
	}
	return int64(r.txEnd - headerLength), nil
}

// Drain 将缓冲区的数据发生到客户端, 并进行消息头封装
func (r *Remote) Drain() error {
	r.lock.Lock()
	defer r.lock.Unlock() // 避免在 Drain 时 Write

	if r.txEnd == headerLength {
		return nil // 没有需要发送的数据
	}

	r.makeHeader() // 构造消息头
	i, err := r.conn.Write(r.tx[:r.txEnd])

	r.txEnd -= i - headerLength // 重置消息头游标

	return err
}

func (r *Remote) reset() {
	r.conn, r.addr = nil, ""
	r.lastRead = headerLength
	r.rxEnd, r.txEnd = headerLength, headerLength // 重置游标
	for i := 0; i < headerLength; i++ {
		r.rx[i] = 0
	}
}

func (r *Remote) makeHeader() {
	if r.byteOrder == "big" {
		binary.BigEndian.PutUint16(r.tx, uint16(r.txEnd-headerLength))
	} else {
		binary.LittleEndian.PutUint16(r.tx, uint16(r.txEnd-headerLength))
	}
}

func (r *Remote) parseHeader() int {
	if r.byteOrder == "big" {
		return int(binary.BigEndian.Uint16(r.rx[:headerLength]))
	} else {
		return int(binary.LittleEndian.Uint16(r.rx[:headerLength]))
	}
}

// 从 net.Conn 中读取数据到缓冲区内
func (r *Remote) readMessage() error {
	// 获取消息长度
	n, err := r.conn.Read(r.rx[0:headerLength])
	if err != nil {
		return err
	}
	if n != headerLength {
		return errors.New("the message header is incomplete")
	}
	// 接收数据
	n, err = io.ReadFull(r.conn, r.rx[headerLength:r.parseHeader()+2]) // ReadFull 会把填充buf填满为止

	if err != nil {
		return err
	}
	r.lastRead = headerLength // 游标重置
	r.rxEnd = n + headerLength

	return nil
}
