package main

import (
	"reflect"
	"testing"
)

func TestMainConfigFlagBeforeSubcommand(t *testing.T) {
	got := normalizeTopLevelArgs([]string{"--config", "/tmp/tokenmeter.json", "daily", "--help"})
	want := []string{"daily", "--config", "/tmp/tokenmeter.json", "--help"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestMainConfigFlagAfterSubcommand(t *testing.T) {
	got := normalizeTopLevelArgs([]string{"daily", "--config", "/tmp/tokenmeter.json", "--help"})
	want := []string{"daily", "--config", "/tmp/tokenmeter.json", "--help"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestMainConfigFlagEquals(t *testing.T) {
	got := normalizeTopLevelArgs([]string{"--config=/tmp/tokenmeter.json", "daily", "--help"})
	want := []string{"daily", "--config=/tmp/tokenmeter.json", "--help"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestMainNoColorFlagBeforeSubcommand(t *testing.T) {
	got := normalizeTopLevelArgs([]string{"--no-color", "daily"})
	want := []string{"daily", "--no-color"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestMainCCUsageGlobalFlagsBeforeSubcommand(t *testing.T) {
	got := normalizeTopLevelArgs([]string{"--json", "-s", "20260101", "--no-offline", "-d", "daily"})
	want := []string{"daily", "--json", "-s", "20260101", "--no-offline", "-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestMainLegacyColonCommandNormalize(t *testing.T) {
	got := normalizeTopLevelArgs([]string{"--json", "codex:daily"})
	want := []string{"codex", "--json", "daily"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestMainGlobalFlagsOnlyDefaultsToDaily(t *testing.T) {
	got := normalizeTopLevelArgs([]string{"--json", "--no-offline"})
	want := []string{"daily", "--json", "--no-offline"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestMainIDGlobalFlagsOnlyDefaultsToSession(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "long",
			args: []string{"--id", "abc", "--json"},
			want: []string{"session", "--id", "abc", "--json"},
		},
		{
			name: "short",
			args: []string{"-i", "abc", "--json"},
			want: []string{"session", "-i", "abc", "--json"},
		},
		{
			name: "equals",
			args: []string{"--id=abc", "--json"},
			want: []string{"session", "--id=abc", "--json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTopLevelArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalized args = %#v, want %#v", got, tt.want)
			}
		})
	}
}
