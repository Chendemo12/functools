// Package cprint 彩色输出
package cprint

import (
	"github.com/Chendemo12/fastapi-tool/helper"
	"github.com/Chendemo12/functools/python"
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

func White(message string) {
	_, _ = os.Stdout.WriteString(message + "\n")
}
func Blue(message string) {
	_, _ = os.Stdout.WriteString(helper.CombineStrings(blue, message, EndLn))
}
func Red(message string) {
	_, _ = os.Stderr.WriteString(helper.CombineStrings(red, message, EndLn))
}
func Yellow(message string) {
	_, _ = os.Stderr.WriteString(helper.CombineStrings(yellow, message, EndLn))
}
func Green(message string) {
	_, _ = os.Stdout.WriteString(helper.CombineStrings(green, message, EndLn))
}
func Fuchsia(message string) {
	_, _ = os.Stdout.WriteString(helper.CombineStrings(fuchsia, message, EndLn))
}

func FWhite(message string, object any)  { White(helper.CombineStrings(message, python.Repr(object))) }
func FBlue(message string, object any)   { Blue(helper.CombineStrings(message, python.Repr(object))) }
func FRed(message string, object any)    { Red(helper.CombineStrings(message, python.Repr(object))) }
func FYellow(message string, object any) { Yellow(helper.CombineStrings(message, python.Repr(object))) }
func FGreen(message string, object any)  { Green(helper.CombineStrings(message, python.Repr(object))) }
func FFuchsia(message string, object any) {
	Fuchsia(helper.CombineStrings(message, python.Repr(object)))
}
