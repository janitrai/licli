package cmd

import (
	"testing"
)

func TestRootCmd_HasExpectedSubcommands(t *testing.T) {
	expected := map[string]bool{
		"auth":    false,
		"post":    false,
		"profile": false,
		"search":  false,
		"connect": false,
		"follow":  false,
	}

	for _, sub := range rootCmd.Commands() {
		name := sub.Name()
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("rootCmd missing expected subcommand %q", name)
		}
	}
}

func TestRootCmd_PersistentFlags(t *testing.T) {
	flags := []string{"config", "debug"}
	for _, name := range flags {
		f := rootCmd.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("rootCmd missing persistent flag %q", name)
		}
	}
}

func TestRootCmd_UseAndShort(t *testing.T) {
	if rootCmd.Use != "li" {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, "li")
	}
	if rootCmd.Short == "" {
		t.Error("rootCmd.Short is empty")
	}
}
