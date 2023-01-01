package client

import (
	"context"
	"net"
	"strconv"

	"github.com/go-redis/redis/v8"
)

// NewRedis 创建一个Redis数据库连接
// @param  host  string  主机地址
// @param  port  int     端口
// @param  pwd   string  数据库密码
// @param  db    int     数据库编号
func NewRedis(host string, port int, pwd string, db int) *redis.Client {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pwd,
		DB:       db,
	})

	return rdb
}

// NewDefaultRedis 创建一个默认的Redis数据库连接
// @param  host  string  主机地址
// @param  port  int     端口
func NewDefaultRedis(host string, port int) *redis.Client {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default db
	})

	return rdb
}

// NewSentinelRedis 创建一个redis哨兵节点
// @param   mastername   string    主节点名称
// @param   password     string    主节点密码
// @param   addrs        []string  集群地址
// @param   db           int       数据库编号
// @param   contextback  context.Context
// @return  *redis.Client redis 客户端
func NewSentinelRedis(mastername, password string, addrs []string, db int, contextback context.Context) *redis.Client {
	sf := &redis.FailoverOptions{
		MasterName:    mastername,
		SentinelAddrs: addrs,
		Password:      password,
		DB:            db,
	}
	sentinelClient := redis.NewFailoverClient(sf)
	res := sentinelClient.Ping(contextback)
	if res.Err() != nil {
		panic(res.Err())
	}
	return sentinelClient
}
