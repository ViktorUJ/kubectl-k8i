package terminal

import (
	"os"

	"golang.org/x/term"
)

// GetTerminalWidth returns the current terminal width in columns.
// Uses golang.org/x/term which abstracts POSIX ioctl and Windows Console API.
// Returns defaultWidth if detection fails (e.g., stdout is piped to a file).
func GetTerminalWidth(defaultWidth int) int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return defaultWidth
	}
	return width
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}
