// Package colorlog has various Print functions that can be called to change the color of the text in standard out
package colorlog

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	logger     *log.Logger
	loggerOnce sync.Once
)

// getLogger returns the configured logger instance (singleton)
func getLogger() *log.Logger {
	loggerOnce.Do(func() {
		logger = log.NewWithOptions(os.Stdout, log.Options{
			ReportTimestamp: false,
			ReportCaller:    false,
		})

		// Configure based on GHORG_COLOR setting
		if os.Getenv("GHORG_COLOR") != "enabled" {
			// Disable colors by using a no-color profile
			logger.SetStyles(log.DefaultStyles())
			logger.SetColorProfile(0) // No color
		}
	})
	return logger
}

// formatMsg converts any type to string
func formatMsg(msg any) string {
	switch v := msg.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		return fmt.Sprint(v)
	}
}

// PrintInfo prints yellow colored text to standard out
func PrintInfo(msg any) {
	if os.Getenv("GHORG_QUIET") == "true" {
		return
	}

	if os.Getenv("GHORG_COLOR") == "enabled" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
		fmt.Println(style.Render(formatMsg(msg)))
	} else {
		fmt.Println(formatMsg(msg))
	}
}

// PrintSuccess prints green colored text to standard out
func PrintSuccess(msg any) {
	if os.Getenv("GHORG_COLOR") == "enabled" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
		fmt.Println(style.Render(formatMsg(msg)))
	} else {
		fmt.Println(formatMsg(msg))
	}
}

// PrintError prints red colored text to standard out
func PrintError(msg any) {
	if os.Getenv("GHORG_COLOR") == "enabled" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
		fmt.Println(style.Render(formatMsg(msg)))
	} else {
		fmt.Println(formatMsg(msg))
	}
}

// PrintErrorAndExit prints red colored text to standard out then exits 1
func PrintErrorAndExit(msg any) {
	PrintError(msg)
	os.Exit(1)
}

// PrintSubtleInfo prints magenta colored text to standard out
func PrintSubtleInfo(msg any) {
	if os.Getenv("GHORG_QUIET") == "true" {
		return
	}

	if os.Getenv("GHORG_COLOR") == "enabled" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // Magenta
		fmt.Println(style.Render(formatMsg(msg)))
	} else {
		fmt.Println(formatMsg(msg))
	}
}

// For future use - structured logging functions using charmbracelet/log

// Info logs an info message with structured logging
func Info(msg string, keyvals ...interface{}) {
	if os.Getenv("GHORG_QUIET") == "true" {
		return
	}
	getLogger().Info(msg, keyvals...)
}

// Error logs an error message with structured logging
func Error(msg string, keyvals ...interface{}) {
	getLogger().Error(msg, keyvals...)
}

// Debug logs a debug message with structured logging
func Debug(msg string, keyvals ...interface{}) {
	if os.Getenv("GHORG_DEBUG") == "true" {
		getLogger().Debug(msg, keyvals...)
	}
}

// Warn logs a warning message with structured logging
func Warn(msg string, keyvals ...interface{}) {
	getLogger().Warn(msg, keyvals...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, keyvals ...interface{}) {
	getLogger().Fatal(msg, keyvals...)
}

// SetTimeFunction allows setting a custom time function (useful for testing)
func SetTimeFunction(f log.TimeFunction) {
	getLogger().SetTimeFunction(f)
}
