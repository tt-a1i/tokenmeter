package render

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Resolve decides whether color output should be enabled. Order of
// precedence (highest first):
//  1. JSON mode → off (avoid polluting pipes)
//  2. --no-color flag → off
//  3. NO_COLOR env var (any non-empty value) → off
//  4. FORCE_COLOR env var → on
//  5. stdout/writer is a TTY → on
//  6. otherwise → off
func Resolve(jsonMode bool, noColorFlag bool, w io.Writer) bool {
	if jsonMode {
		return false
	}
	if noColorFlag {
		return false
	}
	if v := os.Getenv("NO_COLOR"); v != "" {
		return false
	}
	if v := os.Getenv("FORCE_COLOR"); v != "" {
		return true
	}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return true
	}
	return false
}
