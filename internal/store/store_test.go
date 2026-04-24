package store

import (
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

func TestStoreSavesAndRetrievesTransaction(t *testing.T) {
	s := New()
	txn := domain.Transaction{
		ID:                 "T1",
		Acquirer:           domain.AcquirerThai,
		AmountMinor:        10000,
		Currency:           "THB",
		TransactionDate:    time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC),
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
	}
	s.SaveTransaction(txn)

	got, ok := s.GetTransaction("T1")
	if !ok {
		t.Fatalf("expected transaction T1 to be found")
	}
	if got.ID != "T1" || got.AmountMinor != 10000 || got.Acquirer != domain.AcquirerThai {
		t.Fatalf("retrieved txn does not match: %+v", got)
	}
}

func TestStoreSavesAndRetrievesSettlement(t *testing.T) {
	s := New()
	rec := domain.SettlementRecord{
		TransactionID:  "T1",
		Acquirer:       domain.AcquirerThai,
		GrossMinor:     10000,
		FeeMinor:       100,
		NetMinor:       9900,
		Currency:       "THB",
		SettlementDate: time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
		PaymentMethod:  domain.MethodCreditCard,
		SourceFile:     "settlements_2026_04_24.csv",
	}
	s.SaveSettlement(rec)

	got, ok := s.FindSettlement("T1", domain.AcquirerThai)
	if !ok {
		t.Fatalf("expected settlement (T1, ThaiAcquirer) to be found")
	}
	if got.TransactionID != "T1" || got.Acquirer != domain.AcquirerThai || got.NetMinor != 9900 {
		t.Fatalf("retrieved settlement does not match: %+v", got)
	}
}
