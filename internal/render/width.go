package render

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const compactWidthThreshold = 120

type fdWriter interface {
	Fd() uintptr
}

var terminalWidth = detectTerminalWidth

// detectTerminalWidth returns the effective terminal width with the
// precedence ccusage v20 establishes
// (rust/crates/ccusage-terminal/src/terminal.rs:5-19): COLUMNS env first,
// then live TTY detection, then "unavailable" which compactLayout treats
// as full-layout default. The COLUMNS path lets scripted / piped runs
// drive the compact/full switch even though the writer is not a TTY;
// the existing non-TTY-defaults-to-full contract is preserved when the
// env var is absent or unparseable.
//
// ANSI escape sequences and Unicode display width inside table cells
// are handled by go-pretty/v6/text (used by every renderer in this
// package), so no separate visible-width helper is added here.
func detectTerminalWidth(w io.Writer) (int, bool) {
	if env := strings.TrimSpace(os.Getenv("COLUMNS")); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			return n, true
		}
	}
	f, ok := w.(fdWriter)
	if !ok {
		return 0, false
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return 0, false
	}
	width, _, err := term.GetSize(fd)
	if err != nil {
		return 0, false
	}
	return width, true
}

func compactLayout(w io.Writer, opts Options) bool {
	if opts.Compact {
		return true
	}
	width, ok := terminalWidth(w)
	return ok && width < compactWidthThreshold
}
