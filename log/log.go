package log

import (
	"fmt"
	"time"

	"github.com/fatih/color"
)

var (
	errColor  = color.New(color.FgRed, color.Bold)
	warnColor = color.New(color.FgYellow, color.Bold)
	infoColor = color.New()
	dbgColor  = color.New(color.FgHiBlack)
)

func prefix(severity string) string {
  return fmt.Sprintf("[%s] %s", severity, time.Now().Format("15:03:01.002"))
}

func Err(a ...any) {
	errColor.Println(prefix("ERROR"), fmt.Sprint(a...))
}

func Errf(format string, a ...any) {
  errColor.Println(prefix("ERROR"), fmt.Errorf(format, a...))
}

func Warn(a ...any) {
	warnColor.Println(prefix("WARN"), fmt.Sprint(a...))
}

func Warnf(format string, a ...any) {
  warnColor.Println(prefix("WARN"), fmt.Errorf(format, a...))
}

func Info(a ...any) {
	infoColor.Println(prefix("INFO"), fmt.Sprint(a...))
}

func Infof(format string, a ...any) {
  infoColor.Println(prefix("INFO"), fmt.Errorf(format, a...))
}

func Debug(a ...any) {
	dbgColor.Println(prefix("DEBUG"), fmt.Sprint(a...))
}

func Debugf(format string, a ...any) {
  dbgColor.Println(prefix("DEBUG"), fmt.Errorf(format, a...))
}
