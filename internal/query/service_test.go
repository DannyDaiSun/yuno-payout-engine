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
