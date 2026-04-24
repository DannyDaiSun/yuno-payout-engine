package ingest

import (
	"strings"
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

func TestGlobalCSVMapsReferenceNumberToTransactionID(t *testing.T) {
	input := "reference_number,processed_on,payout_date,original_amount,processing_fee,settled_amount,type\n" +
		"REF42,20/04/2026,24/04/2026,1000.00,30.00,970.00,credit_card\n"

	recs, err := ParseGlobalCSV(strings.NewReader(input), "global.csv")
	if err != nil {
		t.Fatalf("ParseGlobalCSV returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].TransactionID != "REF42" {
		t.Errorf("TransactionID: got %q, want %q", recs[0].TransactionID, "REF42")
	}
}

func TestGlobalCSVParsesDDMMYYYY(t *testing.T) {
	input := "reference_number,processed_on,payout_date,original_amount,processing_fee,settled_amount,type\n" +
		"REF99,20/04/2026,24/04/2026,1000.00,30.00,970.00,credit_card\n"

	recs, err := ParseGlobalCSV(strings.NewReader(input), "global.csv")
	if err != nil {
		t.Fatalf("ParseGlobalCSV returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	want := time.Date(2026, 4, 24, 0, 0, 0, 0, domain.BangkokTZ())
	if !recs[0].SettlementDate.Equal(want) {
		t.Errorf("SettlementDate: got %v, want %v", recs[0].SettlementDate, want)
	}
	wantTxn := time.Date(2026, 4, 20, 0, 0, 0, 0, domain.BangkokTZ())
	if !recs[0].TransactionDate.Equal(wantTxn) {
		t.Errorf("TransactionDate: got %v, want %v", recs[0].TransactionDate, wantTxn)
	}
}

func TestGlobalCSVDoesNotPanicOnShortRow(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked on short row: %v", r)
		}
	}()
	// Header has 7 cols; data row has only 3.
	input := "reference_number,processed_on,payout_date,original_amount,processing_fee,settled_amount,type\n" +
		"REF001,20/04/2026,24/04/2026\n"

	_, err := ParseGlobalCSV(strings.NewReader(input), "global.csv")
	if err == nil {
		t.Fatal("expected error for short row, got nil")
	}
	if !strings.Contains(err.Error(), "short row") {
		t.Errorf("expected error to mention short row, got: %v", err)
	}
}

func TestGlobalCSVRejectsMissingColumn(t *testing.T) {
	// Header is missing the processing_fee column.
	input := "reference_number,processed_on,payout_date,original_amount,settled_amount,type\n" +
		"REF002,20/04/2026,24/04/2026,1000.00,970.00,credit_card\n"

	_, err := ParseGlobalCSV(strings.NewReader(input), "global.csv")
	if err == nil {
		t.Fatal("expected error for missing processing_fee column, got nil")
	}
	if !strings.Contains(err.Error(), "processing_fee") {
		t.Errorf("expected error to mention processing_fee, got: %v", err)
	}
}
