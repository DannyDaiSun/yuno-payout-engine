package ingest

import (
	"strings"
	"testing"
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
