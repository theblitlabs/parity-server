package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	log zerolog.Logger
	mu  sync.RWMutex
)

type LogLevel string

const (
	LogLevelDebug    LogLevel = "debug"
	LogLevelInfo     LogLevel = "info"
	LogLevelWarn     LogLevel = "warn"
	LogLevelError    LogLevel = "error"
	LogLevelDisabled LogLevel = "disabled"
)

type LogMode string

const (
	LogModeDebug  LogMode = "debug"
	LogModePretty LogMode = "pretty"
	LogModeInfo   LogMode = "info"
	LogModeProd   LogMode = "prod"
	LogModeTest   LogMode = "test"
)

type Config struct {
	Level         LogLevel
	Pretty        bool
	TimeFormat    string
	CallerEnabled bool
	NoColor       bool
}

func DefaultConfig() Config {
	return Config{
		Level:         LogLevelInfo,
		Pretty:        false,
		TimeFormat:    time.RFC3339,
		CallerEnabled: true,
		NoColor:       false,
	}
}

func ConfigForMode(mode LogMode) Config {
	switch mode {
	case LogModeDebug:
		return Config{
			Level:         LogLevelDebug,
			Pretty:        true,
			TimeFormat:    time.RFC3339,
			CallerEnabled: true,
			NoColor:       false,
		}
	case LogModePretty:
		return Config{
			Level:         LogLevelInfo,
			Pretty:        true,
			TimeFormat:    time.RFC3339,
			CallerEnabled: true,
			NoColor:       false,
		}
	case LogModeInfo:
		return Config{
			Level:         LogLevelInfo,
			Pretty:        false,
			TimeFormat:    time.RFC3339,
			CallerEnabled: true,
			NoColor:       false,
		}
	case LogModeProd:
		return Config{
			Level:         LogLevelInfo,
			Pretty:        false,
			TimeFormat:    time.RFC3339Nano,
			CallerEnabled: false,
			NoColor:       true,
		}
	case LogModeTest:
		return Config{
			Level:         LogLevelError,
			Pretty:        false,
			TimeFormat:    time.RFC3339,
			CallerEnabled: false,
			NoColor:       true,
		}
	default:
		return DefaultConfig()
	}
}

func InitWithMode(mode LogMode) {
	Init(ConfigForMode(mode))
}

func Init(cfg Config) {
	mu.Lock()
	defer mu.Unlock()

	if cfg.Level == LogLevelDisabled {
		zerolog.SetGlobalLevel(zerolog.Disabled)
		log = zerolog.New(io.Discard).With().Logger()
		zerolog.DefaultContextLogger = &log
		return
	}

	var output io.Writer = os.Stdout

	if cfg.Pretty {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: cfg.TimeFormat,
			NoColor:    cfg.NoColor,
			FormatLevel: func(i interface{}) string {
				return colorizeLevel(i.(string))
			},
			FormatFieldName: func(i interface{}) string {
				name := fmt.Sprint(i)
				// Skip component and trace_id if they're going to have empty values
				if name == "component" || name == "trace_id" {
					return ""
				}
				return colorize(fmt.Sprintf("%s=", name), dim+cyan)
			},
			FormatFieldValue: func(i interface{}) string {
				switch v := i.(type) {
				case string:
					if v == "" {
						return ""
					}
					return colorize(v, blue)
				case json.Number:
					return colorize(v.String(), magenta)
				case error:
					return colorize(v.Error(), red)
				case nil:
					return ""
				default:
					s := fmt.Sprint(v)
					if s == "" {
						return ""
					}
					return colorize(s, blue)
				}
			},
			FormatMessage: func(i interface{}) string {
				msg := fmt.Sprint(i)
				msg = strings.Replace(msg, "Request started", colorize("→", bold+green), 1)
				msg = strings.Replace(msg, "Request completed", colorize("←", bold+green), 1)
				return colorize(msg, bold)
			},
			FormatTimestamp: func(i interface{}) string {
				t := fmt.Sprint(i)
				return colorize(t, dim+gray)
			},
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.CallerFieldName,
				zerolog.MessageFieldName,
				"device_id",
			},
			PartsExclude: []string{
				"query",
				"referer",
				"user_agent",
				"remote_addr",
				"duration_human",
				"component",
				"trace_id",
			},
		}
		output = consoleWriter
	}

	switch cfg.Level {
	case LogLevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LogLevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LogLevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LogLevelError:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	zerolog.TimeFieldFormat = cfg.TimeFormat

	// Create the logger with or without caller info
	logCtx := zerolog.New(output).With().Timestamp()
	if cfg.CallerEnabled {
		logCtx = logCtx.Caller()
	}

	log = logCtx.Logger()
	zerolog.DefaultContextLogger = &log
}

const (
	gray    = "\x1b[37m"
	blue    = "\x1b[34m"
	cyan    = "\x1b[36m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	magenta = "\x1b[35m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	reset   = "\x1b[0m"
)

func colorize(s, color string) string {
	return color + s + reset
}

func colorizeLevel(level string) string {
	switch level {
	case "debug":
		return colorize("DBG", dim+magenta)
	case "info":
		return colorize("INF", bold+green)
	case "warn":
		return colorize("WRN", bold+yellow)
	case "error":
		return colorize("ERR", bold+red)
	case "fatal":
		return colorize("FTL", bold+red+"\x1b[7m")
	default:
		return colorize(level, blue)
	}
}

func Get() zerolog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return log
}

func WithComponent(component string) zerolog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return log.With().Str("component", component).Logger()
}

func WithTraceID(traceID string) zerolog.Logger {
	return log.With().Str("trace_id", traceID).Logger()
}

func Error(component string, err error, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Error().Err(err)
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}

func Info(component string, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Info()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}

func Debug(component string, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Debug()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}

func Warn(component string, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Warn()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}
