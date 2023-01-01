// Package cprint 彩色输出
package cprint

import (
	"github.com/Chendemo12/functools/helper"
	"github.com/Chendemo12/functools/python"
	"os"
)

const (
	End   = "\u001B[0m"
	EndLn = "\u001B[0m\n"
	Ero   = "\u001B[31m"
	Suc   = "\u001B[32m"
	War   = "\u001B[33m"
	Inf   = "\u001B[34m"
	Deb   = "\u001B[35m" // 紫红色
)

func White(message string)  { _, _ = os.Stdout.WriteString(message + "\n") }
func Blue(message string)   { _, _ = os.Stdout.WriteString(helper.CombineStrings(Inf, message, EndLn)) }
func Red(message string)    { _, _ = os.Stderr.WriteString(helper.CombineStrings(Ero, message, EndLn)) }
func Yellow(message string) { _, _ = os.Stderr.WriteString(helper.CombineStrings(War, message, EndLn)) }
func Green(message string)  { _, _ = os.Stdout.WriteString(helper.CombineStrings(Suc, message, EndLn)) }
func Fuchsia(message string) {
	_, _ = os.Stdout.WriteString(helper.CombineStrings(Deb, message, EndLn))
}

func FWhite(message string, object any)  { White(helper.CombineStrings(message, python.Repr(object))) }
func FBlue(message string, object any)   { Blue(helper.CombineStrings(message, python.Repr(object))) }
func FRed(message string, object any)    { Red(helper.CombineStrings(message, python.Repr(object))) }
func FYellow(message string, object any) { Yellow(helper.CombineStrings(message, python.Repr(object))) }
func FGreen(message string, object any)  { Green(helper.CombineStrings(message, python.Repr(object))) }
func FFuchsia(message string, object any) {
	Fuchsia(helper.CombineStrings(message, python.Repr(object)))
}
