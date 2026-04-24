package ingest

import (
	"strings"
	"testing"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

func TestPromptJSONRejectsMalformedJSON(t *testing.T) {
	input := `{"broken`

	_, err := ParsePromptJSON(strings.NewReader(input), "prompt.json")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "json") && !strings.Contains(msg, "parse") && !strings.Contains(msg, "decode") {
		t.Errorf("expected error to mention JSON/parse/decode, got: %v", err)
	}
}

func TestPromptJSONRejectsNullRequiredField(t *testing.T) {
	input := `[{"transaction_id":null,"txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":1000.00,"merchant_fee":15.00,"net_payout":985.00,"channel":"promptpay"}]`

	_, err := ParsePromptJSON(strings.NewReader(input), "prompt.json")
	if err == nil {
		t.Fatal("expected error for null transaction_id, got nil")
	}
}

func TestPromptJSONHandlesEmptyArray(t *testing.T) {
	recs, err := ParsePromptJSON(strings.NewReader(`[]`), "prompt.json")
	if err != nil {
		t.Fatalf("expected no error for empty array, got: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 records, got %d", len(recs))
	}
}

func TestPromptJSONRejectsStringAmount(t *testing.T) {
	input := `[{"transaction_id":"T1","txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":"1000.00","merchant_fee":"15.00","net_payout":"985.00","channel":"promptpay"}]`
	_, err := ParsePromptJSON(strings.NewReader(input), "test")
	if err == nil {
		t.Errorf("expected error for string amount in JSON")
	}
}

// TestPromptJSONTieredFeeBoundary verifies the parser captures merchant_fee
// values exactly as supplied at the 5,000 THB tier boundary. Per PromptPay
// spec the fee is 1.5% under 5000 and 1.8% from 5000-20000, but the parser
// does NOT compute fees -- it must faithfully pass through the supplied
// merchant_fee values without rounding or substitution.
func TestPromptJSONTieredFeeBoundary(t *testing.T) {
	input := `[
		{"transaction_id":"T_BELOW","txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":4999.99,"merchant_fee":74.99,"net_payout":4925.00,"channel":"promptpay"},
		{"transaction_id":"T_AT","txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":5000.00,"merchant_fee":90.00,"net_payout":4910.00,"channel":"promptpay"}
	]`

	recs, err := ParsePromptJSON(strings.NewReader(input), "prompt.json")
	if err != nil {
		t.Fatalf("ParsePromptJSON returned error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}

	wantFee0, err := domain.ParseMinorUnits("74.99")
	if err != nil {
		t.Fatalf("ParseMinorUnits for 74.99: %v", err)
	}
	wantFee1, err := domain.ParseMinorUnits("90.00")
	if err != nil {
		t.Fatalf("ParseMinorUnits for 90.00: %v", err)
	}

	if recs[0].FeeMinor != wantFee0 {
		t.Errorf("record[0] FeeMinor: got %d, want %d (74.99 in minor units)", recs[0].FeeMinor, wantFee0)
	}
	if recs[1].FeeMinor != wantFee1 {
		t.Errorf("record[1] FeeMinor: got %d, want %d (90.00 in minor units)", recs[1].FeeMinor, wantFee1)
	}

	// Sanity: gross amounts straddle the 5000 boundary.
	wantGross0, _ := domain.ParseMinorUnits("4999.99")
	wantGross1, _ := domain.ParseMinorUnits("5000.00")
	if recs[0].GrossMinor != wantGross0 {
		t.Errorf("record[0] GrossMinor: got %d, want %d", recs[0].GrossMinor, wantGross0)
	}
	if recs[1].GrossMinor != wantGross1 {
		t.Errorf("record[1] GrossMinor: got %d, want %d", recs[1].GrossMinor, wantGross1)
	}
}
