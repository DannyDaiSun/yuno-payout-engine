package ingest

import (
	"strings"
	"testing"
)

func TestThaiCSVRejectsMissingColumn(t *testing.T) {
	// Header is missing the fee_amt column.
	input := "txn_ref,transaction_date,settlement_date,gross_amt,net_amt,payment_method\n" +
		"TXN001,2026-04-20,2026-04-21,1000.00,975.00,credit_card\n"

	_, err := ParseThaiCSV(strings.NewReader(input), "thai.csv")
	if err == nil {
		t.Fatal("expected error for missing fee_amt column, got nil")
	}
	if !strings.Contains(err.Error(), "fee_amt") {
		t.Errorf("expected error to mention fee_amt, got: %v", err)
	}
}

func TestThaiCSVRejectsEmptyFile(t *testing.T) {
	_, err := ParseThaiCSV(strings.NewReader(""), "thai.csv")
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestThaiCSVHeaderOnlyReturnsZeroRecords(t *testing.T) {
	input := "txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n"

	recs, err := ParseThaiCSV(strings.NewReader(input), "thai.csv")
	if err != nil {
		t.Fatalf("expected no error for header-only CSV, got: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 records, got %d", len(recs))
	}
}
