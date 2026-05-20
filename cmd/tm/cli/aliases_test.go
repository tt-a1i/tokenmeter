package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
)

func TestRunDeprecatedAliasPrintsWarningAndCallsTarget(t *testing.T) {
	var out, errOut bytes.Buffer
	loader := stubAggregateLoader{}
	err := cli.RunDeprecatedAlias(context.Background(), &out, &errOut, "cost", []string{}, loader)
	if err != nil {
		t.Fatalf("RunDeprecatedAlias: %v", err)
	}
	if !strings.Contains(errOut.String(), "deprecated") {
		t.Fatalf("stderr must mention deprecation: %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "tm daily") {
		t.Fatalf("stderr must guide user to new command: %q", errOut.String())
	}
	if out.Len() == 0 {
		t.Fatal("alias must still produce normal output")
	}
}
