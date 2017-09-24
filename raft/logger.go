package raft

import (
	"log"
)

// Logger for Raft
type Logger interface {
	// Add Debug when necessary
	Printf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
}

// Default Logger for raft
type DefaultLogger struct {
	*log.Logger
	// Add debug bool when necessary
}

// Printf for Logger
func (l *DefaultLogger) Printf(format string, v ...interface{}) {
	l.Printf(format, v...)
}

// Fatalf for Logger
func (l *DefaultLogger) Fatalf(format string, v ...interface{}) {
	l.Fatalf(format, v...)
}
