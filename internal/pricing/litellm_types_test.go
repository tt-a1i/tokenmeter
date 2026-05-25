package pricing

import (
	"encoding/json"
	"testing"
)

func TestStringOrIntUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "int", input: `12345`, want: 12345},
		{name: "integer float", input: `2000000.0`, want: 2000000},
		{name: "string", input: `"12345"`, want: 12345},
		{name: "null", input: `null`, want: 0},
		{name: "empty string", input: `""`, want: 0},
		{name: "bad string", input: `"abc"`, wantErr: true},
		{name: "float string", input: `"12.5"`, wantErr: true},
		{name: "fractional number", input: `12.5`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got stringOrInt
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalJSON: %v", err)
			}
			if int64(got) != tt.want {
				t.Fatalf("value = %d, want %d", int64(got), tt.want)
			}
		})
	}
}

func TestStringOrFloatUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "float", input: `0.00000125`, want: 0.00000125},
		{name: "string", input: `"0.00000125"`, want: 0.00000125},
		{name: "null", input: `null`, want: 0},
		{name: "empty string", input: `""`, want: 0},
		{name: "bad string", input: `"abc"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got stringOrFloat
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalJSON: %v", err)
			}
			if float64(got) != tt.want {
				t.Fatalf("value = %g, want %g", float64(got), tt.want)
			}
		})
	}
}
