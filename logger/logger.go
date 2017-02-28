package logger

import (
	"github.com/Sirupsen/logrus"
	ravenSentry "github.com/evalphobia/logrus_sentry"
	elastic "gopkg.in/olivere/elastic.v5"
	elogrus "gopkg.in/sohlich/elogrus.v2"
)

var (
	environment string
)

const (
	elasticsearchURL   = "https://search-wisegrowth-jl3kj4kwv225mzcssitu47xlqe.us-west-2.es.amazonaws.com"
	elasticsearchIndex = "wisebot-operator"
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

var (
	log Logger = logrus.New()
)

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
		client, err := elastic.NewClient(
			elastic.SetURL(elasticsearchURL),
			elastic.SetScheme("https"),
			elastic.SetSniff(false),
		)
		if err != nil {
			log.Panic(err)
		}
		ehook, err := elogrus.NewElasticHook(client, "localhost", logrus.DebugLevel, elasticsearchIndex)
		if err != nil {
			return err
		}

		log.Hooks.Add(ehook)
	}

	levels := []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
	}

	shook, err := ravenSentry.NewAsyncWithTagsSentryHook(sentryDSN, map[string]string{"wisebot-id": wisebotID}, levels)
	if err != nil {
		return err
	}

	shook.StacktraceConfiguration.Enable = true

	log.Hooks.Add(shook)

	setLogger(log.WithField("wisebot-id", wisebotID))

	return nil
}
