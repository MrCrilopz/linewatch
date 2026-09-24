package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("LINEWATCH_TEST_KEY=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LINEWATCH_TEST_KEY", "")
	if err := loadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("LINEWATCH_TEST_KEY") != "from-file" {
		t.Fatalf("got %q", os.Getenv("LINEWATCH_TEST_KEY"))
	}
	t.Setenv("LINEWATCH_TEST_KEY", "from-shell")
	if err := loadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("LINEWATCH_TEST_KEY") != "from-shell" {
		t.Fatalf("overwrote %q", os.Getenv("LINEWATCH_TEST_KEY"))
	}
}
