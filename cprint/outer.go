// Package cprint 彩色输出
package cprint

import (
	"github.com/Chendemo12/functools/helper"
	"os"
)

const (
	End     = "\u001B[0m"
	EndLn   = "\u001B[0m\n"
	red     = "\u001B[31m"
	green   = "\u001B[32m"
	yellow  = "\u001B[33m"
	blue    = "\u001B[34m"
	fuchsia = "\u001B[35m" // 紫红色
)

// White 默认白色输出
func White(message string) {
	_, _ = os.Stdout.WriteString(message + "\n")
}

// Blue 蓝色输出
func Blue(message string) {
	_, _ = os.Stdout.WriteString(helper.CombineStrings(blue, message, EndLn))
}

// Red 红色输出
func Red(message string) {
	_, _ = os.Stderr.WriteString(helper.CombineStrings(red, message, EndLn))
}

// Yellow 黄色输出
func Yellow(message string) {
	_, _ = os.Stderr.WriteString(helper.CombineStrings(yellow, message, EndLn))
}

// Green 绿色输出
func Green(message string) {
	_, _ = os.Stdout.WriteString(helper.CombineStrings(green, message, EndLn))
}

// Fuchsia 紫红色输出
func Fuchsia(message string) {
	_, _ = os.Stdout.WriteString(helper.CombineStrings(fuchsia, message, EndLn))
}
