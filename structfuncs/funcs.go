// Package structfuncs struct格式化相关的函数方法，慎用，包含大量反射方法
package structfuncs

import (
	"errors"
	"gitlab.cowave.com/gogo/functools/helper"
	"reflect"
	"unicode"
)

// Reflect 反射struct
func Reflect(object any) (at reflect.Type, v reflect.Value, err error) {
	at = reflect.TypeOf(object)
	if at.Kind() != reflect.Ptr && at.Kind() != reflect.Struct {
		err = errors.New("object is not a struct")
	}
	switch at.Kind() {
	case reflect.Pointer: // 指针类型
		v = reflect.Indirect(reflect.ValueOf(object))
		at = v.Type()
	case reflect.Struct: // 结构体类型
		v = reflect.ValueOf(object)
	}

	return
}

// ToMap 转换struct为map (未优化)
// @param  object  struct对象
func ToMap(object any) (map[string]any, error) {
	mp := make(map[string]any)
	bytes, err := helper.DefaultJsonMarshal(object)
	if err != nil {
		return nil, err
	}

	err = helper.DefaultJsonUnmarshal(bytes, &mp)
	if err != nil {
		return nil, err
	}
	return mp, nil
}

// ToString 转换struct为string类型
func ToString(object any) string {
	if bytes, err := helper.DefaultJsonMarshal(&object); err != nil {
		return ""
	} else {
		return string(bytes)
	}
}

// ToJson 转换struct为map和string类型
func ToJson(s any) (map[string]any, string) {
	data, _ := helper.DefaultJsonMarshal(s)
	m := make(map[string]any)
	if err := helper.DefaultJsonUnmarshal(data, &m); err != nil {
		return nil, ""
	}
	return m, string(data)
}

// GetName 获取struct的名称
func GetName(s any) (name string) {
	at := reflect.TypeOf(s)
	if at.Kind() == reflect.Pointer {
		name = at.Elem().Name()
	} else {
		name = at.Name()
	}
	return
}

// GetFullName 获取struct的绝对名称：packageName.StructName
func GetFullName(s any) (name string) {
	at := reflect.TypeOf(s)
	if at.Kind() == reflect.Pointer {
		name = at.Elem().String()
	} else {
		name = at.String()
	}
	return name
}

// GetFieldsName 获取struct的全部字段名(不包含私有字段)
func GetFieldsName(s any) []string {
	var v reflect.Value
	at := reflect.TypeOf(s)

	fieldsNum := 0 // 字段总数量
	fields := make([]string, fieldsNum)

	switch at.Kind() {

	case reflect.Pointer: // 指针类型
		v = reflect.Indirect(reflect.ValueOf(s))
		at = v.Type()
		fieldsNum = v.NumField()
	case reflect.Struct: // 结构体类型
		fieldsNum = at.NumField()
	default:
		return nil
	}

	for i := 0; i < fieldsNum; i++ {
		fn := at.Field(i).Name
		if unicode.IsUpper(rune(fn[0])) {
			fields = append(fields, fn)
		}
	}
	return fields
}

// GetFieldsValue 获取struct的字段键值对
// @param   s  any  struct  Object
// @return  map[string]any {key: value}
func GetFieldsValue(s any) map[string]any {
	var v reflect.Value
	at := reflect.TypeOf(s)

	fieldsNum := 0                 // 字段总数量
	fields := make(map[string]any) // 字段键值对

	switch at.Kind() {

	case reflect.Pointer: // 指针类型
		v = reflect.Indirect(reflect.ValueOf(s))
		fieldsNum = v.NumField()

	case reflect.Struct: // 结构体类型
		v = reflect.ValueOf(s)
		fieldsNum = at.NumField()

	default:
		return nil
	}

	for i := 0; i < fieldsNum; i++ { // 遍历获取全部键值对
		field := at.Field(i)
		if unicode.IsUpper(rune(field.Name[0])) { // 仅提取公共属性
			switch field.Type.Kind() { // 获取字段定义的类型

			case reflect.Array, reflect.Slice:
				fields[field.Name] = v.Field(i).Bytes()

			case reflect.Uint8:
				fields[field.Name] = byte(v.Field(i).Uint())
			case reflect.Uint16:
				fields[field.Name] = uint16(v.Field(i).Uint())
			case reflect.Uint32:
				fields[field.Name] = uint32(v.Field(i).Uint())
			case reflect.Uint64, reflect.Uint:
				fields[field.Name] = v.Field(i).Uint()

			case reflect.Int8:
				fields[field.Name] = int8(v.Field(i).Int())
			case reflect.Int16:
				fields[field.Name] = int16(v.Field(i).Int())
			case reflect.Int32:
				fields[field.Name] = int32(v.Field(i).Int())
			case reflect.Int64, reflect.Int:
				fields[field.Name] = v.Field(i).Int()

			case reflect.Float32:
				fields[field.Name] = float32(v.Field(i).Float())
			case reflect.Float64:
				fields[field.Name] = v.Field(i).Float()

			case reflect.Struct, reflect.Interface, reflect.Map:
				fields[field.Name] = v.Field(i).Interface()

			case reflect.String:
				fields[field.Name] = v.Field(i).String()

			case reflect.Pointer:
				fields[field.Name] = v.Field(i).Pointer()
			case reflect.Bool:
				fields[field.Name] = v.Field(i).Bool()
			}
		}
	}
	return fields
}

// GetFieldsTags 获取struct的字段tags
func GetFieldsTags(s any) map[string]reflect.StructTag {
	var v reflect.Value
	at := reflect.TypeOf(s)

	fieldsNum := 0                               // 字段总数量
	fields := make(map[string]reflect.StructTag) // 字段键值对

	switch at.Kind() {
	case reflect.Pointer: // 指针类型
		v = reflect.Indirect(reflect.ValueOf(s))
		at = v.Type()
		fieldsNum = v.NumField()
	case reflect.Struct: // 结构体类型
		fieldsNum = at.NumField()
	default:
		return nil
	}

	for i := 0; i < fieldsNum; i++ {
		fn := at.Field(i).Name
		if unicode.IsUpper(rune(fn[0])) {
			fields[fn] = at.Field(i).Tag
		}
	}
	return fields
}

// GetFieldsType 获取struct的字段类型
// @param   s  any  struct  Object
// @return  map[string]any {key: type}
func GetFieldsType(s any) map[string]reflect.Kind {
	var v reflect.Value
	at := reflect.TypeOf(s)

	fieldsNum := 0 // 字段总数量

	switch at.Kind() {
	case reflect.Pointer: // 指针类型
		v = reflect.Indirect(reflect.ValueOf(s))

		fieldsNum = v.NumField()
	case reflect.Struct: // 结构体类型
		v = reflect.ValueOf(s)
		fieldsNum = at.NumField()

	default:
		return nil
	}
	fields := make(map[string]reflect.Kind, fieldsNum)

	for i := 0; i < fieldsNum; i++ { // 遍历获取全部键值对
		field := at.Field(i)
		if unicode.IsUpper(rune(field.Name[0])) { // 仅提取公共属性
			fields[field.Name] = field.Type.Kind()
		}
	}
	return fields
}
