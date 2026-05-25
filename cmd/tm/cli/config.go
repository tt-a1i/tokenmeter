package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	tmconfig "github.com/tt-a1i/tokenmeter/internal/config"
)

func RunConfig(ctx context.Context, out io.Writer, subcommand string, configPath string) error {
	_ = ctx
	switch subcommand {
	case "show":
		cfg, err := tmconfig.LoadWithOptions(tmconfig.Options{ExplicitPath: configPath})
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	case "path":
		_, err := fmt.Fprintln(out, tmconfig.EffectivePath(configPath))
		return err
	case "init":
		path := tmconfig.GlobalPath()
		if configPath != "" {
			path = configPath
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := tmconfig.Save(tmconfig.Example(), path); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "created %s\n", path)
		return err
	default:
		return fmt.Errorf("unknown config command: %s", subcommand)
	}
}
