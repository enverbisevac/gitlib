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

func (l *sLog) Error(format string, args ...any) {
	if l == nil {
		return
	}
	slog.Log(slog.ErrorLevel, format, fmt.Sprintf(format, args...))
}
