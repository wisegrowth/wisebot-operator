package logger

import (
	"github.com/Sirupsen/logrus"
	ravenSentry "github.com/evalphobia/logrus_sentry"
)

var (
	log *logrus.Logger
)

// setLogger sets the package level logger
func setLogger(l *logrus.Logger) {
	log = l
}

// GetLogger returns package level logger
func GetLogger() *logrus.Logger {
	return log
}

// Init initialize the global module logger
func Init(wisebotID, sentryDSN string) error {
	log := logrus.New()

	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.JSONFormatter{}

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
	log = log.WithField("wisebot-id", wisebotID).Logger
	setLogger(log)

	return nil
}
