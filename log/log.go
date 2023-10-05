package log

import (
	"fmt"
	"strings"
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
  return fmt.Sprintf("[%s] %s", severity, time.Now().Format("15:04:05.000"))
}

func format(severity, s string) string {
  res := ""
  lines := strings.FieldsFunc(s, func(r rune) bool {return r == '\n'})
  indent := ""
  for i, line := range lines {
    if i == 0 {
      p := prefix(severity)
      indent = strings.Repeat(" ", len(p))
      res += fmt.Sprintf(p, " ", line, '\n')
      continue
    }
    res += fmt.Sprint(indent, " ", line, '\n')
  }
  return res
}

func Err(a ...any) {
	errColor.Println(format("ERROR", fmt.Sprint(a...)))
}

func Errf(f string, a ...any) {
  errColor.Println(format("ERROR", fmt.Errorf(f, a...).Error()))
}

func Warn(a ...any) {
	warnColor.Println(format("WARN", fmt.Sprint(a...)))
}

func Warnf(f string, a ...any) {
  warnColor.Println(format("WARN", fmt.Errorf(f, a...).Error()))
}

func Info(a ...any) {
	infoColor.Println(prefix("INFO"), fmt.Sprint(a...))
}

func Infof(f string, a ...any) {
  infoColor.Println(format("INFO", fmt.Errorf(f, a...).Error()))
}

func Debug(a ...any) {
	dbgColor.Println(format("DEBUG", fmt.Sprint(a...)))
}

func Debugf(f string, a ...any) {
  dbgColor.Println(format("DEBUG", fmt.Errorf(f, a...).Error()))
}
