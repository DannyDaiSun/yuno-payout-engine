package ingest

import (
	"strings"
	"testing"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

func TestThaiCSVParsesValidRow(t *testing.T) {
	input := "txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n" +
		"TXN001,2026-04-20,2026-04-21,1000.00,25.00,975.00,credit_card\n"

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
	if r.Acquirer != domain.AcquirerThai {
		t.Errorf("Acquirer: got %q, want %q", r.Acquirer, domain.AcquirerThai)
	}
	if r.Currency != "THB" {
		t.Errorf("Currency: got %q, want %q", r.Currency, "THB")
	}
	if r.SourceFile != "thai.csv" {
		t.Errorf("SourceFile: got %q, want %q", r.SourceFile, "thai.csv")
	}
}

func TestGlobalCSVParsesValidRow(t *testing.T) {
	input := "reference_number,processed_on,payout_date,original_amount,processing_fee,settled_amount,type\n" +
		"REF002,20/04/2026,24/04/2026,1000.00,30.00,970.00,credit_card\n"

	recs, err := ParseGlobalCSV(strings.NewReader(input), "global.csv")
	if err != nil {
		t.Fatalf("ParseGlobalCSV returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	r := recs[0]
	if r.TransactionID != "REF002" {
		t.Errorf("TransactionID: got %q, want %q", r.TransactionID, "REF002")
	}
	if r.GrossMinor != 100000 {
		t.Errorf("GrossMinor: got %d, want %d", r.GrossMinor, 100000)
	}
	if r.FeeMinor != 3000 {
		t.Errorf("FeeMinor: got %d, want %d", r.FeeMinor, 3000)
	}
	if r.NetMinor != 97000 {
		t.Errorf("NetMinor: got %d, want %d", r.NetMinor, 97000)
	}
	if r.Acquirer != domain.AcquirerGlobal {
		t.Errorf("Acquirer: got %q, want %q", r.Acquirer, domain.AcquirerGlobal)
	}
	if r.SourceFile != "global.csv" {
		t.Errorf("SourceFile: got %q, want %q", r.SourceFile, "global.csv")
	}
}

func TestPromptJSONParsesValidRecord(t *testing.T) {
	input := `[{"transaction_id":"TXN003","txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":1000.00,"merchant_fee":15.00,"net_payout":985.00,"channel":"promptpay"}]`

	recs, err := ParsePromptJSON(strings.NewReader(input), "prompt.json")
	if err != nil {
		t.Fatalf("ParsePromptJSON returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	r := recs[0]
	if r.TransactionID != "TXN003" {
		t.Errorf("TransactionID: got %q, want %q", r.TransactionID, "TXN003")
	}
	if r.GrossMinor != 100000 {
		t.Errorf("GrossMinor: got %d, want %d", r.GrossMinor, 100000)
	}
	if r.FeeMinor != 1500 {
		t.Errorf("FeeMinor: got %d, want %d", r.FeeMinor, 1500)
	}
	if r.NetMinor != 98500 {
		t.Errorf("NetMinor: got %d, want %d", r.NetMinor, 98500)
	}
	if r.Acquirer != domain.AcquirerPrompt {
		t.Errorf("Acquirer: got %q, want %q", r.Acquirer, domain.AcquirerPrompt)
	}
	if r.SourceFile != "prompt.json" {
		t.Errorf("SourceFile: got %q, want %q", r.SourceFile, "prompt.json")
	}
}
