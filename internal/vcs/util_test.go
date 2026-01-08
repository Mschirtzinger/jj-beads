package vcs

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLines(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []string
	}{
		{
			name:     "empty input",
			input:    []byte(""),
			expected: nil,
		},
		{
			name:     "single line",
			input:    []byte("line1"),
			expected: []string{"line1"},
		},
		{
			name:     "multiple lines",
			input:    []byte("line1\nline2\nline3"),
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "lines with whitespace",
			input:    []byte("  line1  \n  line2  \n  line3  "),
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "empty lines filtered",
			input:    []byte("line1\n\nline2\n\n\nline3"),
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "trailing newline",
			input:    []byte("line1\nline2\n"),
			expected: []string{"line1", "line2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLines(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d", len(tt.expected), len(result))
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("Line %d: expected '%s', got '%s'", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected map[string]string
	}{
		{
			name:     "empty input",
			input:    []byte(""),
			expected: map[string]string{},
		},
		{
			name:  "single key-value",
			input: []byte("key: value"),
			expected: map[string]string{
				"key": "value",
			},
		},
		{
			name:  "multiple key-values",
			input: []byte("key1: value1\nkey2: value2\nkey3: value3"),
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name:  "values with colons",
			input: []byte("url: https://example.com:8080"),
			expected: map[string]string{
				"url": "https://example.com:8080",
			},
		},
		{
			name:  "whitespace trimming",
			input: []byte("  key  :  value  "),
			expected: map[string]string{
				"key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseKeyValue(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d entries, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("Key '%s': expected '%s', got '%s'", key, expectedValue, result[key])
				}
			}
		})
	}
}

func TestSplitFirstLine(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		expectedFirst string
		expectedRest  []string
	}{
		{
			name:          "empty input",
			input:         []byte(""),
			expectedFirst: "",
			expectedRest:  nil,
		},
		{
			name:          "single line",
			input:         []byte("first"),
			expectedFirst: "first",
			expectedRest:  nil,
		},
		{
			name:          "multiple lines",
			input:         []byte("first\nsecond\nthird"),
			expectedFirst: "first",
			expectedRest:  []string{"second", "third"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, rest := SplitFirstLine(tt.input)

			if first != tt.expectedFirst {
				t.Errorf("Expected first line '%s', got '%s'", tt.expectedFirst, first)
			}

			if len(rest) != len(tt.expectedRest) {
				t.Errorf("Expected %d remaining lines, got %d", len(tt.expectedRest), len(rest))
				return
			}

			for i, line := range rest {
				if line != tt.expectedRest[i] {
					t.Errorf("Line %d: expected '%s', got '%s'", i, tt.expectedRest[i], line)
				}
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		baseDir  string
		expected string
		wantErr  bool
	}{
		{
			name:    "empty path",
			path:    "",
			baseDir: "/base",
			wantErr: true,
		},
		{
			name:     "absolute path",
			path:     "/absolute/path",
			baseDir:  "/base",
			expected: "/absolute/path",
			wantErr:  false,
		},
		{
			name:     "relative path",
			path:     "relative/path",
			baseDir:  "/base",
			expected: "/base/relative/path",
			wantErr:  false,
		},
		{
			name:    "relative path without base",
			path:    "relative/path",
			baseDir: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizePath(tt.path, tt.baseDir)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Normalize expected path for comparison
			expected := filepath.Clean(tt.expected)
			if result != expected {
				t.Errorf("Expected '%s', got '%s'", expected, result)
			}
		})
	}
}

func TestRelativePath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		target   string
		expected string
	}{
		{
			name:     "same directory",
			base:     "/base",
			target:   "/base",
			expected: ".",
		},
		{
			name:     "child directory",
			base:     "/base",
			target:   "/base/child",
			expected: "child",
		},
		{
			name:     "nested child",
			base:     "/base",
			target:   "/base/child/nested",
			expected: "child/nested",
		},
		{
			name:     "parent directory",
			base:     "/base/child",
			target:   "/base",
			expected: "..",
		},
		{
			name:     "sibling directory",
			base:     "/base/dir1",
			target:   "/base/dir2",
			expected: "../dir2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RelativePath(tt.base, tt.target)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		target   string
		expected bool
	}{
		{
			name:     "same directory",
			base:     "/base",
			target:   "/base",
			expected: true,
		},
		{
			name:     "child directory",
			base:     "/base",
			target:   "/base/child",
			expected: true,
		},
		{
			name:     "nested child",
			base:     "/base",
			target:   "/base/child/nested",
			expected: true,
		},
		{
			name:     "parent directory",
			base:     "/base/child",
			target:   "/base",
			expected: false,
		},
		{
			name:     "sibling directory",
			base:     "/base/dir1",
			target:   "/base/dir2",
			expected: false,
		},
		{
			name:     "completely different",
			base:     "/base",
			target:   "/other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSubPath(tt.base, tt.target)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTrimOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty",
			input:    []byte(""),
			expected: "",
		},
		{
			name:     "no whitespace",
			input:    []byte("content"),
			expected: "content",
		},
		{
			name:     "leading whitespace",
			input:    []byte("  content"),
			expected: "content",
		},
		{
			name:     "trailing whitespace",
			input:    []byte("content  "),
			expected: "content",
		},
		{
			name:     "both",
			input:    []byte("  content  "),
			expected: "content",
		},
		{
			name:     "newlines",
			input:    []byte("\n\ncontent\n\n"),
			expected: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimOutput(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFirstWord(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty",
			input:    []byte(""),
			expected: "",
		},
		{
			name:     "single word",
			input:    []byte("word"),
			expected: "word",
		},
		{
			name:     "multiple words",
			input:    []byte("first second third"),
			expected: "first",
		},
		{
			name:     "with whitespace",
			input:    []byte("  first  second  "),
			expected: "first",
		},
		{
			name:     "with newlines",
			input:    []byte("first\nsecond"),
			expected: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FirstWord(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		prefix   string
		expected bool
	}{
		{
			name:     "exact match",
			input:    []byte("prefix"),
			prefix:   "prefix",
			expected: true,
		},
		{
			name:     "has prefix",
			input:    []byte("prefix content"),
			prefix:   "prefix",
			expected: true,
		},
		{
			name:     "no prefix",
			input:    []byte("content"),
			prefix:   "prefix",
			expected: false,
		},
		{
			name:     "case insensitive",
			input:    []byte("PREFIX content"),
			prefix:   "prefix",
			expected: true,
		},
		{
			name:     "with whitespace",
			input:    []byte("  prefix content"),
			prefix:   "prefix",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPrefix(tt.input, tt.prefix)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExecSimple(t *testing.T) {
	// Test with a simple command that should always work
	output, err := ExecSimple("/tmp", "echo", "test")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "test" {
		t.Errorf("Expected 'test', got '%s'", result)
	}
}

func TestExecContext(t *testing.T) {
	ctx := context.Background()

	// Test successful execution
	output, err := ExecContext(ctx, 5*time.Second, "/tmp", "echo", "test")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "test" {
		t.Errorf("Expected 'test', got '%s'", result)
	}
}

func TestExecContextTimeout(t *testing.T) {
	ctx := context.Background()

	// Test timeout - sleep for 2 seconds with 100ms timeout
	_, err := ExecContext(ctx, 100*time.Millisecond, "/tmp", "sleep", "2")
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestExecLines(t *testing.T) {
	ctx := context.Background()

	// Test command that outputs multiple lines
	lines, err := ExecLines(ctx, 5*time.Second, "/tmp", "sh", "-c", "echo line1; echo line2; echo line3")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := []string{"line1", "line2", "line3"}
	if len(lines) != len(expected) {
		t.Errorf("Expected %d lines, got %d", len(expected), len(lines))
		return
	}

	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("Line %d: expected '%s', got '%s'", i, expected[i], line)
		}
	}
}

func TestIsExitError(t *testing.T) {
	// Test with nil error
	if IsExitError(nil) {
		t.Error("Expected false for nil error")
	}

	// Test with successful command
	err := exec.Command("echo", "test").Run()
	if IsExitError(err) {
		t.Error("Expected false for successful command")
	}

	// Test with failed command
	err = exec.Command("sh", "-c", "exit 1").Run()
	if !IsExitError(err) {
		t.Error("Expected true for failed command")
	}
}

func TestGetExitCode(t *testing.T) {
	// Test with nil error
	if code := GetExitCode(nil); code != 0 {
		t.Errorf("Expected exit code 0 for nil error, got %d", code)
	}

	// Test with successful command
	err := exec.Command("echo", "test").Run()
	if code := GetExitCode(err); code != 0 {
		t.Errorf("Expected exit code 0 for successful command, got %d", code)
	}

	// Test with failed command
	err = exec.Command("sh", "-c", "exit 42").Run()
	if code := GetExitCode(err); code != 42 {
		t.Errorf("Expected exit code 42, got %d", code)
	}
}
