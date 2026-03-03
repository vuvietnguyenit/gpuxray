package logging

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

type Config struct {
	Level  string // debug, info, warn, error
	Format string // auto, json, console
}

// Init initializes the global logger.
// Call this ONCE from main().
func Init(cfg Config) {
	level := parseLevel(cfg.Level)

	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339

	var out io.Writer = os.Stderr

	switch cfg.Format {
	case "json":
		log.Logger = zerolog.New(out).With().Timestamp().Logger()

	case "console":
		log.Logger = zerolog.New(
			zerolog.ConsoleWriter{
				Out:        out,
				TimeFormat: "15:04:05",
			},
		).With().Timestamp().Logger()

	default: // auto
		if isTerminal(out) {
			log.Logger = zerolog.New(
				zerolog.ConsoleWriter{
					Out:        out,
					TimeFormat: "15:04:05",
				},
			).With().Timestamp().Logger()
		} else {
			log.Logger = zerolog.New(out).With().Timestamp().Logger()
		}
	}
}

// L returns the global logger (shortcut).
func L() *zerolog.Logger {
	l := log.Logger
	return &l
}

// With returns a child logger with common fields.
func With(fields map[string]any) zerolog.Logger {
	l := log.Logger
	for k, v := range fields {
		l = l.With().Interface(k, v).Logger()
	}
	return l
}

func parseLevel(lvl string) zerolog.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return zerolog.DebugLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	default:
		return zerolog.InfoLevel
	}
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}
