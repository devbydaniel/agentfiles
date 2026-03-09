package cmd

import (
	"bytes"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "agentfiles version " + Version + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestVersionVar(t *testing.T) {
	old := Version
	defer func() { Version = old }()

	Version = "test-1.2.3"
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "agentfiles version test-1.2.3\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStoreDefaultFlag(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("store")
	if f == nil {
		t.Fatal("--store flag not registered")
	}
	// Default is empty; resolved at runtime via config or fallback.
	if f.DefValue != "" {
		t.Errorf("default store = %q, want %q", f.DefValue, "")
	}
}

func TestConfigFlag(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("config")
	if f == nil {
		t.Fatal("--config flag not registered")
	}
	if f.DefValue != "" {
		t.Errorf("default config = %q, want %q", f.DefValue, "")
	}
}

func TestUnknownCommandError(t *testing.T) {
	rootCmd.SetArgs([]string{"nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}
