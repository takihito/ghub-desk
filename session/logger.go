package session

import (
	"io"
	"log"
	"os"
)

var (
	// debugLogger is the logger for debug messages.
	// By default, it discards output.
	debugLogger = log.New(io.Discard, "DEBUG ", log.LstdFlags|log.Lshortfile)
)

// EnableDebug enables debug logging by setting the output to stderr.
func EnableDebug() {
	debugLogger.SetOutput(os.Stderr)
}

// Debugf formats and writes a debug message if debug logging is enabled.
func Debugf(format string, v ...interface{}) {
	debugLogger.Printf(format, v...)
}
