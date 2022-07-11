package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Ctx is an alias for logrus.Fields.
type Ctx logrus.Fields

var logger *logrus.Logger

// InitLogger initialises the package level logger.
func InitLogger(filepath string, debug bool) error {
	logger = logrus.StandardLogger()
	logger.SetOutput(os.Stdout)
	logger.Formatter = &logrus.TextFormatter{FullTimestamp: true}

	if filepath != "" {
		f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return err
		}

		logger.Out = f
		defer f.Close()
	}

	if debug {
		logger.Level = logrus.DebugLevel
	}

	return nil
}

// Debug writes a debug log message.
func Debug(msg string, ctx ...Ctx) {
	if logger != nil {
		entry := logrus.NewEntry(logger)
		for _, c := range ctx {
			entry = entry.WithFields(logrus.Fields(c))
		}

		entry.Debug(msg)
	}
}

// Debugf writes a formatted debug log message.
func Debugf(msg string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(msg, args...)
	}
}

// Info writes an info log message.
func Info(msg string, ctx ...Ctx) {
	if logger != nil {
		entry := logrus.NewEntry(logger)
		for _, c := range ctx {
			entry = entry.WithFields(logrus.Fields(c))
		}

		entry.Info(msg)
	}
}

// Infof writes a formatted info log message.
func Infof(msg string, args ...interface{}) {
	if logger != nil {
		logger.Infof(msg, args...)
	}
}

// Warn writes a warn log message.
func Warn(msg string, ctx ...Ctx) {
	if logger != nil {
		entry := logrus.NewEntry(logger)
		for _, c := range ctx {
			entry = entry.WithFields(logrus.Fields(c))
		}

		entry.Warn(msg)
	}
}

// Warnf writes a formatted warn log message.
func Warnf(msg string, args ...interface{}) {
	if logger != nil {
		logger.Warnf(msg, args...)
	}
}

// Error writes an error log message.
func Error(msg string, ctx ...Ctx) {
	if logger != nil {
		entry := logrus.NewEntry(logger)
		for _, c := range ctx {
			entry = entry.WithFields(logrus.Fields(c))
		}

		entry.Error(msg)
	}
}

// Errorf writes a formatted error log message.
func Errorf(msg string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(msg, args...)
	}
}

// WithCtx adds the given Ctx fields to the logger.
func WithCtx(fields Ctx) *logrus.Entry {
	return logger.WithFields(logrus.Fields(fields))
}
