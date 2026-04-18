package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsStringInSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s        string
		sl       []string
		expected bool
	}{
		{
			name:     "found in slice",
			s:        "foo",
			sl:       []string{"foo", "bar", "baz"},
			expected: true,
		},
		{
			name:     "not found in slice",
			s:        "qux",
			sl:       []string{"foo", "bar", "baz"},
			expected: false,
		},
		{
			name:     "empty slice",
			s:        "foo",
			sl:       []string{},
			expected: false,
		},
		{
			name:     "empty string matches empty element",
			s:        "",
			sl:       []string{"", "foo"},
			expected: true,
		},
		{
			name:     "case sensitive",
			s:        "Foo",
			sl:       []string{"foo", "bar"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsStringInSlice(tt.s, tt.sl)
			if got != tt.expected {
				t.Errorf("IsStringInSlice(%q, %v) = %v, want %v", tt.s, tt.sl, got, tt.expected)
			}
		})
	}
}

func TestCalculateDirSizeInMb(t *testing.T) {
	t.Parallel()

	t.Run("empty directory", func(t *testing.T) {
		t.Parallel()
		dir, err := os.MkdirTemp("", "ghorg-test-empty")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		size, err := CalculateDirSizeInMb(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size != 0 {
			t.Errorf("expected 0 MB for empty dir, got %f", size)
		}
	})

	t.Run("directory with files", func(t *testing.T) {
		t.Parallel()
		dir, err := os.MkdirTemp("", "ghorg-test-files")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		// Create a 1000-byte file
		data := make([]byte, 1000)
		if err := os.WriteFile(filepath.Join(dir, "test.txt"), data, 0o644); err != nil {
			t.Fatal(err)
		}

		size, err := CalculateDirSizeInMb(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 1000 bytes = 0.001 MB (using 1000*1000 denominator)
		expected := 0.001
		if size != expected {
			t.Errorf("expected %f MB, got %f", expected, size)
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		t.Parallel()
		_, err := CalculateDirSizeInMb("/nonexistent/path/ghorg-test")
		if err == nil {
			t.Error("expected error for nonexistent path, got nil")
		}
	})
}
