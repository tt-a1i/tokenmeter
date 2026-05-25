package render

import (
	"io"

	"golang.org/x/term"
)

const compactWidthThreshold = 120

type fdWriter interface {
	Fd() uintptr
}

var terminalWidth = detectTerminalWidth

func detectTerminalWidth(w io.Writer) (int, bool) {
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
