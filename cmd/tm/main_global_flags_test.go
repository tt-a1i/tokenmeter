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
