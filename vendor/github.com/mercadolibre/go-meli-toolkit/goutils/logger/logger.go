package logger

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func init() {
	Log = &logrus.Logger{
		Out:       os.Stdout,
		Formatter: &MercuryFormatter{DisableColors: true},
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}
}

func SetLogLevel(logLevel string) {
	if level, error := logrus.ParseLevel(logLevel); error != nil {
		panic(error)
	} else {
		Log.Level = level
	}
}

func Print(e interface{}) {
	Log.Printf("%s: %s", e, debug.Stack())
}

func Debug(message string, tags ...string) {
	if Log.Level >= logrus.DebugLevel {
		tags = append(tags, "level:debug")
		Log.WithFields(addFields(tags...)).Debug(message)
	}
}

func Info(message string, tags ...string) {
	if Log.Level >= logrus.InfoLevel {
		tags = append(tags, "level:info")
		Log.WithFields(addFields(tags...)).Info(message)
	}
}

func Warn(message string, tags ...string) {
	if Log.Level >= logrus.WarnLevel {
		tags = append(tags, "level:warn")
		Log.WithFields(addFields(tags...)).Warn(message)
	}
}

func Error(message string, err error, tags ...string) {
	if Log.Level >= logrus.ErrorLevel {
		tags = append(tags, "level:error")
		message = fmt.Sprintf("%s - ERROR: %v", message, err)
		Log.WithFields(addFields(tags...)).Error(message)
	}
}

func Panic(message string, err error, tags ...string) {
	if Log.Level >= logrus.PanicLevel {
		tags = append(tags, "level:panic")
		message = fmt.Sprintf("%s - PANIC: %v", message, err)
		Log.WithFields(addFields(tags...)).Panic(message)
	}
}

func Debugf(format string, args ...interface{}) {
	if Log.Level >= logrus.DebugLevel {
		Debug(fmt.Sprintf(format, args...))
	}
}

func Infof(format string, args ...interface{}) {
	if Log.Level >= logrus.InfoLevel {
		Info(fmt.Sprintf(format, args...))
	}
}

func Warnf(format string, args ...interface{}) {
	if Log.Level >= logrus.WarnLevel {
		Warn(fmt.Sprintf(format, args...))
	}
}

func Errorf(format string, err error, args ...interface{}) {
	if Log.Level >= logrus.ErrorLevel {
		Error(fmt.Sprintf(format, args...), err)
	}
}

func Panicf(format string, err error, args ...interface{}) {
	if Log.Level >= logrus.PanicLevel {
		Panic(fmt.Sprintf(format, args...), err)
	}
}

func GetOut() io.Writer {
	return Log.Out
}

func addFields(tags ...string) logrus.Fields {
	fields := make(logrus.Fields)
	for _, value := range tags {
		values := strings.SplitN(value, ":", 2)
		fields[strings.TrimSpace(values[0])] = strings.TrimSpace(values[1])
	}

	return fields
}
