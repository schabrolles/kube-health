package print

import (
	"fmt"
	"strings"
	"testing"
)

func TestHighlightLogs(t *testing.T) {
	tests := []struct {
		name     string
		logs     string
		useColor bool
		wantRed  bool
		wantYellow bool
	}{
		{
			name:     "no color when disabled",
			logs:     "error: something went wrong\nwarning: be careful",
			useColor: false,
			wantRed:  false,
			wantYellow: false,
		},
		{
			name:     "red for error keyword",
			logs:     "error: something went wrong",
			useColor: true,
			wantRed:  true,
			wantYellow: false,
		},
		{
			name:     "red for fatal keyword",
			logs:     "fatal: critical failure",
			useColor: true,
			wantRed:  true,
			wantYellow: false,
		},
		{
			name:     "yellow for warning keyword",
			logs:     "warning: be careful",
			useColor: true,
			wantRed:  false,
			wantYellow: true,
		},
		{
			name:     "case insensitive matching",
			logs:     "ERROR: Something went wrong\nWARNING: Be careful",
			useColor: true,
			wantRed:  true,
			wantYellow: true,
		},
		{
			name:     "empty logs",
			logs:     "",
			useColor: true,
			wantRed:  false,
			wantYellow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighlightLogs(tt.logs, tt.useColor)
			
			if !tt.useColor {
				if result != tt.logs {
					t.Errorf("Expected unchanged logs when color disabled, got modified")
				}
				return
			}

			// Check for ANSI color codes
			redCode := fmt.Sprintf("%c[%dm", ESC, RED)
			yellowCode := fmt.Sprintf("%c[%dm", ESC, YELLOW)
			
			hasRed := strings.Contains(result, redCode)
			hasYellow := strings.Contains(result, yellowCode)

			if hasRed != tt.wantRed {
				t.Errorf("Expected red=%v, got red=%v\nResult: %q", tt.wantRed, hasRed, result)
			}
			if hasYellow != tt.wantYellow {
				t.Errorf("Expected yellow=%v, got yellow=%v\nResult: %q", tt.wantYellow, hasYellow, result)
			}
		})
	}
}

// Made with Bob
