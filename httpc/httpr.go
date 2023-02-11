// Package httpc HTTP请求客户端
package httpc

import (
	"errors"
	"fmt"
	"github.com/Chendemo12/functools/helper"
	"github.com/Chendemo12/functools/zaplog"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

// HttprError 'imroc/req/v3'定义的错误消息结构
type HttprError struct {
	Message string `json:"message"`
}

func (e HttprError) String() string {
	return e.Message
}

// 重写req的日志接口，替换成统一的 zaplog.Iface
type httprLogger struct {
	logger zaplog.Iface
}

func (l *httprLogger) Errorf(format string, v ...any) {
	l.logger.Error(fmt.Errorf(format, v...).Error())
}

func (l *httprLogger) Warnf(format string, v ...any) {
	l.logger.Warn(fmt.Errorf(format, v...).Error())
}

func (l *httprLogger) Debugf(format string, v ...any) {
	l.logger.Debug(fmt.Errorf(format, v...).Error())
}

// Httpr http 请求客户端
// 请求地址默认前缀为"/api", 若不是,需在实例化结构体之后，通过"SetUrlPrefix()"显式更改路由前缀
// 请求的默认超时时间15s, 通过"SetTimeout()"显式更改
// HTTP请求基于req3第三方库，完整文档详见：https://req.cool/zh/docs/prologue/introduction/
//
//	# Usage:
//
//	client := &Httpr{Host: "localhost", Port: 8080}
//	client.SetTimeout(5).SetLogger(logger).NewClient()
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
type Httpr struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	client  *req.Client
	logger  *httprLogger
	prefix  string
	timeout time.Duration
	i       bool
}

// makePrefix组合前缀，形如："http://127.0.0.1:3306/api/"
//	@param	prefix	string	地址前缀
func (h *Httpr) makePrefix(prefix string) *Httpr {
	// noinspection HttpUrlsUsage
	h.prefix = helper.CombineStrings(
		"http://", h.Host, ":", h.Port, "/", strings.TrimPrefix(prefix, "/"), "/",
	)

	return h
}

func (h *Httpr) init() *Httpr {
	if h.timeout <= time.Duration(0) {
		h.timeout = time.Duration(15) * time.Second
	}
	h.client = req.C().
		SetTimeout(h.timeout).
		SetLogger(h.logger).
		SetJsonMarshal(helper.DefaultJsonMarshal).
		SetJsonUnmarshal(helper.DefaultJsonUnmarshal)
	if h.prefix == "" { // 允许后期重新创建客户端，而不覆盖路由前缀
		h.makePrefix("/api")
	}
	h.client.SetBaseURL(h.prefix)
	h.i = true
	return h
}

// SetTimeout 设置请求的超时时间，单位s
//	@param	timeout	int	超时时间(s)
func (h *Httpr) SetTimeout(timeout int) *Httpr {
	h.timeout = time.Duration(timeout) * time.Second
	return h
}

// SetLogger 自定义请求日志
func (h *Httpr) SetLogger(logger zaplog.Iface) *Httpr {
	h.logger = &httprLogger{logger: logger}
	return h
}

// SetUrlPrefix 设置地址前缀, 形如"/api"这样的路由前缀，以"/"开头并不以"/"结尾
//	@param	prefix	string	地址前缀
func (h *Httpr) SetUrlPrefix(prefix string) *Httpr {
	// 重写现有的路由前缀
	h.makePrefix(strings.TrimSuffix(prefix, "/"))
	if h.i {
		h.client.SetBaseURL(h.prefix) // 适用于后期修改基地址
	}
	return h
}

// DoRequest 发起网络请求
//	@param	method	string				请求方法，取值为GET/POST/PATCH/PUT/DELETE
//	@param	url		string				路由地址，"url"形如"/tunnels/2"，以"/"开头并不以"/"结尾
//	@param	query	map[string]string	查询参数map
//	@param	body	map[string]any		请求数据
func (h *Httpr) DoRequest(method, url string, query map[string]string, body map[string]any) (*req.Response, error) {
	request := h.client.R()
	request.Error = &HttprError{} // SetError(), SetResult()方法均会增加1次反射调用

	if query != nil {
		request.SetQueryParams(query)
	}

	if body != nil {
		if bytes, err := helper.DefaultJsonMarshal(&body); err != nil {
			return nil, errors.New("marshal body error")
		} else {
			request.SetBodyJsonBytes(bytes)
		}
	}

	switch method {
	case "GET":
		return request.Get(url)
	case "POST":
		return request.Post(url)
	case "PUT":
		return request.Put(url)
	case "PATCH":
		return request.Patch(url)
	case "DELETE":
		return request.Delete(url)
	}
	return nil, errors.New("method error")
}

// GetClient 获取内部client，可用于设置请求头和中间件等，作用于全部请求
func (h *Httpr) GetClient() *req.Client {
	return h.client
}

// NewClient 新创建一个Client，并替换现有的client, 也是默认创建的client类型
func (h *Httpr) NewClient() *Httpr {
	// 设置默认的超时时间15s
	return h.init()
}

// NewDevClient 新创建一个调试Client，并替换现有的client
func (h *Httpr) NewDevClient() *Httpr {
	// 设置默认的超时时间15s
	h.init().client.DevMode()
	return h
}

// UnmarshalJson 将Http的返回体转换成结构体
//	@param	resp	*req.Response	Http返回体
//	@param	v		any				结果结构体指针
func (h *Httpr) UnmarshalJson(resp *req.Response, v any) error {
	if resp.IsSuccess() {
		if bytes, err := resp.ToBytes(); err != nil {
			return err
		} else {
			// 此处替换标准库json的Unmarshal()方法,以获取更好的性能
			if err := helper.DefaultJsonUnmarshal(bytes, v); err != nil {
				return err
			}
			return nil
		}
	}
	return errors.New("request failed")
}

// Get 发起一个带查询参数的Get请求
//	@param	url		string				路由地址，"url"形如"/tunnels/2"，以"/"开头并不以"/"结尾
//	@param	query	map[string]string	查询参数map
func (h *Httpr) Get(url string, query map[string]string) (*req.Response, error) {
	return h.DoRequest("GET", url, query, nil)
}

// Post 发起一个带请求体的Post请求
//	@param	url		string			路由地址
//	@param	body	map[string]any	请求数据	cannot	be	nil
func (h *Httpr) Post(url string, body map[string]any) (*req.Response, error) {
	if body == nil {
		return nil, errors.New("request body cannot be nil")
	}

	return h.DoRequest("POST", url, nil, body)
}

// Put 发起一个带请求体的Put请求
//	@param	url		string			路由地址
//	@param	body	map[string]any	请求数据
func (h *Httpr) Put(url string, body map[string]any) (*req.Response, error) {
	return h.DoRequest("PUT", url, nil, body)
}

// Patch 发起一个带请求体的Patch请求
//	@param	url		string			路由地址
//	@param	body	map[string]any	请求数据
func (h *Httpr) Patch(url string, body map[string]any) (*req.Response, error) {
	return h.DoRequest("PATCH", url, nil, body)
}

// Delete  发起一个Delete请求
//	@param	url	string	路由地址
func (h *Httpr) Delete(url string) (*req.Response, error) {
	return h.DoRequest("DELETE", url, nil, nil)
}

// PostWithQuery 发起一个带查询参数的Post请求
//	@param	url		string				路由地址
//	@param	query	map[string]string	查询参数map
//	@param	body	map[string]any		请求数据	cannot	be	nil
func (h *Httpr) PostWithQuery(url string, query map[string]string, body map[string]any) (*req.Response, error) {
	if body == nil {
		return nil, errors.New("body is nil")
	}

	return h.DoRequest("POST", url, query, body)
}

// DeleteWithQuery 发起一个带查询参数的Delete请求
//	@param	url		string				路由地址
//	@param	query	map[string]string	查询参数map
func (h *Httpr) DeleteWithQuery(url string, query map[string]string) (*req.Response, error) {
	return h.DoRequest("DELETE", url, query, nil)
}

// PutWithQuery 发起一个带查询参数的Put请求
//	@param	url		string				路由地址
//	@param	query	map[string]string	查询参数map
//	@param	body	map[string]any		请求数据
func (h *Httpr) PutWithQuery(url string, query map[string]string, body map[string]any) (*req.Response, error) {
	return h.DoRequest("PUT", url, query, body)
}

// PatchWithQuery 发起一个带查询参数的Patch请求
//	@param	url		string				路由地址
//	@param	query	map[string]string	查询参数map
//	@param	body	map[string]any		请求数据
func (h *Httpr) PatchWithQuery(url string, query map[string]string, body map[string]any) (*req.Response, error) {
	return h.DoRequest("PATCH", url, query, body)
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
func NewHttpr(host, port string, timeout int, logger zaplog.Iface) (*Httpr, error) {
	if host == "" || port == "" {
		return nil, errors.New("host and port cannot be empty")
	}

	r := &Httpr{Host: host, Port: port}
	r.SetTimeout(timeout).SetLogger(logger).NewClient()

	return r, nil
}
