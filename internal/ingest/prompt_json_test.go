package ingest

import (
	"strings"
	"testing"
	"time"

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

func TestPromptJSONParsesArray(t *testing.T) {
	input := `[
		{"transaction_id":"T1","txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":1000.00,"merchant_fee":15.00,"net_payout":985.00,"channel":"promptpay"},
		{"transaction_id":"T2","txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":2000.00,"merchant_fee":30.00,"net_payout":1970.00,"channel":"promptpay"},
		{"transaction_id":"T3","txn_date":"2026-04-20T10:00:00Z","settle_date":"2026-04-23T10:00:00Z","amount":3000.00,"merchant_fee":45.00,"net_payout":2955.00,"channel":"promptpay"}
	]`

	recs, err := ParsePromptJSON(strings.NewReader(input), "prompt.json")
	if err != nil {
		t.Fatalf("ParsePromptJSON returned error: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	wantIDs := []string{"T1", "T2", "T3"}
	for i, want := range wantIDs {
		if recs[i].TransactionID != want {
			t.Errorf("record[%d] TransactionID: got %q, want %q", i, recs[i].TransactionID, want)
		}
	}
}

func TestPromptJSONParsesRFC3339WithBangkokOffset(t *testing.T) {
	input := `[{"transaction_id":"T1","txn_date":"2026-04-24T10:00:00+07:00","settle_date":"2026-04-27T10:00:00+07:00","amount":1000.00,"merchant_fee":15.00,"net_payout":985.00,"channel":"promptpay"}]`

	recs, err := ParsePromptJSON(strings.NewReader(input), "prompt.json")
	if err != nil {
		t.Fatalf("ParsePromptJSON returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	// 10:00 +07:00 must map to the same instant as 03:00 UTC.
	wantInstant := time.Date(2026, 4, 24, 3, 0, 0, 0, time.UTC)
	if !recs[0].TransactionDate.Equal(wantInstant) {
		t.Errorf("TransactionDate: got %v, want instant equal to %v", recs[0].TransactionDate, wantInstant)
	}
}

func TestPromptJSONParsesRFC3339UTC(t *testing.T) {
	input := `[{"transaction_id":"T1","txn_date":"2026-04-24T03:00:00Z","settle_date":"2026-04-27T03:00:00Z","amount":1000.00,"merchant_fee":15.00,"net_payout":985.00,"channel":"promptpay"}]`

	recs, err := ParsePromptJSON(strings.NewReader(input), "prompt.json")
	if err != nil {
		t.Fatalf("ParsePromptJSON returned error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	// 03:00 UTC == 10:00 Asia/Bangkok same calendar day.
	gotInBangkok := recs[0].TransactionDate.In(domain.BangkokTZ())
	wantBangkok := time.Date(2026, 4, 24, 10, 0, 0, 0, domain.BangkokTZ())
	if !gotInBangkok.Equal(wantBangkok) {
		t.Errorf("TransactionDate (in Bangkok): got %v, want %v", gotInBangkok, wantBangkok)
	}
}

func TestPromptJSONRejectsEmptyObjectInArray(t *testing.T) {
	_, err := ParsePromptJSON(strings.NewReader(`[{}]`), "prompt.json")
	if err == nil {
		t.Fatal("expected error for empty object (missing required fields), got nil")
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
