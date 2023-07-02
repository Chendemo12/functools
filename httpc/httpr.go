// Package httpc HTTP请求客户端
package httpc

import (
	"bytes"
	"context"
	"errors"
	"github.com/Chendemo12/fastapi-tool/helper"
	"github.com/Chendemo12/functools/logger"
	"net/http"
	"strings"
	"time"
)

type Opt struct {
	Url            string
	ContextType    string
	Query          map[string]string
	RequestModel   any
	ResponseModel  any
	DisableMarshal bool
	Timeout        time.Duration
	Ctx            context.Context
	Err            error
	StatusCode     int
}

// Httpr http 请求客户端
// 请求地址默认前缀为"/api", 若不是,需在实例化结构体之后，通过"SetUrlPrefix()"显式更改路由前缀
// 请求的默认超时时间15s, 通过"SetTimeout()"显式更改
type Httpr struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	client  *http.Client
	logger  logger.Iface
	prefix  string
	timeout time.Duration
	i       bool
}

// makePrefix组合前缀，形如："http://127.0.0.1:3306/api/"
//
//	@param	prefix	string	地址前缀
func (h *Httpr) makePrefix(prefix string) *Httpr {
	// noinspection HttpUrlsUsage
	h.prefix = helper.F(
		"http://", h.Host, ":", h.Port, "/", strings.TrimPrefix(prefix, "/"), "/",
	)

	return h
}

// SetTimeout 设置请求的超时时间，单位s
//
//	@param	timeout	int	超时时间(s)
func (h *Httpr) SetTimeout(timeout int) *Httpr {
	h.timeout = time.Duration(timeout) * time.Second
	return h
}

// SetLogger 自定义请求日志
func (h *Httpr) SetLogger(logger logger.Iface) *Httpr {
	h.logger = logger
	return h
}

// SetUrlPrefix 设置地址前缀, 形如"/api"这样的路由前缀，以"/"开头并不以"/"结尾
//
//	@param	prefix	string	地址前缀
func (h *Httpr) SetUrlPrefix(prefix string) *Httpr {
	// 重写现有的路由前缀
	h.makePrefix(strings.TrimSuffix(prefix, "/"))
	return h
}

func (h *Httpr) cleanOpt(opts ...Opt) *Opt {
	return &Opt{}
}

// DoRequest 发起网络请求
//
//	@param	method	string				请求方法，取值为GET/POST/PATCH/PUT/DELETE
//	@param	url		string				路由地址，"url"形如"/tunnels/2"，以"/"开头并不以"/"结尾
func (h *Httpr) DoRequest(method, url string, opts ...Opt) *Opt {
	opt := h.cleanOpt()
	var req *http.Request
	var reqBody []byte
	var err error

	switch opt.RequestModel.(type) {
	case []byte:
		reqBody = opt.RequestModel.([]byte)
	default:
		reqBody, err = helper.JsonMarshal(opt.RequestModel)
	}
	if err != nil {
		opt.StatusCode = 0
		opt.Err = errors.New("marshal body error")
		return opt
	}

	switch method {
	case http.MethodPost:
		req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodPost, opt.Url, bytes.NewReader(reqBody))
	case http.MethodPut:
		req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodPut, opt.Url, bytes.NewReader(reqBody))
	case http.MethodPatch:
		req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodPatch, opt.Url, bytes.NewReader(reqBody))
	case http.MethodDelete:
		req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodDelete, opt.Url, nil)
	default:
		req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodGet, opt.Url, nil)
	}
	for range opt.Query {

	}
	h.client.Do(req)

	return opt
}

// Get 发起一个带查询参数的Get请求
//
//	@param	url		string				路由地址，"url"形如"/tunnels/2"，以"/"开头并不以"/"结尾
func (h *Httpr) Get(url string, opts ...Opt) *Opt {
	return h.DoRequest("GET", url, opts...)
}

// Post 发起一个Post请求
//
//	@param	url		string			路由地址
func (h *Httpr) Post(url string, opts ...Opt) *Opt {
	return h.DoRequest("POST", url, opts...)
}

// Put 发起一个Put请求
//
//	@param	url		string			路由地址
func (h *Httpr) Put(url string, opts ...Opt) *Opt {
	return h.DoRequest("PUT", url, opts...)
}

// Patch 发起一个Patch请求
//
//	@param	url		string			路由地址
func (h *Httpr) Patch(url string, opts ...Opt) *Opt {
	return h.DoRequest("PATCH", url, opts...)
}

// Delete  发起一个Delete请求
//
//	@param	url	string	路由地址
func (h *Httpr) Delete(url string, opts ...Opt) *Opt {
	return h.DoRequest("DELETE", url, opts...)
}

// NewHttpr 创建一个新的HTTP请求基类
//
//	#  Usage:
//
//	client := NewHttpr("localhost", "8080", 5, logger)
//	resp, err := client.Get("/clipboard", map[string]string{})
//	if err != nil {
//		return "", err
//	}
//	text := ""
//	err = client.UnmarshalJson(resp, &text)
//	if err != nil {
//		return "", err
//	}
//	return text, nil
func NewHttpr(host, port string, timeout int, logger logger.Iface) (*Httpr, error) {
	if host == "" || port == "" {
		return nil, errors.New("host and port cannot be empty")
	}

	r := &Httpr{Host: host, Port: port, client: &http.Client{}}
	r.SetTimeout(timeout).SetLogger(logger)

	return r, nil
}
