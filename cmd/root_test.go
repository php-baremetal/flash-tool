package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommandName(t *testing.T) {
	root := NewRootCmd()
	if root.Use != "phpflash" {
		t.Fatalf("root Use = %q, want %q", root.Use, "phpflash")
	}
	if root.Version != Version {
		t.Errorf("root Version = %q, want %q", root.Version, Version)
	}
}

func TestRootHelpListsSubcommands(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"init", "system-setup"} {
		if !bytes.Contains(out.Bytes(), []byte(sub)) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}
