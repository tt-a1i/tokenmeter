package projectalias

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type AliasEntry struct {
	Project string
	CWDs    []string
}

type Aliases []AliasEntry

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
	aliases, err := parseAliases(data)
	if err != nil {
		return nil, err
	}
	return aliases, nil
}

func (a Aliases) Resolve(cwd string) string {
	for _, entry := range a {
		for _, candidate := range entry.CWDs {
			if samePath(candidate, cwd) {
				return entry.Project
			}
		}
	}
	base := filepath.Base(filepath.Clean(cwd))
	if base == "." || base == string(filepath.Separator) {
		return cwd
	}
	return base
}

func parseAliases(data []byte) (Aliases, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("project aliases must be a JSON object")
	}
	var aliases Aliases
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		project, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("project alias key must be a string")
		}
		var cwds []string
		if err := dec.Decode(&cwds); err != nil {
			return nil, err
		}
		aliases = append(aliases, AliasEntry{Project: project, CWDs: cwds})
	}
	tok, err = dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok = tok.(json.Delim)
	if !ok || delim != '}' {
		return nil, fmt.Errorf("project aliases object not closed")
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("project aliases JSON has trailing content")
		}
		return nil, err
	}
	return aliases, nil
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
