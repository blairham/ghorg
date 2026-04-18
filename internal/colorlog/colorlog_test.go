package colorlog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures stdout output during function execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestFormatMsg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"error", fmt.Errorf("test error"), "test error"},
		{"int", 42, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatMsg(tt.input)
			if got != tt.expected {
				t.Errorf("formatMsg(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPrintInfo(t *testing.T) {
	os.Setenv("GHORG_QUIET", "")
	os.Setenv("GHORG_COLOR", "")
	defer os.Unsetenv("GHORG_QUIET")
	defer os.Unsetenv("GHORG_COLOR")

	output := captureStdout(t, func() {
		PrintInfo("test info message")
	})
	if !strings.Contains(output, "test info message") {
		t.Errorf("expected output to contain 'test info message', got %q", output)
	}
}

func TestPrintInfo_Quiet(t *testing.T) {
	os.Setenv("GHORG_QUIET", "true")
	defer os.Unsetenv("GHORG_QUIET")

	output := captureStdout(t, func() {
		PrintInfo("should not appear")
	})
	if output != "" {
		t.Errorf("expected no output in quiet mode, got %q", output)
	}
}

func TestPrintSuccess(t *testing.T) {
	os.Setenv("GHORG_COLOR", "")
	defer os.Unsetenv("GHORG_COLOR")

	output := captureStdout(t, func() {
		PrintSuccess("success message")
	})
	if !strings.Contains(output, "success message") {
		t.Errorf("expected output to contain 'success message', got %q", output)
	}
}

func TestPrintError(t *testing.T) {
	os.Setenv("GHORG_COLOR", "")
	defer os.Unsetenv("GHORG_COLOR")

	output := captureStdout(t, func() {
		PrintError("error message")
	})
	if !strings.Contains(output, "error message") {
		t.Errorf("expected output to contain 'error message', got %q", output)
	}
}

func TestPrintSubtleInfo(t *testing.T) {
	os.Setenv("GHORG_QUIET", "")
	os.Setenv("GHORG_COLOR", "")
	defer os.Unsetenv("GHORG_QUIET")
	defer os.Unsetenv("GHORG_COLOR")

	output := captureStdout(t, func() {
		PrintSubtleInfo("subtle message")
	})
	if !strings.Contains(output, "subtle message") {
		t.Errorf("expected output to contain 'subtle message', got %q", output)
	}
}

func TestPrintSubtleInfo_Quiet(t *testing.T) {
	os.Setenv("GHORG_QUIET", "true")
	defer os.Unsetenv("GHORG_QUIET")

	output := captureStdout(t, func() {
		PrintSubtleInfo("should not appear")
	})
	if output != "" {
		t.Errorf("expected no output in quiet mode, got %q", output)
	}
}

func TestPrintError_WithError(t *testing.T) {
	os.Setenv("GHORG_COLOR", "")
	defer os.Unsetenv("GHORG_COLOR")

	output := captureStdout(t, func() {
		PrintError(fmt.Errorf("test error value"))
	})
	if !strings.Contains(output, "test error value") {
		t.Errorf("expected output to contain 'test error value', got %q", output)
	}
}
