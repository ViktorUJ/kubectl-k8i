package watch

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vt is a minimal terminal emulator for the escape sequences emitted by
// screen: CUP (ESC[r;1H), ED (ESC[2J, ESC[J), EL (ESC[2K) and SGR (ignored).
// It also records which rows were written, so tests can assert that
// unchanged rows were not touched.
type vt struct {
	rows    []string
	touched map[int]bool
}

var csi = regexp.MustCompile(`^\033\[([0-9;]*)([A-Za-z])`)

func newVT(height int) *vt { return &vt{rows: make([]string, height), touched: map[int]bool{}} }

func (v *vt) feed(t *testing.T, data string) {
	t.Helper()
	v.touched = map[int]bool{}
	row := 0
	for len(data) > 0 {
		if m := csi.FindStringSubmatch(data); m != nil {
			data = data[len(m[0]):]
			switch m[2] {
			case "H":
				r := 1
				if m[1] != "" {
					_, _ = fmt.Sscanf(m[1], "%d;", &r)
				}
				row = r - 1
				require.Less(t, row, len(v.rows)+1, "cursor moved past the screen")
			case "J":
				if m[1] == "2" {
					for i := range v.rows {
						v.rows[i] = ""
					}
				} else {
					for i := row; i < len(v.rows); i++ {
						v.rows[i] = ""
					}
				}
			case "K":
				v.rows[row] = ""
				v.touched[row] = true
			case "m":
			default:
				t.Fatalf("unexpected escape sequence %q", m[0])
			}
			continue
		}
		next := strings.IndexByte(data, '\033')
		if next < 0 {
			next = len(data)
		}
		text := data[:next]
		require.NotContains(t, text, "\n", "screen must position rows explicitly, not rely on newlines")
		v.rows[row] += text
		v.touched[row] = true
		data = data[next:]
	}
}

func (v *vt) text() string { return strings.TrimRight(strings.Join(v.rows, "\n"), "\n") }

func TestScreen_DrawAndIncrementalUpdate(t *testing.T) {
	var out bytes.Buffer
	s := newScreen(&out)
	term := newVT(10)

	require.NoError(t, s.Draw("title 1\nline A\nline B\nline C\n", 80, 10))
	term.feed(t, out.String())
	assert.Equal(t, "title 1\nline A\nline B\nline C", term.text())
	assert.Contains(t, out.String(), escClearScreen)

	out.Reset()
	require.NoError(t, s.Draw("title 2\nline A\nline B changed\nline C\n", 80, 10))
	assert.NotContains(t, out.String(), escClearScreen, "no full clear on a normal refresh")
	assert.NotContains(t, out.String(), "line A", "unchanged lines are not rewritten")
	term.feed(t, out.String())
	assert.Equal(t, "title 2\nline A\nline B changed\nline C", term.text())
	assert.False(t, term.touched[1], "row of 'line A' must stay untouched so a selection on it survives")
	assert.False(t, term.touched[3])

	// Shrinking content clears the leftover rows.
	out.Reset()
	require.NoError(t, s.Draw("title 3\nline A\n", 80, 10))
	term.feed(t, out.String())
	assert.Equal(t, "title 3\nline A", term.text())

	// Identical frame: only the cursor is parked, nothing is redrawn.
	out.Reset()
	require.NoError(t, s.Draw("title 3\nline A\n", 80, 10))
	assert.Equal(t, moveTo(3), out.String())
}

func TestScreen_ResizeTriggersFullRedraw(t *testing.T) {
	var out bytes.Buffer
	s := newScreen(&out)
	require.NoError(t, s.Draw("a\nb\n", 80, 10))
	out.Reset()
	require.NoError(t, s.Draw("a\nb\n", 100, 10))
	assert.Contains(t, out.String(), escClearScreen)
	out.Reset()
	require.NoError(t, s.Draw("a\nb\n", 100, 20))
	assert.Contains(t, out.String(), escClearScreen)
}

func TestScreen_ClipsToTerminalSize(t *testing.T) {
	var content strings.Builder
	for i := range 30 {
		fmt.Fprintf(&content, "row %02d %s\n", i, strings.Repeat("x", 50))
	}
	var out bytes.Buffer
	s := newScreen(&out)
	term := newVT(10)
	require.NoError(t, s.Draw(content.String(), 20, 10))
	term.feed(t, out.String())

	lines := strings.Split(term.text(), "\n")
	require.Len(t, lines, 9, "height-1 rows used, last row kept for the cursor so the terminal never scrolls")
	assert.Equal(t, "row 00 xxxxxxxxxxxx…", lines[0])
	assert.True(t, strings.HasPrefix(lines[8], "... 22 more lines"), lines[8])
	for _, l := range lines {
		assert.LessOrEqual(t, displayWidth(l), 20, l)
	}
}

func TestClipVisible(t *testing.T) {
	assert.Equal(t, "short", clipVisible("short", 10))
	assert.Equal(t, "abcdefghi…", clipVisible("abcdefghijklmnop", 10))
	assert.Equal(t, "", clipVisible("abc", 0))

	colored := "\033[0;31mCrashLoopBackOff\033[0m tail"
	assert.Equal(t, colored, clipVisible(colored, 30), "fits: unchanged")
	got := clipVisible(colored, 6)
	assert.Equal(t, 6, displayWidth(got))
	assert.Equal(t, "\033[0;31mCrash…"+escReset, got, "ANSI codes do not count and color is reset after clipping")
}

func TestFitLines_TinyTerminal(t *testing.T) {
	assert.Empty(t, fitLines("a\nb\nc", 80, 1))
	lines := fitLines("a\nb\nc", 80, 2)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "3 more lines")
	assert.Len(t, fitLines("a\nb\nc", 80, 0), 3, "unknown height: no limit")
}
