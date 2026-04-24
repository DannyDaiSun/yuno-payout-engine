package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerationIsDeterministicWithSeed ensures two runs with the same seed
// produce byte-identical output across all four fixture files.
func TestGenerationIsDeterministicWithSeed(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	if err := Generate(42, dirA); err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	if err := Generate(42, dirB); err != nil {
		t.Fatalf("second Generate: %v", err)
	}

	files := []string{
		"transactions.csv",
		filepath.Join("settlements", "thai_acquirer.csv"),
		filepath.Join("settlements", "global_pay.csv"),
		filepath.Join("settlements", "promptpay.json"),
	}

	for _, f := range files {
		a, err := os.ReadFile(filepath.Join(dirA, f))
		if err != nil {
			t.Fatalf("read %s from dirA: %v", f, err)
		}
		b, err := os.ReadFile(filepath.Join(dirB, f))
		if err != nil {
			t.Fatalf("read %s from dirB: %v", f, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("file %s differs between runs (lenA=%d lenB=%d)", f, len(a), len(b))
		}
		if len(a) == 0 {
			t.Errorf("file %s is empty", f)
		}
	}
}
