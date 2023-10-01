package log

import (
	log "github.com/sirupsen/logrus"
)

/**
 * Given a string representation of a log level, return the logrus log level
 * that corresponds to it.
 */
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
