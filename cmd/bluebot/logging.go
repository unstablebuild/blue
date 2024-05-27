package main

import (
	log "github.com/sirupsen/logrus"
)

type loggingAdapter struct {
	logger *log.Logger
}

func (l loggingAdapter) Output(callDepth int, msg string) error {
	l.logger.Debug(msg)
	return nil
}
