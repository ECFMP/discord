package log

import (
	log "github.com/sirupsen/logrus"
)

func EnvToLogLevel(envLevel string) log.Level {
	switch envLevel {
	case "TRACE":
		return log.TraceLevel
	case "DEBUG":
		return log.DebugLevel
	case "INFO":
		return log.InfoLevel
	case "WARN":
		return log.WarnLevel
	case "ERROR":
		return log.ErrorLevel
	case "FATAL":
		return log.FatalLevel
	default:
		return log.FatalLevel
	}
}
