package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
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

// TestGeneratesExactly300Transactions verifies that transactions.csv contains
// exactly 300 data rows (excluding the header).
func TestGeneratesExactly300Transactions(t *testing.T) {
	dir := t.TempDir()
	if err := Generate(42, dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	f, err := os.Open(filepath.Join(dir, "transactions.csv"))
	if err != nil {
		t.Fatalf("open transactions.csv: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read transactions.csv: %v", err)
	}
	// rows[0] is the header.
	got := len(rows) - 1
	if got != 300 {
		t.Errorf("transactions.csv: got %d data rows, want 300", got)
	}
}

// TestEachSettlementFileHas70Entries verifies that each of the three
// settlement files contains exactly 70 entries.
func TestEachSettlementFileHas70Entries(t *testing.T) {
	dir := t.TempDir()
	if err := Generate(42, dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	csvFiles := []string{
		filepath.Join("settlements", "thai_acquirer.csv"),
		filepath.Join("settlements", "global_pay.csv"),
	}
	for _, name := range csvFiles {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		rows, err := csv.NewReader(f).ReadAll()
		f.Close()
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		got := len(rows) - 1 // exclude header
		if got != 70 {
			t.Errorf("%s: got %d entries, want 70", name, got)
		}
	}

	// promptpay.json is a JSON array.
	buf, err := os.ReadFile(filepath.Join(dir, "settlements", "promptpay.json"))
	if err != nil {
		t.Fatalf("read promptpay.json: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(buf, &entries); err != nil {
		t.Fatalf("unmarshal promptpay.json: %v", err)
	}
	if len(entries) != 70 {
		t.Errorf("promptpay.json: got %d entries, want 70", len(entries))
	}
}

// TestPromptPayJSONEmitsNumericAmount verifies that the amount, merchant_fee,
// and net_payout fields in promptpay.json are emitted as JSON numbers
// (which decode to float64), not as quoted strings.
func TestPromptPayJSONEmitsNumericAmount(t *testing.T) {
	dir := t.TempDir()
	if err := Generate(42, dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	buf, err := os.ReadFile(filepath.Join(dir, "settlements", "promptpay.json"))
	if err != nil {
		t.Fatalf("read promptpay.json: %v", err)
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal(buf, &entries); err != nil {
		t.Fatalf("unmarshal promptpay.json: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("promptpay.json contained no entries")
	}

	numericFields := []string{"amount", "merchant_fee", "net_payout"}
	for i, e := range entries {
		for _, field := range numericFields {
			v, ok := e[field]
			if !ok {
				t.Fatalf("entry %d missing field %q", i, field)
			}
			if _, isFloat := v.(float64); !isFloat {
				t.Errorf("entry %d field %q: got %T (%v), want float64 (numeric, unquoted)", i, field, v, v)
			}
		}
	}
}
