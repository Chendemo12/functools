// Package python 常用的python标准库方法实现
package python

import (
	"github.com/Chendemo12/functools/types"
)

// Any 任意一个参数为true时为true
func Any(args ...bool) bool {
	for i := 0; i < len(args); i++ {
		if args[i] {
			return true
		}
	}
	return false
}

// All 参数全部为true时为true
func All(args ...bool) bool {
	if len(args) == 0 {
		return false
	}
	for i := 0; i < len(args); i++ {
		if !args[i] {
			return false
		}
	}
	return true
}

// Max 计算列表内部的最大元素, 需确保目标列表不为空
func Max[T types.Ordered](p ...T) T {
	r := p[0]
	for i := 0; i < len(p); i++ {
		if r > p[i] {
			continue
		}
		r = p[i]
	}
	return r
}

// Min 计算列表内部的最小元素, 需确保目标列表不为空
func Min[T types.Ordered](p ...T) T {
	r := p[0]
	for i := 0; i < len(p); i++ {
		if r < p[i] {
			continue
		}
		r = p[i]
	}
	return r
}

// Index 获取指定元素在列表中的下标,若不存在则返回-1
func Index[T comparable](s []T, x T) int {
	for i := 0; i < len(s); i++ {
		if s[i] == x {
			return i
		}
	}

	return -1
}

// In 查找序列s内是否存在元素x
//
//	@param	s	[]T	查找序列
//	@param	x	T	特定元素
//	@return	bool true if x in s, false otherwise
func In[T comparable](x T, s []T) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == x {
			return true
		}
	}
	return false
}

// Map 对序列 seq 中的每一个元素一次执行方法fn，仅指针类型有效
func Map[T any](fn func(t T), seq []T) {
	for i := 0; i < len(seq); i++ {
		fn(seq[i])
	}
}

// Filter 返回序列seq中满足条件fn的元素，产生一个符合条件的新切片
func Filter[T any](fn func(t T) bool, seq []T) []T {
	dst := make([]T, 0)
	for _, s := range seq {
		if fn(s) {
			dst = append(dst, s)
		}
	}
	return dst
}

// Has 查找序列s内是否存在元素x
//
//	@param	s	[]T	查找序列
//	@param	x	T	特定元素
//	@return	bool true if s contains x, false otherwise
func Has[T comparable](s []T, x T) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == x {
			return true
		}
	}
	return false
}
