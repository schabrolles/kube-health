package print

// Helper functions for printing to the terminal.

import (
	"fmt"
	"strings"
)

const (
	// RESET is the escape sequence for unsetting any previous commands.
	RESET = 0
	// ESC is the escape sequence used to send ANSI commands in the terminal.
	ESC = 27
)

// color is a type that captures the ANSI code for colors on the
// terminal.
type Color int

var (
	RED    Color = 31
	GREEN  Color = 32
	YELLOW Color = 33
)

// SprintfWithColor formats according to the provided pattern and returns
// the result as a string with the necessary ansii escape codes for
// color
func SprintfWithColor(color Color, format string, a ...interface{}) string {
	return fmt.Sprintf("%c[%dm", ESC, color) +
		fmt.Sprintf(format, a...) +
		fmt.Sprintf("%c[%dm", ESC, RESET)
}

// HighlightLogs applies color coding to log lines based on keywords.
// If useColor is false, returns logs unchanged.
// Applies colors based on keywords:
//   - Red: "error", "fatal", "panic", "exception", "failed", "failure"
//   - Yellow: "warning", "warn", "deprecated"
// Uses case-insensitive matching and colors the entire line when a keyword is found.
func HighlightLogs(logs string, useColor bool) string {
	if !useColor || logs == "" {
		return logs
	}

	// Keywords that trigger red color
	redKeywords := []string{"error", "fatal", "panic", "exception", "failed", "failure"}
	// Keywords that trigger yellow color
	yellowKeywords := []string{"warning", "warn", "deprecated"}

	lines := strings.Split(logs, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}

		lowerLine := strings.ToLower(line)

		// Check for red keywords first (higher priority)
		colored := false
		for _, keyword := range redKeywords {
			if strings.Contains(lowerLine, keyword) {
				lines[i] = SprintfWithColor(RED, "%s", line)
				colored = true
				break
			}
		}

		// If not colored red, check for yellow keywords
		if !colored {
			for _, keyword := range yellowKeywords {
				if strings.Contains(lowerLine, keyword) {
					lines[i] = SprintfWithColor(YELLOW, "%s", line)
					break
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}
