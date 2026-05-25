package projectalias_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/projectalias"
)

func TestLoadJSONString(t *testing.T) {
	aliases, err := projectalias.Load(`{"agmon":["/Users/admin/code/agmon","/tmp/agmon-wt"]}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := aliases.Resolve("/tmp/agmon-wt"); got != "agmon" {
		t.Fatalf("Resolve=%q want agmon", got)
	}
}

func TestLoadJSONFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aliases.json")
	if err := os.WriteFile(path, []byte(`{"tokenmeter":["/repo/tm"]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	aliases, err := projectalias.Load(path)
	if err != nil {
		t.Fatalf("Load file: %v", err)
	}
	if got := aliases.Resolve("/repo/tm"); got != "tokenmeter" {
		t.Fatalf("Resolve=%q want tokenmeter", got)
	}
}

func TestLoadRejectsDamagedJSON(t *testing.T) {
	if _, err := projectalias.Load(`{"agmon":`); err == nil {
		t.Fatal("expected damaged JSON to fail")
	}
}

func TestResolveFallsBackToBaseName(t *testing.T) {
	aliases := projectalias.Aliases{{Project: "agmon", CWDs: []string{"/repo/agmon"}}}
	if got := aliases.Resolve("/repo/other"); got != "other" {
		t.Fatalf("Resolve fallback=%q want other", got)
	}
}

func TestResolveFirstProjectWins(t *testing.T) {
	aliases := projectalias.Aliases{
		{Project: "first", CWDs: []string{"/repo/shared"}},
		{Project: "second", CWDs: []string{"/repo/shared"}},
	}
	if got := aliases.Resolve("/repo/shared"); got != "first" {
		t.Fatalf("Resolve=%q want first", got)
	}
}

func TestResolveFirstWinsAcrossMultipleProjects(t *testing.T) {
	for i := 0; i < 100; i++ {
		aliases, err := projectalias.Load(`{"first":["/repo/shared"],"second":["/repo/shared"]}`)
		if err != nil {
			t.Fatalf("Load iteration %d: %v", i, err)
		}
		if got := aliases.Resolve("/repo/shared"); got != "first" {
			t.Fatalf("iteration %d Resolve=%q want first", i, got)
		}
	}
}
