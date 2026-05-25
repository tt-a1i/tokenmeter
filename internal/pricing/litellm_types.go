package pricing

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type stringOrInt int64

func (s *stringOrInt) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return nil
	}
	if raw[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		if str == "" {
			return nil
		}
		n, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("stringOrInt: parse %q: %w", str, err)
		}
		*s = stringOrInt(n)
		return nil
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return err
	}
	if math.Trunc(f) != f {
		return fmt.Errorf("stringOrInt: non-integer number %q", raw)
	}
	*s = stringOrInt(int64(f))
	return nil
}

type stringOrFloat float64

func (s *stringOrFloat) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return nil
	}
	if raw[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		if str == "" {
			return nil
		}
		n, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return fmt.Errorf("stringOrFloat: parse %q: %w", str, err)
		}
		*s = stringOrFloat(n)
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*s = stringOrFloat(n)
	return nil
}
