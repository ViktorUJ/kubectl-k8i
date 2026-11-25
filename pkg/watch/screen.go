package watch

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

const (
	escClearScreen = "\033[H\033[2J" // cursor home + clear screen
	escClearLine   = "\033[2K"       // clear the whole current line
	escClearBelow  = "\033[J"        // clear from cursor to end of screen
	escReset       = "\033[0m"
)

// screen redraws a terminal view in place.
//
// Only lines that differ from the previous frame are rewritten, so text on
// unchanged lines stays put and a mouse selection on it (e.g. a pod name being
// copied) is not disturbed by a refresh. The screen is cleared only on the
// first frame and when the terminal is resized. Output is clipped to the
// terminal size so the terminal never scrolls and old frames do not pile up in
// the scrollback.
type screen struct {
	w      io.Writer
	prev   []string
	width  int
	height int
	drawn  bool
}

func newScreen(w io.Writer) *screen { return &screen{w: w} }

// moveTo returns the escape sequence that moves the cursor to row (1-based), column 1.
func moveTo(row int) string { return fmt.Sprintf("\033[%d;1H", row) }

// Draw renders content (newline-separated lines) for a terminal of the given
// size. width/height <= 0 means unknown (no clipping in that dimension).
func (s *screen) Draw(content string, width, height int) error {
	lines := fitLines(content, width, height)

	full := !s.drawn || width != s.width || height != s.height

	var buf bytes.Buffer
	if full {
		buf.WriteString(escClearScreen)
	}
	for i, line := range lines {
		if !full && i < len(s.prev) && s.prev[i] == line {
			continue
		}
		buf.WriteString(moveTo(i + 1))
		buf.WriteString(escClearLine)
		buf.WriteString(line)
	}
	if !full && len(s.prev) > len(lines) {
		buf.WriteString(moveTo(len(lines) + 1))
		buf.WriteString(escClearBelow)
	}
	// Park the cursor below the content, so Ctrl+C and the shell prompt
	// appear after the last frame, which stays visible after exit.
	buf.WriteString(moveTo(len(lines) + 1))

	if _, err := s.w.Write(buf.Bytes()); err != nil {
		return err
	}
	s.prev, s.width, s.height, s.drawn = lines, width, height, true
	return nil
}

// fitLines splits content into lines, clips each line to width visible
// characters and keeps at most height-1 lines (the last row is left for the
// cursor), replacing the overflow with a footer line.
func fitLines(content string, width, height int) []string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")

	if maxLines := height - 1; height > 0 && len(lines) > maxLines {
		keep := max(maxLines-1, 0)
		hidden := len(lines) - keep
		footer := fmt.Sprintf("... %d more lines not shown: enlarge the terminal, use --namespaces or disable sections", hidden)
		lines = append(lines[:keep:keep], footer)
		if maxLines < 1 {
			lines = lines[:0]
		}
	}

	if width > 0 {
		for i, l := range lines {
			lines[i] = clipVisible(l, width)
		}
	}
	return lines
}

// clipVisible truncates s to at most width visible characters, ignoring ANSI
// escape sequences. A clipped line ends with "…" and, if it contained escape
// sequences, a color reset so the color does not leak into the next line.
func clipVisible(s string, width int) string {
	if displayWidth(s) <= width {
		return s
	}
	if width <= 0 {
		return ""
	}

	var b strings.Builder
	visible, hasEsc, inEsc := 0, false, false
	for _, r := range s {
		switch {
		case r == '\033':
			inEsc, hasEsc = true, true
			b.WriteRune(r)
			continue
		case inEsc:
			b.WriteRune(r)
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if visible == width-1 {
			break
		}
		b.WriteRune(r)
		visible++
	}
	b.WriteString("…")
	if hasEsc {
		b.WriteString(escReset)
	}
	return b.String()
}

// displayWidth returns the number of visible runes in s, excluding ANSI escape sequences.
func displayWidth(s string) int {
	width, inEsc := 0, false
	for _, r := range s {
		switch {
		case r == '\033':
			inEsc = true
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		default:
			width++
		}
	}
	return width
}
