package log_test

import (
	"testing"
	log "ecfmp/internal/log"
	logger "github.com/sirupsen/logrus"
)

var logLevelTest = [struct]{
	in string
	expected logger.Level
} {
	{"TRACE", logger.TraceLevel},
	{"DEBUG", logger.DebugLevel},
	{"INFO", logger.InfoLevel},
	{"WARN", logger.WarnLevel},
	{"ERROR", logger.ErrorLevel},
	{"FATAL", logger.FatalLevel},
	{"UNKOWN", logger.FatalLevel},
}

funct TestEnvToLogLevel(t *testing.T) {
	for _, tt := range logLevelTest {
		t.Run(tt.in, func(t *testing.T) {
			actual := log.EnvToLogLevel(tt.in)
			if actual != tt.expected {
				t.Errorf("EnvToLogLevel(%s): expected %s, actual %s", tt.in, tt.expected, actual)
			}
		})
	}
}
