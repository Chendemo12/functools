package helper

import (
	"encoding/base64"
	jsoniter "github.com/json-iterator/go"
	"strings"
)

const hexTable = "0123456789abcdef"

//goland:noinspection GoUnusedGlobalVariable
var ( // 替换json标准库，提供更好的性能
	// 与标准库 100%兼容的配置
	json              = jsoniter.ConfigCompatibleWithStandardLibrary
	JsonMarshal       = json.Marshal
	JsonUnmarshal     = json.Unmarshal
	JsonMarshalIndent = json.MarshalIndent
	JsonNewDecoder    = json.NewDecoder
	JsonNewEncoder    = json.NewEncoder
	// FasterJson 更快的配置，浮点数仅能保留6位小数, 且不能序列化HTML
	FasterJson          = jsoniter.ConfigFastest
	FasterJsonMarshal   = FasterJson.Marshal
	FasterJsonUnmarshal = FasterJson.Unmarshal
	// DefaultJson 默认配置
	DefaultJson          = FasterJson
	DefaultJsonMarshal   = DefaultJson.Marshal
	DefaultJsonUnmarshal = DefaultJson.Unmarshal
)

// HexBeautify 格式化显示十六进制
func HexBeautify(src []byte) string {
	length := len(src)*3 + 1
	dst := make([]byte, length)

	j := 0
	for _, v := range src {
		dst[j] = hexTable[v>>4]
		dst[j+1] = hexTable[v&0x0f]
		dst[j+2] = 32 // 空格
		j += 3
	}
	// 去除末尾的空格
	if length >= 2 {
		return string(dst[:length-2])
	} else {
		return string(dst[:length-1])
	}
}

// CombineStrings 合并字符串, 实现等同于strings.Join()，只是少了判断分隔符
func CombineStrings(elems ...string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}
	n := 0
	for i := 0; i < len(elems); i++ {
		n += len(elems[i])
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(elems[0])
	for _, s := range elems[1:] {
		b.WriteString(s)
	}
	return b.String()
}

// WordCapitalize 单词首字母大写
// @param   word  string  单词
// @return  string 首字母大写的单词
func WordCapitalize(word string) string {
	return strings.ToUpper(word)[:1] + strings.ToLower(word[1:])
}

// Base64Encode base64编码
// @param   data  []byte  字节流
// @return  string base64字符串
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode base64解码
// @param  data  string  base64字符串
func Base64Decode(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

func MapToString(object map[string]any) string {
	if bytes, err := DefaultJsonMarshal(&object); err != nil {
		return ""
	} else {
		return string(bytes)
	}
}
