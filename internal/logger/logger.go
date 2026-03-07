/*
Madexchanger — HTTP/HTTPS email relay proxy for Madmail.
Copyright © 2024 The Mad Org contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Package logger provides a structured logger for madexchanger.
// It supports four log levels (debug, info, warn, error) and outputs
// timestamped, prefixed messages to stderr.
package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Level represents the severity of a log message.
type Level int

const (
	// LevelDebug is the most verbose log level. Use for development
	// and protocol-level tracing.
	LevelDebug Level = iota

	// LevelInfo is the default log level. Use for normal operational
	// messages (startup, shutdown, successful deliveries).
	LevelInfo

	// LevelWarn indicates a recoverable issue that should be investigated.
	LevelWarn

	// LevelError indicates a failure that prevented an operation
	// from completing.
	LevelError
)

// Logger wraps the standard library logger with level-based filtering.
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a new Logger with output to stderr and the given minimum
// log level. Messages below the configured level are silently discarded.
func New(level string) *Logger {
	return &Logger{
		level:  parseLevel(level),
		logger: log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds),
	}
}

// parseLevel converts a string log level name to a Level constant.
// Defaults to LevelInfo for unrecognized values.
func parseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Debug logs a message at debug level with optional key-value pairs.
func (l *Logger) Debug(msg string, kvs ...interface{}) {
	if l.level <= LevelDebug {
		l.log("DEBUG", msg, kvs...)
	}
}

// Info logs a message at info level with optional key-value pairs.
func (l *Logger) Info(msg string, kvs ...interface{}) {
	if l.level <= LevelInfo {
		l.log("INFO", msg, kvs...)
	}
}

// Warn logs a message at warn level with optional key-value pairs.
func (l *Logger) Warn(msg string, kvs ...interface{}) {
	if l.level <= LevelWarn {
		l.log("WARN", msg, kvs...)
	}
}

// Error logs a message at error level with optional key-value pairs.
func (l *Logger) Error(msg string, kvs ...interface{}) {
	if l.level <= LevelError {
		l.log("ERROR", msg, kvs...)
	}
}

// log formats and writes a log entry. Key-value pairs are appended as
// space-separated key=value tokens after the message text.
func (l *Logger) log(level, msg string, kvs ...interface{}) {
	if len(kvs) == 0 {
		l.logger.Printf("[%s] %s", level, msg)
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[%s] %s", level, msg))
	for i := 0; i+1 < len(kvs); i += 2 {
		b.WriteString(fmt.Sprintf(" %v=%v", kvs[i], kvs[i+1]))
	}
	// Handle odd number of kvs (trailing key without value).
	if len(kvs)%2 != 0 {
		b.WriteString(fmt.Sprintf(" %v=<MISSING>", kvs[len(kvs)-1]))
	}
	l.logger.Print(b.String())
}
