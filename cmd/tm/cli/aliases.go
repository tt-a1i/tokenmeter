package cli

import (
	"context"
	"fmt"
	"io"
)

var aliasMap = map[string]struct {
	target string
	hint   string
}{
	"cost":   {target: "daily", hint: "tm cost is deprecated and will be removed in v2.0. Use `tm daily` instead."},
	"report": {target: "session", hint: "tm report is deprecated. Use `tm session` / `tm weekly` / `tm monthly` instead."},
	"status": {target: "blocks-active", hint: "tm status is deprecated. Use `tm blocks --active` instead."},
	"top":    {target: "blocks-active", hint: "tm top is deprecated. Use `tm blocks --active` instead."},
}

// IsDeprecatedAlias reports whether `name` is a legacy command alias and
// returns the underlying target name (one of "daily", "session", or
// "blocks-active") when it is.
func IsDeprecatedAlias(name string) (string, bool) {
	a, ok := aliasMap[name]
	return a.target, ok
}

// RunDeprecatedAlias prints a deprecation warning to stderr and invokes the
// new-style target command. The blocks-active target degrades to a header
// line because wiring a BlocksLoader through the AggregateLoader-based alias
// helper would expand the API surface; users should switch to `tm blocks
// --active` directly. The `rest` argument is reserved for future routes;
// today's targets ignore it.
func RunDeprecatedAlias(ctx context.Context, stdout, stderr io.Writer, name string, rest []string, loader AggregateLoader) error {
	a, ok := aliasMap[name]
	if !ok {
		return fmt.Errorf("not a deprecated alias: %s", name)
	}
	fmt.Fprintln(stderr, "warning:", a.hint)
	switch a.target {
	case "daily":
		return RunAggregate(ctx, stdout, AggregateArgs{Bucket: BucketDaily}, loader)
	case "session":
		return RunSession(ctx, stdout, SessionArgs{}, loader)
	case "blocks-active":
		fmt.Fprintln(stdout, "(use tm blocks --active for full active-block view)")
		return nil
	}
	return nil
}
