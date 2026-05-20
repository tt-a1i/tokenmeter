package render

import (
	"bytes"
	"os"
	"testing"
)

func TestResolveColorJSONForcesOff(t *testing.T) {
	if Resolve(true, false, &bytes.Buffer{}) != false {
		t.Fatal("JSON mode must disable color")
	}
}

func TestResolveColorNoColorFlag(t *testing.T) {
	if Resolve(false, true, &bytes.Buffer{}) != false {
		t.Fatal("--no-color must disable color")
	}
}

func TestResolveColorNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if Resolve(false, false, &bytes.Buffer{}) != false {
		t.Fatal("NO_COLOR env must disable color")
	}
}

func TestResolveColorNonTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	// FORCE_COLOR is checked before isatty; clear it so we exercise the
	// non-tty fallback path rather than the env override.
	t.Setenv("FORCE_COLOR", "")
	// io.Writer that's not a tty (a bytes.Buffer) => off
	if Resolve(false, false, &bytes.Buffer{}) != false {
		t.Fatal("non-tty writer must disable color")
	}
}

func TestResolveColorTTYActsOnFile(t *testing.T) {
	// We can't easily fake isatty on bytes.Buffer; this is a smoke test
	// to ensure Resolve doesn't panic when given an *os.File.
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	_ = Resolve(false, false, os.Stderr)
}
