package logging

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var LOGGER *zerolog.Logger                      // Global logger instance
var LogFolder string                            // Global log folder path
var LogLevel zerolog.Level = zerolog.DebugLevel // -1: Trace, 0: Debug, 1: Info, 2: Warn, 3: Error
var LogFilePath string                          // Global log file path
var logFile = &logFileWriter{}                  // Log file writer, handles rotation and cleanup

// logFileWriter writes to the current log file.
// Lumberjack reads its settings from another goroutine, so it is replaced instead of changed.
type logFileWriter struct {
	mu   sync.RWMutex
	file *lumberjack.Logger
}

func (w *logFileWriter) Write(p []byte) (int, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.file.Write(p)
}

// setKeep replaces the log file writer if the keep settings changed. 0 keeps the current value.
func (w *logFileWriter) setKeep(days, files int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if days <= 0 {
		days = w.file.MaxAge
	}
	if files <= 0 {
		files = w.file.MaxBackups
	}
	if days == w.file.MaxAge && files == w.file.MaxBackups {
		return
	}
	old := w.file
	w.file = newLumberjackLogger(days, files)
	_ = old.Close()
}

// newLumberjackLogger creates the log file writer
func newLumberjackLogger(keepDays, keepFiles int) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   LogFilePath, // log file path
		MaxSize:    25,          // megabytes
		MaxBackups: keepFiles,   // number of backups
		MaxAge:     keepDays,    // days
		LocalTime:  true,        // use local time
		Compress:   false,       // disabled by default
	}
}

var (
	devMode   atomic.Bool
	nopLogger = zerolog.New(io.Discard)
)

// SetDevMode should be called from main during startup.
func SetDevMode(enabled bool) {
	if enabled {
		fmt.Println("Dev mode enabled: TRACE logging is active")
	}
	devMode.Store(enabled)
}

// Dev returns a TRACE event only when dev mode is enabled.
// When disabled, it returns a no-op event (safe to call .Msg()).
func Dev() *zerolog.Event {
	if !devMode.Load() || LOGGER == nil {
		return nopLogger.Trace()
	}
	return LOGGER.Trace().Timestamp().Str("dev", "aura")
}

// DevMsg is a convenience wrapper.
func DevMsg(msg string) {
	Dev().Msg(msg)
}

// DevMsgf is a convenience wrapper.
func DevMsgf(format string, v ...any) {
	Dev().Msgf(format, v...)
}

// CreateLogFolder creates the log folder if it doesn't exist.
func CreateLogFolder() {
	// Create the log directory if it doesn't exist
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" && testing.Testing() {
		// When testing, use a temp folder so tests run without CONFIG_PATH set
		if dir, err := os.MkdirTemp("", "aura-test-config-"); err == nil {
			configPath = dir
			_ = os.Setenv("CONFIG_PATH", dir)
		}
	}
	if configPath == "" {
		configPath = "/config"
	}

	logPath := path.Join(configPath, "logs")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		err := os.MkdirAll(logPath, 0755)
		if err != nil {
			fmt.Printf("Error creating log directory: %v\n", err)
		}
	}
	LogFolder = logPath

}

func init() {
	CreateLogFolder()

	LogFilePath = path.Join(LogFolder, "aura.log")

	// rotate logs using lumberjack; output is written as JSON lines (jsonl)
	logFile.file = newLumberjackLogger(7, 7)

	// Create a console writer for pretty printing to console
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006/01/02 15:04:05",
		FormatLevel: func(i any) string {
			if s, ok := i.(string); ok {
				switch strings.ToLower(s) {
				case "error":
					return "\033[31m[ERROR]\033[0m" // Red
				case "warn":
					return "\033[33m[WARN]\033[0m" // Yellow
				case "info":
					return "\033[32m[INFO]\033[0m" // Green
				case "debug":
					return "\033[36m[DEBUG]\033[0m" // Cyan
				case "trace":
					return "\033[35m[TRACE]\033[0m" // Magenta
				default:
					return fmt.Sprintf("[%s]", strings.ToUpper(s))
				}
			}
			return fmt.Sprintf("[%v]", i)
		},
	}

	// Combine log file writer and console writer
	multi := zerolog.MultiLevelWriter(logFile, consoleWriter)

	// Use the global LogLevel
	zerolog.SetGlobalLevel(LogLevel)
	logger := zerolog.New(multi).Level(LogLevel)
	LOGGER = &logger
}

// ApplyLogLevel applies the current LogLevel to the global logger.
func ApplyLogLevel() {
	zerolog.SetGlobalLevel(LogLevel)
	if LOGGER != nil {
		l := LOGGER.Level(LogLevel)
		LOGGER = &l
	}
}

// SetLogLevel sets the global log level and applies it to the logger.
func SetLogLevel(level string) {
	switch strings.ToLower(level) {
	case "trace":
		LogLevel = zerolog.TraceLevel
	case "debug":
		LogLevel = zerolog.DebugLevel
	case "info":
		LogLevel = zerolog.InfoLevel
	case "warn", "warning":
		LogLevel = zerolog.WarnLevel
	case "error":
		LogLevel = zerolog.ErrorLevel
	default:
		LogLevel = zerolog.InfoLevel
	}
	ApplyLogLevel()
}

// SetLogKeepDays sets how many days old log files are kept.
func SetLogKeepDays(days int) {
	logFile.setKeep(days, 0)
}

// SetLogKeepFiles sets how many old log files are kept.
func SetLogKeepFiles(files int) {
	logFile.setKeep(0, files)
}
