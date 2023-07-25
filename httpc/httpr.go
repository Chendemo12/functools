// Package httpc HTTP请求客户端
package httpc

import (
	"bytes"
	"context"
	"errors"
	"github.com/Chendemo12/fastapi-tool/helper"
	"github.com/Chendemo12/functools/logger"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type ContentType string

const (
	JsonContentType ContentType = "application/json"
)

var ErrRespUnmarshal = errors.New("response unmarshal error")
var ErrReadResponse = errors.New("read response error")

type Opt struct {
	// 对于作为 DoRequest 函数参数时，其为请求路由后缀，对于返回值而言，其为包含了域名及路由参数等的全路径
	Url string
	// 请求体的表单类型，默认为 application/json, 目前(230725)仅支持 application/json
	ContextType ContentType
	// 查询参数
	Query map[string]string
	// 请求体模型, 会结合 ContextType 对其序列化后设置到请求体内
	RequestModel any
	// 相应体模型, 若请求成功,则会尝试反序列化,默认为 map[string]any
	ResponseModel any
	// 是否禁用返回值自动序列化
	DisableMarshal bool
	// 请求的默认超时时间
	Timeout time.Duration
	// 父context
	Ctx    context.Context
	Cancel context.CancelFunc
	// 依据处理步骤, 其可能为请求错误, 也可能为返回错误
	Err error
	// 响应状态码
	StatusCode int
	// (最后调用)在请求发起之前可自行添加操作
	ReqHook func(req *http.Request)
	// (最先调用)在返回值被处理之前可自行添加操作
	RespHook func(resp *http.Response)
	req      *http.Request
	resp     *http.Response
}

// IsOK 请求是否成功
func (o Opt) IsOK() bool {
	return http.StatusOK <= o.StatusCode && o.StatusCode <= http.StatusIMUsed
}

// ErrorOccurred 是否发送错误
func (o Opt) ErrorOccurred() bool { return o.Err != nil }

// IsUnmarshalError 是否是反序列化错误, 对于请求发起之前代表请求体反序列化错误
// 请求成功之后为响应体反序列化错误
func (o Opt) IsUnmarshalError() bool {
	if o.resp != nil { // 响应体反序列化
		if o.DisableMarshal {
			return false
		}
		return errors.Is(o.Err, ErrRespUnmarshal)
	} else { // 请求体反序列化错误
		return o.Err != nil
	}
}

// IsTimeout 判断是否是因为超时引发的错误
func (o Opt) IsTimeout() bool {
	if !o.IsOK() {
		var err net.Error
		ok := errors.As(o.Err, &err)
		if ok {
			return err.Timeout()
		}
	}

	return false
}

// IsConnectError 是否是请求发起错误
func (o Opt) IsConnectError() bool {
	if !o.IsOK() {
		var opErr *net.OpError
		if errors.As(o.Err, &opErr) && opErr.Op == "dial" {
			return true
		}
	}

	return false
}

// IsReadResponseError 读取返回体是否出错
func (o Opt) IsReadResponseError() bool {
	if o.IsOK() {
		return false
	}
	return errors.Is(o.Err, ErrReadResponse)
}

// Httpr http 请求客户端
// 请求地址默认前缀为"/api", 若不是,需在实例化结构体之后，通过 SetUrlPrefix 显式更改路由前缀
// 请求的默认超时时间15s, 通过 SetTimeout 显式更改
type Httpr struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	ctx     context.Context
	client  *http.Client
	logger  logger.Iface
	prefix  string // 请求路由前缀，以/结尾
	timeout time.Duration
}

// 组合路由，形如："http://127.0.0.1:3306/api/suffix"
//
//	@param	prefix	string	地址前缀
func (h *Httpr) cUrl(suffix string) string {
	if h.prefix == "" {
		h.SetUrlPrefix("")
	}

	return h.prefix + strings.TrimPrefix(suffix, "/")
}

func (h *Httpr) cleanOpt(opts ...*Opt) *Opt {
	var opt *Opt

	if len(opts) > 0 {
		opt = opts[0]
	} else {
		opt = &Opt{}
	}

	if opt.ContextType == "" {
		opt.ContextType = JsonContentType
	}
	if opt.Query == nil {
		opt.Query = map[string]string{}
	}
	if opt.RequestModel == nil {
		opt.RequestModel = []byte{}
	}
	if opt.ResponseModel == nil {
		opt.ResponseModel = []byte{}
	}
	if opt.Timeout == 0 {
		opt.Timeout = h.timeout
	}
	if opt.Ctx == nil {
		opt.Ctx, opt.Cancel = context.WithTimeout(h.ctx, h.timeout)
	}
	if opt.ReqHook == nil {
		opt.ReqHook = func(req *http.Request) {}
	}
	if opt.RespHook == nil {
		opt.RespHook = func(resp *http.Response) {}
	}

	return opt
}

// SetTimeout 设置请求的超时时间，单位s
//
//	@param	timeout	int	超时时间(s)
func (h *Httpr) SetTimeout(timeout time.Duration) *Httpr {
	h.timeout = timeout
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
	// noinspection HttpUrlsUsage
	h.prefix = helper.F(
		"http://", h.Host, ":", h.Port,
		"/", strings.TrimPrefix(prefix, "/"), "/",
	)
	return h
}

// DoRequest 发起网络请求
//
//	@param	method	string		请求方法，取值为GET/POST/PATCH/PUT/DELETE
//	@param	url		string		路由地址，"url"形如"/tunnels/2"，以"/"开头并不以"/"结尾
func (h *Httpr) DoRequest(method, url string, opts ...*Opt) (opt *Opt) {
	opt = h.cleanOpt(opts...)
	// 释放资源
	defer func() {
		if opt.resp != nil && opt.resp.Body != nil {
			_ = opt.resp.Body.Close()
		}
		if opt.Cancel != nil {
			opt.Cancel()
		}
		opt.req = nil
		opt.resp = nil
	}()

	// 必须首先设置请求路由
	// 请求参数具有更高的优先级
	if url != "" {
		opt.Url = url
	}

	// 对于绝对路由则不拼接
	// noinspection HttpUrlsUsage
	if !strings.HasPrefix(opt.Url, "http://") && !strings.HasPrefix(opt.Url, "https://") {
		opt.Url = h.cUrl(opt.Url)
	}

	// -------------------------------------------------------------------
	// 请求体设置
	var reqBody []byte
	switch opt.RequestModel.(type) {
	case []byte:
		reqBody = opt.RequestModel.([]byte)
	default:
		if opt.ContextType == JsonContentType {
			reqBody, opt.Err = helper.JsonMarshal(opt.RequestModel)
		}
	}

	if opt.Err != nil {
		opt.StatusCode = 0
		opt.Err = errors.New("request-body marshal error")
		return
	}

	// 依据请求方法构建http请求
	switch method {
	case http.MethodPost:
		opt.req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodPost, opt.Url, bytes.NewReader(reqBody))
	case http.MethodPut:
		opt.req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodPut, opt.Url, bytes.NewReader(reqBody))
	case http.MethodPatch:
		opt.req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodPatch, opt.Url, bytes.NewReader(reqBody))
	case http.MethodDelete:
		opt.req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodDelete, opt.Url, nil)
	default:
		opt.req, _ = http.NewRequestWithContext(opt.Ctx, http.MethodGet, opt.Url, nil)
	}
	// 设置请求的Content-Type为application/json
	opt.req.Header.Set("Content-Type", string(opt.ContextType))

	// -------------------------------------------------------------------
	// 设置路由参数
	// 将现有的查询参数解析为 url.Values
	query := opt.req.URL.Query()
	// 添加新的查询参数
	for key, value := range opt.Query {
		query.Add(key, value)
	}

	// 重新编码查询参数并设置到请求的 URL 中
	opt.req.URL.RawQuery = query.Encode()
	opt.Url = opt.req.URL.String()

	// 执行发送前钩子
	opt.ReqHook(opt.req)

	// -------------------------------------------------------------------
	// 发起网络请求
	opt.resp, opt.Err = h.client.Do(opt.req)
	if opt.Err != nil { // 网络请求失败
		return
	}

	// 请求成功，首先执行 RespHook
	opt.StatusCode = opt.resp.StatusCode
	opt.RespHook(opt.resp)

	if !opt.IsOK() { // 但是状态码为错误码，不再进行响应体处理
		return
	}

	// 请求成功，序列化返回值
	_bytes, err := io.ReadAll(opt.resp.Body)
	if err != nil { // 读取返回值错误
		opt.Err = errors.Join(ErrReadResponse, err)
	} else {
		if !opt.DisableMarshal { // 响应体反序列化, 返回响应体
			opt.Err = helper.JsonUnmarshal(_bytes, opt.ResponseModel)
		} else {
			opt.ResponseModel = _bytes
		}
	}

	return
}

// Get 发起一个Get请求
func (h *Httpr) Get(path string, opts ...*Opt) *Opt {
	return h.DoRequest("GET", path, opts...)
}

// Post 发起一个Post请求
func (h *Httpr) Post(path string, opts ...*Opt) *Opt {
	return h.DoRequest("POST", path, opts...)
}

// Put 发起一个Put请求
func (h *Httpr) Put(path string, opts ...*Opt) *Opt {
	return h.DoRequest("PUT", path, opts...)
}

// Patch 发起一个Patch请求
func (h *Httpr) Patch(path string, opts ...*Opt) *Opt {
	return h.DoRequest("PATCH", path, opts...)
}

// Delete  发起一个Delete请求
func (h *Httpr) Delete(path string, opts ...*Opt) *Opt {
	return h.DoRequest("DELETE", path, opts...)
}

// NewHttpr 创建一个新的HTTP请求基类
func NewHttpr(host, port string) (*Httpr, error) {
	if host == "" || port == "" {
		return nil, errors.New("host and port cannot be empty")
	}

	r := &Httpr{Host: host, Port: port, client: &http.Client{}, ctx: context.Background()}
	r.SetTimeout(time.Second * 15).SetLogger(logger.NewDefaultLogger())

	return r, nil
}
