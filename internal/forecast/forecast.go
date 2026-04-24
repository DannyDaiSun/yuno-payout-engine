// Package forecast predicts future settlement cash flow based on currently
// unsettled (pending) transactions and each acquirer's settlement schedule.
//
// The reconciler already attaches a Status (settled / pending / overdue) to
// every transaction and computes the ExpectedSettleDate via the schedule
// module, so this package's job reduces to:
//   1. take a Reconcile.Result,
//   2. keep only the transactions that are still pending in the future window,
//   3. estimate the net amount (gross minus expected fee) for each one,
//   4. bucket by (Bangkok-day, acquirer) and emit a per-day breakdown.
package forecast

import (
	"sort"
	"time"

	"github.com/dannydaisun/payout-engine/internal/anomaly"
	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/reconcile"
)

// ByAcquirerItem is the per-acquirer slice of a single forecasted day.
// PredictedNet is in formatted minor units (e.g. "1234.56").
type ByAcquirerItem struct {
	Acquirer     domain.Acquirer `json:"acquirer"`
	PredictedNet string          `json:"predicted_net"`
	TxnCount     int             `json:"txn_count"`
}

// DayForecast aggregates predicted settlements landing on a given Bangkok day.
type DayForecast struct {
	Date       string           `json:"date"`
	ByAcquirer []ByAcquirerItem `json:"by_acquirer"`
	Total      string           `json:"total"`
}

// Result is the top-level forecast payload.
type Result struct {
	AsOf          string        `json:"as_of"`
	WindowDays    int           `json:"window_days"`
	Currency      string        `json:"currency"`
	ForecastByDay []DayForecast `json:"forecast_by_day"`
	GrandTotal    string        `json:"grand_total"`
}

// dayKey is a stable map key for a Bangkok day.
type dayKey struct {
	Year  int
	Month time.Month
	Day   int
}

func dayKeyOf(t time.Time) dayKey {
	bt := domain.BangkokMidnight(t)
	return dayKey{bt.Year(), bt.Month(), bt.Day()}
}

// Forecast returns a per-day cashflow prediction for the next `days` days
// starting from asOf+1 (Bangkok-day boundaries). Days with zero predicted
// settlements are omitted from the output to keep payloads small; consumers
// can interpret a missing day as "no expected cashflow".
//
// Predicted net per transaction = AmountMinor - ExpectedFee(Acquirer, Amount).
// If the acquirer is unknown to anomaly.ExpectedFee, we fall back to the gross
// amount (i.e., assume zero fee) rather than dropping the txn — better to
// over-predict cash than to silently lose visibility on a settlement.
func Forecast(r reconcile.Result, asOf time.Time, days int) Result {
	asOfDay := domain.BangkokMidnight(asOf)
	currency := ""
	res := Result{
		AsOf:          asOfDay.Format("2006-01-02"),
		WindowDays:    days,
		ForecastByDay: []DayForecast{},
		GrandTotal:    domain.FormatMinorUnits(0),
	}
	if days < 1 || days > 14 {
		// Defensive: HTTP layer is the source of truth for validation, but a
		// direct caller passing 0 or 100 should still get a well-formed empty
		// payload rather than nonsense buckets.
		return res
	}
	maxDay := asOfDay.AddDate(0, 0, days)

	type bucket struct {
		acquirer map[domain.Acquirer]*ByAcquirerItem
		total    int64
		netByAcq map[domain.Acquirer]int64
	}
	dayBuckets := make(map[dayKey]*bucket)
	dayOrder := make(map[dayKey]time.Time)

	for _, rt := range r.Reconciled {
		if rt.Status != domain.StatusPending {
			continue
		}
		expDay := domain.BangkokMidnight(rt.Transaction.ExpectedSettleDate)
		// Only include strictly-future days within the window. Same-day
		// (asOfDay) is excluded because the spec asks for the *next* N days.
		if !expDay.After(asOfDay) {
			continue
		}
		if expDay.After(maxDay) {
			continue
		}
		fee, err := anomaly.ExpectedFee(rt.Transaction.Acquirer, rt.Transaction.AmountMinor)
		if err != nil {
			// Unknown acquirer: treat fee as zero so the cash forecast still
			// surfaces this txn. Operators can spot the missing fee policy
			// elsewhere (anomaly module logs unknowns).
			fee = 0
		}
		net := rt.Transaction.AmountMinor - fee
		if currency == "" {
			currency = rt.Transaction.Currency
		}
		k := dayKeyOf(expDay)
		b, ok := dayBuckets[k]
		if !ok {
			b = &bucket{
				acquirer: make(map[domain.Acquirer]*ByAcquirerItem),
				netByAcq: make(map[domain.Acquirer]int64),
			}
			dayBuckets[k] = b
			dayOrder[k] = expDay
		}
		item, ok := b.acquirer[rt.Transaction.Acquirer]
		if !ok {
			item = &ByAcquirerItem{Acquirer: rt.Transaction.Acquirer}
			b.acquirer[rt.Transaction.Acquirer] = item
		}
		item.TxnCount++
		b.netByAcq[rt.Transaction.Acquirer] += net
		b.total += net
	}

	// Sort days ascending for stable, human-friendly output.
	keys := make([]dayKey, 0, len(dayBuckets))
	for k := range dayBuckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return dayOrder[keys[i]].Before(dayOrder[keys[j]])
	})

	var grand int64
	for _, k := range keys {
		b := dayBuckets[k]
		// Sort acquirer items deterministically by name so output is stable
		// across runs (map iteration order is otherwise random).
		acqNames := make([]domain.Acquirer, 0, len(b.acquirer))
		for a := range b.acquirer {
			acqNames = append(acqNames, a)
		}
		sort.Slice(acqNames, func(i, j int) bool {
			return string(acqNames[i]) < string(acqNames[j])
		})
		items := make([]ByAcquirerItem, 0, len(acqNames))
		for _, a := range acqNames {
			it := b.acquirer[a]
			it.PredictedNet = domain.FormatMinorUnits(b.netByAcq[a])
			items = append(items, *it)
		}
		res.ForecastByDay = append(res.ForecastByDay, DayForecast{
			Date:       dayOrder[k].Format("2006-01-02"),
			ByAcquirer: items,
			Total:      domain.FormatMinorUnits(b.total),
		})
		grand += b.total
	}
	res.Currency = currency
	res.GrandTotal = domain.FormatMinorUnits(grand)
	return res
}
