package log

import "github.com/fatih/color"

var (
	errColor  = color.New(color.FgRed, color.Bold)
	warnColor = color.New(color.FgYellow, color.Bold)
	infoColor = color.New()
	dbgColor  = color.New(color.FgHiBlack)
)

func Err(a ...any) {
	errColor.Println(a...)
}

func Warn(a ...any) {
	warnColor.Println(a...)
}

func Info(a ...any) {
	infoColor.Println(a...)
}

func Debug(a ...any) {
	dbgColor.Println(a...)
}
