package types

// Ordered 是匹配任何有序类型的类型约束.
// An ordered type is one that supports the <, <=, >, and >= operators.
type Ordered interface {
	~int | ~uint | ~float64 | ~string
}

// ComparableHash is a type constraint that matches all
// comparable types with a Hash method.
type ComparableHash interface {
	comparable
	Hash() uintptr
}

// ImpossibleConstraint is a type constraint that no type can satisfy,
// because slice types are not comparable.
type ImpossibleConstraint interface {
	comparable
	[]int
}

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

type Number interface {
	~float64 | ~float32 | ~complex64 | ~complex128
}

// UnsignedInteger 无符号数字约束
type UnsignedInteger interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint
}

// SignedInteger 有符号数字约束
type SignedInteger interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int
}
