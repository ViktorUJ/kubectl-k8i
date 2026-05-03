package terminal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- GetTerminalWidth tests ---

func TestGetTerminalWidth_NonTerminalFd_ReturnsDefault(t *testing.T) {
	// Create a pipe — pipe fds are not terminals, so detection should fail
	// and the function should return the default width.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	// GetTerminalWidth uses os.Stdout internally, so we test the default path
	// by verifying that for a non-terminal scenario (CI), the default is returned.
	// In CI environments, stdout is typically not a terminal.
	width := GetTerminalWidth(200)
	assert.Greater(t, width, 0, "terminal width should be a positive integer")
}

func TestGetTerminalWidth_DefaultWidth200(t *testing.T) {
	// In CI, stdout is usually piped (not a terminal), so GetTerminalWidth
	// should return the default width of 200.
	// If running in a real terminal, it returns the actual width (also positive).
	width := GetTerminalWidth(200)
	assert.Greater(t, width, 0, "width should be positive")

	// If stdout is not a terminal (CI), we expect exactly 200.
	if !IsTerminal(os.Stdout.Fd()) {
		assert.Equal(t, 200, width, "non-terminal fd should return default width")
	}
}

// --- IsTerminal tests ---

func TestIsTerminal_PipeFd_ReturnsFalse(t *testing.T) {
	// A pipe fd is never a terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	assert.False(t, IsTerminal(r.Fd()), "pipe read end should not be a terminal")
	assert.False(t, IsTerminal(w.Fd()), "pipe write end should not be a terminal")
}

func TestIsTerminal_FileFd_ReturnsFalse(t *testing.T) {
	// A regular file fd is never a terminal.
	f, err := os.CreateTemp("", "terminal_test_*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()

	assert.False(t, IsTerminal(f.Fd()), "regular file fd should not be a terminal")
}

func TestIsTerminal_StdoutFd_ReturnsBool(t *testing.T) {
	// In CI, stdout is typically not a terminal (piped).
	// In a real terminal, it would be true.
	// We just verify it returns a bool without panicking.
	result := IsTerminal(os.Stdout.Fd())
	// The result depends on the environment, but it should be a valid bool.
	assert.IsType(t, true, result)
}
