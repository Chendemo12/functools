// Package httpc HTTP请求客户端
package httpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Chendemo12/functools/helper"
	"io"
	"net"
	"net/http"
	"time"
)

var client = &http.Client{}

const (
	JsonContentType string = "application/json"
)

var ErrRespUnmarshal = errors.New("response unmarshal error")
var ErrReadResponse = errors.New("read response error")

// Opt 请求参数, T 为响应体类型
type Opt[T any] struct {
	// 对于作为 DoRequest 函数参数时，其为请求路由后缀，对于返回值而言，其为包含了域名及路由参数等的全路径
	Url string
	// 请求体的表单类型，默认为 application/json, 目前(230725)仅支持 application/json
	ContextType string
	// 查询参数
	Query map[string]string
	// 请求体模型, 会结合 ContextType 对其序列化后设置到请求体内
	RequestModel any
	// 相应体模型, 若请求成功,则会尝试反序列化,默认为 map[string]any
	ResponseModel T
	// 相应体数据流
	ResponseSteam []byte
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
	ReqHook func(req *http.Request) error
	// (最先调用)在返回值被处理之前可自行添加操作
	RespHook func(resp *http.Response) error
	req      *http.Request
	resp     *http.Response
}

func (o *Opt[T]) clean(ctx context.Context, timeout time.Duration) {
	var data = new(T)
	o.ResponseModel = *data

	if o.ContextType == "" {
		o.ContextType = JsonContentType
	}
	if o.Query == nil {
		o.Query = map[string]string{}
	}
	if o.RequestModel == nil {
		o.RequestModel = []byte{}
	}
	if o.Timeout == 0 {
		o.Timeout = timeout
	}
	if o.Ctx == nil {
		o.Ctx, o.Cancel = context.WithTimeout(ctx, timeout)
	}
	if o.ReqHook == nil {
		o.ReqHook = func(req *http.Request) error { return nil }
	}
	if o.RespHook == nil {
		o.RespHook = func(resp *http.Response) error { return nil }
	}
}

// IsOK 请求是否成功
func (o *Opt[T]) IsOK() bool {
	return http.StatusOK <= o.StatusCode && o.StatusCode <= http.StatusIMUsed
}

// ErrorOccurred 是否发送错误
func (o *Opt[T]) ErrorOccurred() bool { return o.Err != nil }

// IsUnmarshalError 是否是反序列化错误, 对于请求发起之前代表请求体反序列化错误
// 请求成功之后为响应体反序列化错误
func (o *Opt[T]) IsUnmarshalError() bool {
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
func (o *Opt[T]) IsTimeout() bool {
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
func (o *Opt[T]) IsConnectError() bool {
	if !o.IsOK() {
		var opErr *net.OpError
		if errors.As(o.Err, &opErr) && opErr.Op == "dial" {
			return true
		}
	}

	return false
}

// IsReadResponseError 读取返回体是否出错
func (o *Opt[T]) IsReadResponseError() bool {
	if o.IsOK() {
		return false
	}
	return errors.Is(o.Err, ErrReadResponse)
}

// DoRequest 发起网络请求
//
//	@param	method	string	请求方法，取值为GET/POST/PATCH/PUT/DELETE
//	@param	url		string	路由地址，"url"形如"/tunnels/2"，以"/"开头并不以"/"结尾
func DoRequest[T any](method, url string, opts ...*Opt[T]) *Opt[T] {
	var opt *Opt[T]
	if len(opts) > 0 {
		opt = opts[0]
	} else {
		opt = &Opt[T]{}
	}

	opt.clean(context.Background(), 15*time.Second)

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
		return opt
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

	// 设置请求体的Content-Type
	opt.req.Header.Set("Content-Type", opt.ContextType)

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
	opt.Err = opt.ReqHook(opt.req)
	if opt.Err != nil {
		return opt
	}

	// -------------------------------------------------------------------
	// 发起网络请求
	opt.resp, opt.Err = client.Do(opt.req)
	if opt.Err != nil { // 网络请求失败
		return opt
	}

	// 请求成功，首先执行 RespHook
	opt.StatusCode = opt.resp.StatusCode
	opt.Err = opt.RespHook(opt.resp)
	if opt.Err != nil {
		return opt
	}

	opt.ResponseSteam, opt.Err = io.ReadAll(opt.resp.Body)
	if opt.Err != nil {
		opt.Err = errors.Join(ErrReadResponse, opt.Err)
		return opt
	}

	if opt.IsOK() && !opt.DisableMarshal {
		// 响应体反序列化, 返回响应体
		opt.Err = helper.JsonUnmarshal(opt.ResponseSteam, &opt.ResponseModel)
	}

	return opt
}

// GetWithOption 发起一个Get请求
func GetWithOption[T any](path string, opts ...*Opt[T]) *Opt[T] {
	return DoRequest[T]("GET", path, opts...)
}

// PostWithOption 发起一个Post请求
func PostWithOption[T any](path string, opts ...*Opt[T]) *Opt[T] {
	return DoRequest[T]("POST", path, opts...)
}

// PutWithOption 发起一个Put请求
func PutWithOption[T any](path string, opts ...*Opt[T]) *Opt[T] {
	return DoRequest[T]("PUT", path, opts...)
}

// PatchWithOption 发起一个Patch请求
func PatchWithOption[T any](path string, opts ...*Opt[T]) *Opt[T] {
	return DoRequest[T]("PATCH", path, opts...)
}

// DeleteWithOption 发起一个Delete请求
func DeleteWithOption[T any](path string, opts ...*Opt[T]) *Opt[T] {
	return DoRequest[T]("DELETE", path, opts...)
}

func Get[T any](url string, query map[string]string) (T, error) {
	opt := GetWithOption[T](url, &Opt[T]{Query: query, Url: url, DisableMarshal: false})
	if opt.ErrorOccurred() {
		return opt.ResponseModel, opt.Err
	}

	if !opt.IsOK() {
		return opt.ResponseModel, fmt.Errorf("code: %d, resp: %s", opt.StatusCode, string(opt.ResponseSteam))
	}

	return opt.ResponseModel, nil
}

func Delete[T any](url string, query map[string]string) (T, error) {
	opt := DeleteWithOption[T](url, &Opt[T]{Query: query, Url: url, DisableMarshal: false})
	if opt.ErrorOccurred() {
		return opt.ResponseModel, opt.Err
	}
	if !opt.IsOK() {
		return opt.ResponseModel, fmt.Errorf("code: %d, resp: %s", opt.StatusCode, string(opt.ResponseSteam))
	}

	return opt.ResponseModel, nil
}

func Post[T any](url string, query map[string]string, req any) (T, error) {
	opt := PostWithOption[T](url, &Opt[T]{Query: query, Url: url, DisableMarshal: false, RequestModel: req})
	if opt.ErrorOccurred() {
		return opt.ResponseModel, opt.Err
	}
	if !opt.IsOK() {
		return opt.ResponseModel, fmt.Errorf("code: %d, resp: %s", opt.StatusCode, string(opt.ResponseSteam))
	}

	return opt.ResponseModel, nil
}

func Patch[T any](url string, query map[string]string, req any) (T, error) {
	opt := PatchWithOption[T](url, &Opt[T]{Query: query, Url: url, DisableMarshal: false, RequestModel: req})
	if opt.ErrorOccurred() {
		return opt.ResponseModel, opt.Err
	}
	if !opt.IsOK() {
		return opt.ResponseModel, fmt.Errorf("code: %d, resp: %s", opt.StatusCode, string(opt.ResponseSteam))
	}

	return opt.ResponseModel, nil
}

func Put[T any](url string, query map[string]string, req any) (T, error) {
	opt := PutWithOption[T](url, &Opt[T]{Query: query, Url: url, DisableMarshal: false, RequestModel: req})
	if opt.ErrorOccurred() {
		return opt.ResponseModel, opt.Err
	}
	if !opt.IsOK() {
		return opt.ResponseModel, fmt.Errorf("code: %d, resp: %s", opt.StatusCode, string(opt.ResponseSteam))
	}

	return opt.ResponseModel, nil
}
