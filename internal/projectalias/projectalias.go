package projectalias

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Aliases map[string][]string

func Load(jsonOrPath string) (Aliases, error) {
	raw := strings.TrimSpace(jsonOrPath)
	if raw == "" {
		return nil, nil
	}
	var data []byte
	if strings.HasPrefix(raw, "{") {
		data = []byte(raw)
	} else {
		expanded := raw
		if strings.HasPrefix(raw, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				expanded = filepath.Join(home, raw[2:])
			}
		}
		b, err := os.ReadFile(expanded)
		if err != nil {
			return nil, err
		}
		data = b
	}
	var aliases Aliases
	if err := json.Unmarshal(data, &aliases); err != nil {
		return nil, err
	}
	return aliases, nil
}

func (a Aliases) Resolve(cwd string) string {
	for project, cwds := range a {
		for _, candidate := range cwds {
			if samePath(candidate, cwd) {
				return project
			}
		}
	}
	base := filepath.Base(filepath.Clean(cwd))
	if base == "." || base == string(filepath.Separator) {
		return cwd
	}
	return base
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
