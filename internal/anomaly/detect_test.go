package anomaly

import (
	"testing"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

func TestExpectedFeeThaiAcquirer(t *testing.T) {
	got, err := ExpectedFee(domain.AcquirerThai, 100000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2500 {
		t.Fatalf("expected 2500, got %d", got)
	}
}

func TestExpectedFeeGlobalPay(t *testing.T) {
	got, err := ExpectedFee(domain.AcquirerGlobal, 100000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 3000 {
		t.Fatalf("expected 3000, got %d", got)
	}
}

func TestExpectedFeePromptPayTiers(t *testing.T) {
	cases := []struct {
		name  string
		gross int64
		want  int64
	}{
		{"just_below_5k", 499999, 7500},
		{"at_5k", 500000, 9000},
		{"at_20k", 2000000, 36000},
		{"just_above_20k", 2000001, 44000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExpectedFee(domain.AcquirerPrompt, tc.gross)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("gross=%d: expected %d, got %d", tc.gross, tc.want, got)
			}
		})
	}
}

func TestDetectFlagsAnomaly(t *testing.T) {
	settlements := []domain.SettlementRecord{
		{
			TransactionID: "txn-normal",
			Acquirer:      domain.AcquirerThai,
			GrossMinor:    100000,
			FeeMinor:      2500, // matches expected
		},
		{
			TransactionID: "txn-bad",
			Acquirer:      domain.AcquirerThai,
			GrossMinor:    100000,
			FeeMinor:      5000, // 2x expected
		},
	}
	got := Detect(settlements)
	if len(got) != 1 {
		t.Fatalf("expected 1 anomaly, got %d", len(got))
	}
	a := got[0]
	if a.TransactionID != "txn-bad" {
		t.Fatalf("expected txn-bad, got %s", a.TransactionID)
	}
	if a.Severity != "critical" {
		t.Fatalf("expected severity=critical, got %s", a.Severity)
	}
	if a.ExpectedFee != "25.00" {
		t.Fatalf("expected ExpectedFee=25.00, got %s", a.ExpectedFee)
	}
	if a.ActualFee != "50.00" {
		t.Fatalf("expected ActualFee=50.00, got %s", a.ActualFee)
	}
}
