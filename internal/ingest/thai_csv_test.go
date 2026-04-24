package ingest

import (
	"strings"
	"testing"

	"github.com/dannydaisun/payout-engine/internal/domain"
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

func TestThaiCSVParsesMultipleRows(t *testing.T) {
	input := "txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n" +
		"TXN001,2026-04-20,2026-04-21,1000.00,25.00,975.00,credit_card\n" +
		"TXN002,2026-04-20,2026-04-21,2000.00,50.00,1950.00,credit_card\n" +
		"TXN003,2026-04-20,2026-04-21,3000.00,75.00,2925.00,debit_card\n"

	recs, err := ParseThaiCSV(strings.NewReader(input), "thai.csv")
	if err != nil {
		t.Fatalf("ParseThaiCSV returned error: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	wantIDs := []string{"TXN001", "TXN002", "TXN003"}
	for i, want := range wantIDs {
		if recs[i].TransactionID != want {
			t.Errorf("record[%d] TransactionID: got %q, want %q", i, recs[i].TransactionID, want)
		}
	}
}

func TestThaiCSVColumnsParsedByHeader(t *testing.T) {
	// Columns appear in a different order than the canonical order; mapping
	// is by header name, so values must still be parsed correctly.
	input := "payment_method,fee_amt,net_amt,gross_amt,settlement_date,transaction_date,txn_ref\n" +
		"credit_card,25.00,975.00,1000.00,2026-04-21,2026-04-20,TXN001\n"

	recs, err := ParseThaiCSV(strings.NewReader(input), "thai.csv")
	if err != nil {
		t.Fatalf("ParseThaiCSV returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	r := recs[0]
	if r.TransactionID != "TXN001" {
		t.Errorf("TransactionID: got %q, want %q", r.TransactionID, "TXN001")
	}
	if r.GrossMinor != 100000 {
		t.Errorf("GrossMinor: got %d, want %d", r.GrossMinor, 100000)
	}
	if r.FeeMinor != 2500 {
		t.Errorf("FeeMinor: got %d, want %d", r.FeeMinor, 2500)
	}
	if r.NetMinor != 97500 {
		t.Errorf("NetMinor: got %d, want %d", r.NetMinor, 97500)
	}
	if string(r.PaymentMethod) != "credit_card" {
		t.Errorf("PaymentMethod: got %q, want %q", r.PaymentMethod, "credit_card")
	}
}

func TestThaiCSVTolerateUTF8BOM(t *testing.T) {
	// UTF-8 BOM (0xEF,0xBB,0xBF) prefixed before the header row.
	input := "\uFEFFtxn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n" +
		"TXN001,2026-04-20,2026-04-21,1000.00,25.00,975.00,credit_card\n"

	recs, err := ParseThaiCSV(strings.NewReader(input), "thai.csv")
	if err != nil {
		t.Fatalf("ParseThaiCSV returned error on BOM-prefixed input: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].TransactionID != "TXN001" {
		t.Errorf("TransactionID: got %q, want %q", recs[0].TransactionID, "TXN001")
	}
}

func TestThaiCSVAttachesAcquirerAndSource(t *testing.T) {
	const sourceFile = "uploads/2026/04/thai-batch-001.csv"
	input := "txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n" +
		"TXN001,2026-04-20,2026-04-21,1000.00,25.00,975.00,credit_card\n"

	recs, err := ParseThaiCSV(strings.NewReader(input), sourceFile)
	if err != nil {
		t.Fatalf("ParseThaiCSV returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].Acquirer != domain.AcquirerThai {
		t.Errorf("Acquirer: got %q, want %q", recs[0].Acquirer, domain.AcquirerThai)
	}
	if recs[0].SourceFile != sourceFile {
		t.Errorf("SourceFile: got %q, want %q", recs[0].SourceFile, sourceFile)
	}
}

func TestThaiCSVDatesAreBangkokTZ(t *testing.T) {
	input := "txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n" +
		"TXN001,2026-04-20,2026-04-21,1000.00,25.00,975.00,credit_card\n"

	recs, err := ParseThaiCSV(strings.NewReader(input), "thai.csv")
	if err != nil {
		t.Fatalf("ParseThaiCSV returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	loc := recs[0].TransactionDate.Location().String()
	if loc != "Asia/Bangkok" && loc != "ICT" {
		t.Errorf("TransactionDate: got tz %q, want Asia/Bangkok or ICT", loc)
	}
	loc2 := recs[0].SettlementDate.Location().String()
	if loc2 != "Asia/Bangkok" && loc2 != "ICT" {
		t.Errorf("SettlementDate: got tz %q, want Asia/Bangkok or ICT", loc2)
	}
}
