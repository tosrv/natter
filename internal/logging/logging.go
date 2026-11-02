// Package logging builds the process-wide structured logger.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// NewLogger returns a JSON logger. level is debug|info|warn|error; anything
// else falls back to info.
func NewLogger(w io.Writer, level string) *slog.Logger {
	lv := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv}))
}
