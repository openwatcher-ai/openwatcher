package main

import (
	"reflect"
	"testing"
)

func TestServeAliasDropsLeadingSubcommand(t *testing.T) {
	input := []string{"openwatcher", "serve", "--listen", "127.0.0.1:8787"}
	got := normalizeArgsForServeAlias(input)
	want := []string{"openwatcher", "--listen", "127.0.0.1:8787"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeArgsForServeAlias() = %#v, want %#v", got, want)
	}
}

func TestServeAliasLeavesOtherArgsUntouched(t *testing.T) {
	input := []string{"openwatcher", "--listen", "127.0.0.1:8787"}
	got := normalizeArgsForServeAlias(input)
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("normalizeArgsForServeAlias() changed args: %#v", got)
	}
}
