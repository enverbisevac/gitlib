package log

type Logger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

var logger Logger = &sLog{}

func SetLogger(l Logger) {
	logger = l
}

func Info(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Info(format, args...)
}

func Error(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Error(format, args...)
}
