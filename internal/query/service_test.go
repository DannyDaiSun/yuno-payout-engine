package query

import (
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/store"
)

func newStoreWithFixture(t *testing.T) *store.Store {
	t.Helper()
	s := store.New()
	bk := domain.BangkokTZ()
	day := time.Date(2026, 4, 24, 0, 0, 0, 0, bk)
	s.SaveTransaction(domain.Transaction{
		ID:                 "T1",
		Acquirer:           domain.AcquirerThai,
		AmountMinor:        100000,
		Currency:           "THB",
		TransactionDate:    time.Date(2026, 4, 23, 0, 0, 0, 0, bk),
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: day,
	})
	s.SaveSettlement(domain.SettlementRecord{
		TransactionID:   "T1",
		Acquirer:        domain.AcquirerThai,
		GrossMinor:      100000,
		FeeMinor:        2500,
		NetMinor:        97500,
		Currency:        "THB",
		TransactionDate: time.Date(2026, 4, 23, 0, 0, 0, 0, bk),
		SettlementDate:  day,
		PaymentMethod:   domain.MethodCreditCard,
	})
	return s
}

func TestExpectedCashByAcquirerSumsNet(t *testing.T) {
	q := New(newStoreWithFixture(t))
	res := q.ExpectedCashByAcquirer(time.Date(2026, 4, 24, 0, 0, 0, 0, domain.BangkokTZ()))
	if res.Total != "975.00" {
		t.Errorf("got total %q, want 975.00", res.Total)
	}
}

func TestUnsettledRejectsNegativeDays(t *testing.T) {
	q := New(store.New())
	_, err := q.UnsettledSince(-1, time.Now())
	if err != ErrInvalidDays {
		t.Errorf("got %v, want ErrInvalidDays", err)
	}
}

func TestFeesRejectsInvalidMonthFormat(t *testing.T) {
	q := New(store.New())
	_, err := q.FeesByAcquirer("2026/04")
	if err == nil {
		t.Errorf("expected error for invalid month format")
	}
}

func TestFeesByAcquirerForMonth(t *testing.T) {
	q := New(newStoreWithFixture(t))
	res, err := q.FeesByAcquirer("2026-04")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != "25.00" {
		t.Errorf("got total %q, want 25.00", res.Total)
	}
}

// TestFeesUsesBangkokMonthBoundaries verifies month aggregation uses Bangkok
// timezone, not UTC. Two records straddle the April->May boundary in Bangkok:
// Record A is April 30 23:30 BKK (= April 30 16:30 UTC) -- counts as April.
// Record B is May 1 00:30 BKK (= April 30 17:30 UTC) -- counts as May.
// If the query bucketed by UTC date, both would land in April and the test fails.
func TestFeesUsesBangkokMonthBoundaries(t *testing.T) {
	s := store.New()
	bk := domain.BangkokTZ()

	// Record A: April 30 23:30 Bangkok (the supplied UTC instant equals 16:30Z).
	settleA := time.Date(2026, 4, 30, 23, 30, 0, 0, bk)
	s.SaveSettlement(domain.SettlementRecord{
		TransactionID:   "T_APR",
		Acquirer:        domain.AcquirerThai,
		GrossMinor:      100000,
		FeeMinor:        1500,
		NetMinor:        98500,
		Currency:        "THB",
		TransactionDate: settleA,
		SettlementDate:  settleA,
		PaymentMethod:   domain.MethodCreditCard,
	})
	// Record B: May 1 00:30 Bangkok (= April 30 17:30 UTC).
	settleB := time.Date(2026, 5, 1, 0, 30, 0, 0, bk)
	s.SaveSettlement(domain.SettlementRecord{
		TransactionID:   "T_MAY",
		Acquirer:        domain.AcquirerThai,
		GrossMinor:      100000,
		FeeMinor:        9999,
		NetMinor:        90001,
		Currency:        "THB",
		TransactionDate: settleB,
		SettlementDate:  settleB,
		PaymentMethod:   domain.MethodCreditCard,
	})

	q := New(s)
	res, err := q.FeesByAcquirer("2026-04")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only Record A's fee (15.00) should be in April; Record B (99.99) is May.
	if res.Total != "15.00" {
		t.Errorf("April total: got %q, want 15.00 (only record A; record B is in May Bangkok)", res.Total)
	}
}
