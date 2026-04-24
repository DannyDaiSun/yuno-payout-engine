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
func TestSettledSinceReturnsMatchedTxns(t *testing.T) {
	s := store.New()
	bk := domain.BangkokTZ()
	asOf := time.Date(2026, 4, 24, 12, 0, 0, 0, bk)
	day := time.Date(2026, 4, 24, 0, 0, 0, 0, bk)

	// Txn 1: settled (matching settlement)
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
	// Txn 2: no matching settlement
	s.SaveTransaction(domain.Transaction{
		ID:                 "T2",
		Acquirer:           domain.AcquirerThai,
		AmountMinor:        50000,
		Currency:           "THB",
		TransactionDate:    time.Date(2026, 4, 23, 0, 0, 0, 0, bk),
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: day,
	})

	q := New(s)
	res, err := q.SettledSince(7, asOf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != 1 {
		t.Errorf("got total %d, want 1", res.Total)
	}
	if len(res.SettledTransactions) != 1 || res.SettledTransactions[0].ID != "T1" {
		t.Errorf("expected T1 in settled list, got %+v", res.SettledTransactions)
	}
	if res.SettledTransactions[0].NetAmount != "975.00" {
		t.Errorf("got net %q, want 975.00", res.SettledTransactions[0].NetAmount)
	}
}

// TestExpectedCashEmptyReturnsEmptyGroups verifies that with an empty store,
// ExpectedCashByAcquirer still returns a fully-populated ByAcquirer slice
// (one entry per acquirer constant) with each NetAmount = "0.00" and Total
// = "0.00". This guarantees the JSON shape never has a null slice and the
// caller can rely on the three-acquirer breakdown always being present.
func TestExpectedCashEmptyReturnsEmptyGroups(t *testing.T) {
	q := New(store.New())
	res := q.ExpectedCashByAcquirer(time.Date(2026, 4, 24, 0, 0, 0, 0, domain.BangkokTZ()))

	if res.ByAcquirer == nil {
		t.Fatalf("ByAcquirer must not be nil (would marshal to null)")
	}
	if len(res.ByAcquirer) != 3 {
		t.Fatalf("expected 3 acquirer entries, got %d", len(res.ByAcquirer))
	}
	wantAcquirers := map[domain.Acquirer]bool{
		domain.AcquirerThai:   false,
		domain.AcquirerGlobal: false,
		domain.AcquirerPrompt: false,
	}
	for _, item := range res.ByAcquirer {
		seen, known := wantAcquirers[item.Acquirer]
		if !known {
			t.Fatalf("unexpected acquirer in result: %s", item.Acquirer)
		}
		if seen {
			t.Fatalf("duplicate acquirer in result: %s", item.Acquirer)
		}
		wantAcquirers[item.Acquirer] = true
		if item.NetAmount != "0.00" {
			t.Fatalf("acquirer %s: NetAmount=%q, want \"0.00\"", item.Acquirer, item.NetAmount)
		}
	}
	for acq, seen := range wantAcquirers {
		if !seen {
			t.Fatalf("missing acquirer in result: %s", acq)
		}
	}
	if res.Total != "0.00" {
		t.Fatalf("Total: got %q, want \"0.00\"", res.Total)
	}
}

// TestUnsettledSinceFiltersByDays verifies the days-window filter excludes
// transactions whose TransactionDate is older than asOf - days, while
// including those within the window.
func TestUnsettledSinceFiltersByDays(t *testing.T) {
	bk := domain.BangkokTZ()
	asOf := time.Date(2026, 4, 23, 12, 0, 0, 0, bk)
	asOfDay := domain.BangkokMidnight(asOf)

	s := store.New()
	mkTxn := func(id string, daysAgo int) {
		txnDate := asOfDay.AddDate(0, 0, -daysAgo)
		s.SaveTransaction(domain.Transaction{
			ID:                 id,
			Acquirer:           domain.AcquirerThai,
			AmountMinor:        10000,
			Currency:           "THB",
			TransactionDate:    txnDate,
			PaymentMethod:      domain.MethodCreditCard,
			// Past expected settle date so they are overdue (unsettled, not pending).
			ExpectedSettleDate: txnDate.AddDate(0, 0, 1),
		})
	}
	mkTxn("T_RECENT", 1)  // within 7-day window
	mkTxn("T_MID", 5)     // within 7-day window
	mkTxn("T_OLD", 30)    // outside 7-day window

	q := New(s)
	res, err := q.UnsettledSince(7, asOf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := make(map[string]bool, len(res.UnsettledTransactions))
	for _, u := range res.UnsettledTransactions {
		got[u.ID] = true
	}
	if !got["T_RECENT"] {
		t.Fatalf("expected T_RECENT (1 day old) in window, got %+v", res.UnsettledTransactions)
	}
	if !got["T_MID"] {
		t.Fatalf("expected T_MID (5 days old) in window, got %+v", res.UnsettledTransactions)
	}
	if got["T_OLD"] {
		t.Fatalf("expected T_OLD (30 days old) excluded from 7-day window, got %+v", res.UnsettledTransactions)
	}
	if res.Total != 2 {
		t.Fatalf("Total: got %d, want 2", res.Total)
	}
	if len(res.UnsettledTransactions) != 2 {
		t.Fatalf("len(UnsettledTransactions): got %d, want 2", len(res.UnsettledTransactions))
	}
}

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
