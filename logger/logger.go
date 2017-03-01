package logger

import (
	"io"

	"github.com/Sirupsen/logrus"
	ravenSentry "github.com/evalphobia/logrus_sentry"
)

var (
	environment string
	log         Logger = logrus.New()
)

// Logger is the exposed standard-ish logging interface
type Logger interface {
	Debug(v ...interface{})
	Info(v ...interface{})
	Error(v ...interface{})
	Warn(v ...interface{})
	Fatal(v ...interface{})

	WithFields(logrus.Fields) *logrus.Entry
	WithField(key string, val interface{}) *logrus.Entry
}

// setLogger sets the package level logger
func setLogger(l *logrus.Entry) {
	log = l
}

// GetLogger returns package level logger
func GetLogger() Logger {
	return log
}

// Init initialize the global module logger
func Init(wisebotID, sentryDSN string) error {
	log := logrus.New()

	log.Level = logrus.DebugLevel
	if environment == "production" {
		log.Formatter = &logrus.JSONFormatter{}
	}

	if len(sentryDSN) > 0 {
		levels := []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
		}

		hook, err := ravenSentry.NewAsyncWithTagsSentryHook(sentryDSN, map[string]string{"wisebot-id": wisebotID}, levels)
		if err != nil {
			return err
		}

		hook.StacktraceConfiguration.Enable = true

		log.Hooks.Add(hook)
	}

	setLogger(log.WithField("wisebot-id", wisebotID))

	return nil
}
