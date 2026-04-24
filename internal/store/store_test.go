package store

import (
	"fmt"
	"sync"
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

func TestStoreDuplicateTransactionIsIdempotent(t *testing.T) {
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
	// Save again with the same ID but mutated amount — should overwrite,
	// not append, and ListTransactions must still return exactly one record.
	txn2 := txn
	txn2.AmountMinor = 20000
	s.SaveTransaction(txn2)

	all := s.ListTransactions()
	if len(all) != 1 {
		t.Fatalf("expected 1 transaction after duplicate save, got %d", len(all))
	}
	if all[0].AmountMinor != 20000 {
		t.Fatalf("expected duplicate save to overwrite amount to 20000, got %d", all[0].AmountMinor)
	}
}

func TestStoreDuplicateSettlementIsIdempotent(t *testing.T) {
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
	}
	s.SaveSettlement(rec)
	// Save again with same (TransactionID, Acquirer) — should overwrite.
	rec2 := rec
	rec2.NetMinor = 8800
	s.SaveSettlement(rec2)

	all := s.ListSettlements()
	if len(all) != 1 {
		t.Fatalf("expected 1 settlement after duplicate save, got %d", len(all))
	}
	if all[0].NetMinor != 8800 {
		t.Fatalf("expected duplicate save to overwrite NetMinor to 8800, got %d", all[0].NetMinor)
	}
}

func TestStoreConcurrentSaveAndRead(t *testing.T) {
	s := New()
	base := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	// 50 writer goroutines, each saving a unique transaction.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.SaveTransaction(domain.Transaction{
				ID:                 fmt.Sprintf("T%d", i),
				Acquirer:           domain.AcquirerThai,
				AmountMinor:        int64(1000 + i),
				Currency:           "THB",
				TransactionDate:    base,
				PaymentMethod:      domain.MethodCreditCard,
				ExpectedSettleDate: base.Add(24 * time.Hour),
			})
		}(i)
	}
	// 50 reader goroutines, each calling ListTransactions concurrently with writes.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.ListTransactions()
		}()
	}
	wg.Wait()

	// After all writers finish there must be exactly 50 unique transactions.
	all := s.ListTransactions()
	if len(all) != 50 {
		t.Fatalf("expected 50 transactions after concurrent saves, got %d", len(all))
	}
}

func TestStoreFindSettlementReturnsFalseWhenMissing(t *testing.T) {
	s := New()
	got, ok := s.FindSettlement("nonexistent", domain.AcquirerThai)
	if ok {
		t.Fatalf("expected ok=false for missing settlement, got ok=true")
	}
	if got != (domain.SettlementRecord{}) {
		t.Fatalf("expected zero-value SettlementRecord for missing settlement, got %+v", got)
	}
}
