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

// TestStoreReturnedSlicesDoNotMutateInternal verifies that mutating the slice
// returned by ListTransactions / ListSettlements does not affect subsequent
// calls. This proves the store returns defensive snapshots (new backing array
// per call), not direct references to internal state.
func TestStoreReturnedSlicesDoNotMutateInternal(t *testing.T) {
	s := New()
	bk := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	// Seed two transactions so we can verify by ID set, not by position
	// (map iteration order is non-deterministic).
	originalTxns := map[string]int64{
		"T1": 10000,
		"T2": 20000,
	}
	for id, amt := range originalTxns {
		s.SaveTransaction(domain.Transaction{
			ID:                 id,
			Acquirer:           domain.AcquirerThai,
			AmountMinor:        amt,
			Currency:           "THB",
			TransactionDate:    bk,
			PaymentMethod:      domain.MethodCreditCard,
			ExpectedSettleDate: bk.Add(24 * time.Hour),
		})
	}
	originalSettlements := map[string]int64{
		"T1": 9900,
		"T2": 19800,
	}
	for id, net := range originalSettlements {
		s.SaveSettlement(domain.SettlementRecord{
			TransactionID:  id,
			Acquirer:       domain.AcquirerThai,
			GrossMinor:     net + 100,
			FeeMinor:       100,
			NetMinor:       net,
			Currency:       "THB",
			SettlementDate: bk.Add(24 * time.Hour),
			PaymentMethod:  domain.MethodCreditCard,
		})
	}

	// Mutate the returned slice aggressively: overwrite element 0 and clear all.
	first := s.ListTransactions()
	if len(first) != 2 {
		t.Fatalf("expected 2 txns in first list, got %d", len(first))
	}
	first[0] = domain.Transaction{ID: "MUTATED", AmountMinor: -1}
	for i := range first {
		first[i] = domain.Transaction{}
	}

	// Second call must return original data unaffected.
	second := s.ListTransactions()
	if len(second) != 2 {
		t.Fatalf("expected 2 txns in second list, got %d", len(second))
	}
	for _, txn := range second {
		want, ok := originalTxns[txn.ID]
		if !ok {
			t.Fatalf("unexpected txn ID in second list: %q (mutation leaked into store)", txn.ID)
		}
		if txn.AmountMinor != want {
			t.Fatalf("txn %s: amount=%d, want %d (mutation leaked into store)", txn.ID, txn.AmountMinor, want)
		}
	}

	// Same drill for settlements.
	firstS := s.ListSettlements()
	if len(firstS) != 2 {
		t.Fatalf("expected 2 settlements in first list, got %d", len(firstS))
	}
	firstS[0] = domain.SettlementRecord{TransactionID: "MUTATED", NetMinor: -1}
	for i := range firstS {
		firstS[i] = domain.SettlementRecord{}
	}

	secondS := s.ListSettlements()
	if len(secondS) != 2 {
		t.Fatalf("expected 2 settlements in second list, got %d", len(secondS))
	}
	for _, r := range secondS {
		want, ok := originalSettlements[r.TransactionID]
		if !ok {
			t.Fatalf("unexpected settlement TxnID in second list: %q (mutation leaked into store)", r.TransactionID)
		}
		if r.NetMinor != want {
			t.Fatalf("settlement %s: net=%d, want %d (mutation leaked into store)", r.TransactionID, r.NetMinor, want)
		}
	}
}
