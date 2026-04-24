package forecast

import (
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/reconcile"
)

// bkk returns midnight on the given Bangkok-local date as a time.Time.
func bkk(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, domain.BangkokTZ())
}

// pendingTxn builds a ReconciledTransaction in StatusPending with the given
// expected settle date. Helper for the tests below.
func pendingTxn(id string, acq domain.Acquirer, amountMinor int64, expSettle time.Time) domain.ReconciledTransaction {
	return domain.ReconciledTransaction{
		Transaction: domain.Transaction{
			ID:                 id,
			Acquirer:           acq,
			AmountMinor:        amountMinor,
			Currency:           "THB",
			TransactionDate:    expSettle.AddDate(0, 0, -2),
			PaymentMethod:      domain.MethodCreditCard,
			ExpectedSettleDate: expSettle,
		},
		Status: domain.StatusPending,
	}
}

func TestForecastIncludesFuturePending(t *testing.T) {
	asOf := bkk(2025, 11, 10)
	expDay := bkk(2025, 11, 13) // 3 days out
	// 10000 minor (100.00 THB) on ThaiAcquirer -> fee = 2.5% = 250 minor; net = 9750.
	rt := pendingTxn("TXN-1", domain.AcquirerThai, 10000, expDay)
	rec := reconcile.Result{Reconciled: []domain.ReconciledTransaction{rt}}

	res := Forecast(rec, asOf, 7)

	if got, want := len(res.ForecastByDay), 1; got != want {
		t.Fatalf("ForecastByDay length: got %d, want %d", got, want)
	}
	day := res.ForecastByDay[0]
	if day.Date != "2025-11-13" {
		t.Errorf("date: got %q, want %q", day.Date, "2025-11-13")
	}
	if got, want := day.Total, domain.FormatMinorUnits(9750); got != want {
		t.Errorf("day.Total: got %q, want %q", got, want)
	}
	if len(day.ByAcquirer) != 1 || day.ByAcquirer[0].Acquirer != domain.AcquirerThai {
		t.Fatalf("by_acquirer: got %+v", day.ByAcquirer)
	}
	if day.ByAcquirer[0].TxnCount != 1 {
		t.Errorf("txn_count: got %d, want 1", day.ByAcquirer[0].TxnCount)
	}
	if res.GrandTotal != domain.FormatMinorUnits(9750) {
		t.Errorf("GrandTotal: got %q, want %q", res.GrandTotal, domain.FormatMinorUnits(9750))
	}
}

func TestForecastSkipsOverdueAndSettled(t *testing.T) {
	asOf := bkk(2025, 11, 10)
	pendingFuture := pendingTxn("TXN-PF", domain.AcquirerThai, 20000, bkk(2025, 11, 12))

	overdue := domain.ReconciledTransaction{
		Transaction: domain.Transaction{
			ID:                 "TXN-OD",
			Acquirer:           domain.AcquirerThai,
			AmountMinor:        50000,
			Currency:           "THB",
			ExpectedSettleDate: bkk(2025, 11, 5),
		},
		Status: domain.StatusOverdue,
	}
	settled := domain.ReconciledTransaction{
		Transaction: domain.Transaction{
			ID:                 "TXN-S",
			Acquirer:           domain.AcquirerThai,
			AmountMinor:        77000,
			Currency:           "THB",
			ExpectedSettleDate: bkk(2025, 11, 13),
		},
		Status: domain.StatusSettled,
	}

	rec := reconcile.Result{
		Reconciled: []domain.ReconciledTransaction{pendingFuture, overdue, settled},
	}
	res := Forecast(rec, asOf, 7)

	if len(res.ForecastByDay) != 1 {
		t.Fatalf("expected exactly 1 forecasted day (only the future pending), got %d", len(res.ForecastByDay))
	}
	if res.ForecastByDay[0].Date != "2025-11-12" {
		t.Errorf("expected the pending future day, got %q", res.ForecastByDay[0].Date)
	}
	// Sanity check: ensure none of the by_acquirer rows reference the overdue
	// or settled txn IDs by counting txns. There's exactly one pending txn so
	// the total txn count across the day must be 1.
	totalTxns := 0
	for _, item := range res.ForecastByDay[0].ByAcquirer {
		totalTxns += item.TxnCount
	}
	if totalTxns != 1 {
		t.Errorf("expected 1 txn counted, got %d", totalTxns)
	}
}

func TestForecastSkipsBeyondWindow(t *testing.T) {
	asOf := bkk(2025, 11, 10)
	// 10 days out, window is 7 -> must be excluded.
	rt := pendingTxn("TXN-FAR", domain.AcquirerThai, 10000, bkk(2025, 11, 20))
	rec := reconcile.Result{Reconciled: []domain.ReconciledTransaction{rt}}

	res := Forecast(rec, asOf, 7)

	if len(res.ForecastByDay) != 0 {
		t.Errorf("expected no forecasted days for txn beyond window, got %d", len(res.ForecastByDay))
	}
	if res.GrandTotal != domain.FormatMinorUnits(0) {
		t.Errorf("expected zero grand total, got %q", res.GrandTotal)
	}
}
