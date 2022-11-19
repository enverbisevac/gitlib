package log

type Logger interface {
	Info(format string, args ...any)
	Debug(format string, args ...any)
	Error(format string, args ...any)
	Warn(format string, args ...any)
	Fatal(format string, args ...any)
	Trace(format string, args ...any)
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

func Debug(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Debug(format, args...)
}

func Error(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Error(format, args...)
}

func Warn(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Warn(format, args...)
}

func Fatal(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Fatal(format, args...)
}

func Trace(format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Trace(format, args...)
}
