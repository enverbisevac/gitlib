package log

import (
	"fmt"

	"golang.org/x/exp/slog"
)

type sLog struct {
}

func (l *sLog) Info(format string, args ...any) {
	if logger == nil {
		return
	}
	slog.Info(format, fmt.Sprintf(format, args...))
}

func (l *sLog) Debug(format string, args ...any) {
	if logger == nil {
		return
	}
	slog.Debug(format, fmt.Sprintf(format, args...))
}

func (l *sLog) Error(format string, args ...any) {
	if l == nil {
		return
	}
	slog.Log(slog.ErrorLevel, format, fmt.Sprintf(format, args...))
}

func (l *sLog) Warn(format string, args ...any) {
	if l == nil {
		return
	}
	slog.Warn(format, fmt.Sprintf(format, args...))
}

func (l *sLog) Fatal(format string, args ...any) {
	if l == nil {
		return
	}
}

func (l *sLog) Trace(format string, args ...any) {

}
