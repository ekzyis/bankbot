package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# a comment\n\nCCBANK_FROM_FILE=hello\nexport CCBANK_QUOTED=\"tk_x\"\nCCBANK_PRESET=fromfile\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// A value already in the environment must win over the file.
	t.Setenv("CCBANK_PRESET", "preset")
	os.Unsetenv("CCBANK_FROM_FILE")
	os.Unsetenv("CCBANK_QUOTED")
	t.Cleanup(func() {
		os.Unsetenv("CCBANK_FROM_FILE")
		os.Unsetenv("CCBANK_QUOTED")
	})

	if err := loadEnvFile(path); err != nil {
		t.Fatalf("loadEnvFile: %v", err)
	}
	if got := os.Getenv("CCBANK_FROM_FILE"); got != "hello" {
		t.Errorf("CCBANK_FROM_FILE = %q, want hello", got)
	}
	if got := os.Getenv("CCBANK_QUOTED"); got != "tk_x" {
		t.Errorf("CCBANK_QUOTED = %q, want tk_x (quotes stripped)", got)
	}
	if got := os.Getenv("CCBANK_PRESET"); got != "preset" {
		t.Errorf("CCBANK_PRESET = %q, want preset (env wins over file)", got)
	}
}

func TestLoadEnvFile_MissingIsOK(t *testing.T) {
	if err := loadEnvFile(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Errorf("missing file should not error, got %v", err)
	}
}

func TestLoadEnvFile_Malformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("NO_EQUALS_SIGN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err == nil {
		t.Error("expected error for a line without '='")
	}
}
