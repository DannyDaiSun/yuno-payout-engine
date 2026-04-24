// Command gen-testdata generates deterministic fixture files for the
// Bangkok Settlement Maze challenge. Run as: `go run ./cmd/gen-testdata`.
//
// Spec-scale generator: 300 transactions (100 per acquirer) and 70
// settlement records per acquirer file. The remaining 30 unsettled
// transactions per acquirer are split: 20 "recent" (will appear as
// PENDING at reconcile-time) and 10 "old" (will appear as OVERDUE).
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/schedule"
)

// Acquirer names — must match what the engine expects.
const (
	acqThai      = string(domain.AcquirerThai)
	acqGlobalPay = string(domain.AcquirerGlobal)
	acqPromptPay = string(domain.AcquirerPrompt)
)

// Volume constants (spec).
const (
	txnsPerAcquirer    = 100
	settledPerAcquirer = 70
	pendingPerAcquirer = 20 // unsettled, recent date  -> PENDING
	overduePerAcquirer = 10 // unsettled, old date     -> OVERDUE
	totalTxns          = txnsPerAcquirer * 3           // 300
)

// "Today" is fixed for determinism. The challenge sets 2026-04-23 as the
// reference date; we anchor at midnight Bangkok time.
var bangkok = time.FixedZone("ICT", 7*3600)
var today = time.Date(2026, 4, 23, 0, 0, 0, 0, bangkok)

// payment methods rotate
var paymentMethods = []string{"credit_card", "promptpay", "truemoney_wallet", "bank_transfer"}

// txn is the in-memory representation we share across files.
type txn struct {
	ID            string
	Acquirer      string
	Amount        float64 // THB, 2-decimal
	Currency      string
	TxnDate       time.Time
	PaymentMethod string
	// Settled is true if a settlement record will be emitted for this txn.
	Settled bool
}

// round2 rounds to 2 decimals — money math.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// generateTxns builds the deterministic transaction list.
//
// Distribution per acquirer (100 txns):
//   - first 20 positions  -> unsettled, recent (daysAgo in [0,4])  -> pending
//   - next  10 positions  -> unsettled, old    (daysAgo in [11,29]) -> overdue
//   - last  70 positions  -> settled, spread evenly across [0,29]
//
// Acquirers are assigned round-robin by global index so that each acquirer
// receives exactly 100 transactions and the total is 300.
func generateTxns(r *rand.Rand) []txn {
	acquirers := []string{acqThai, acqGlobalPay, acqPromptPay}
	out := make([]txn, 0, totalTxns)

	// Per-acquirer position counter so we can decide settled/date bucket.
	perAcqPos := map[string]int{
		acqThai:      0,
		acqGlobalPay: 0,
		acqPromptPay: 0,
	}

	for i := 0; i < totalTxns; i++ {
		acq := acquirers[i%3]
		pos := perAcqPos[acq]
		perAcqPos[acq] = pos + 1

		var daysAgo int
		var settled bool
		switch {
		case pos < pendingPerAcquirer:
			// recent unsettled -> PENDING at reconcile time
			daysAgo = pos % 5 // 0..4
			settled = false
		case pos < pendingPerAcquirer+overduePerAcquirer:
			// old unsettled -> OVERDUE at reconcile time
			daysAgo = 11 + ((pos - pendingPerAcquirer) % 19) // 11..29
			settled = false
		default:
			// settled: spread across past 30 days
			daysAgo = (pos - pendingPerAcquirer - overduePerAcquirer) % 30
			settled = true
		}
		date := today.AddDate(0, 0, -daysAgo)

		amt := float64(r.Intn(49901) + 100) // 100..50000 inclusive
		out = append(out, txn{
			ID:            fmt.Sprintf("TXN%04d", i+1),
			Acquirer:      acq,
			Amount:        round2(amt),
			Currency:      "THB",
			TxnDate:       date,
			PaymentMethod: paymentMethods[i%len(paymentMethods)],
			Settled:       settled,
		})
	}
	return out
}

// settledByAcquirer returns the settled transactions for a given acquirer,
// preserving original order so output is deterministic.
func settledByAcquirer(all []txn, acq string) []txn {
	out := make([]txn, 0, settledPerAcquirer)
	for _, t := range all {
		if t.Acquirer == acq && t.Settled {
			out = append(out, t)
		}
	}
	return out
}

// writeTransactionsCSV writes data/transactions.csv.
func writeTransactionsCSV(path string, txns []txn) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"id", "acquirer", "amount", "currency", "transaction_date", "payment_method"}); err != nil {
		return err
	}
	for _, t := range txns {
		row := []string{
			t.ID,
			t.Acquirer,
			fmt.Sprintf("%.2f", t.Amount),
			t.Currency,
			t.TxnDate.Format("2006-01-02"),
			t.PaymentMethod,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// writeThaiCSV writes data/settlements/thai_acquirer.csv.
// fee = 2.5%, settlement = next business day after txn_date (Mon-Fri).
func writeThaiCSV(path string, txns []txn) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"txn_ref", "transaction_date", "settlement_date", "gross_amt", "fee_amt", "net_amt", "payment_method"}); err != nil {
		return err
	}
	for _, t := range txns {
		fee := round2(t.Amount * 0.025)
		net := round2(t.Amount - fee)
		settle, err := schedule.ExpectedSettlementDate(domain.AcquirerThai, t.TxnDate)
		if err != nil {
			return err
		}
		row := []string{
			t.ID,
			t.TxnDate.Format("2006-01-02"),
			settle.Format("2006-01-02"),
			fmt.Sprintf("%.2f", t.Amount),
			fmt.Sprintf("%.2f", fee),
			fmt.Sprintf("%.2f", net),
			t.PaymentMethod,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// writeGlobalPayCSV writes data/settlements/global_pay.csv.
// payout = next Tue/Fri window after processed; fee = 10 + 2% gross. Date format DD/MM/YYYY.
func writeGlobalPayCSV(path string, txns []txn) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"reference_number", "processed_on", "payout_date", "original_amount", "processing_fee", "settled_amount", "type"}); err != nil {
		return err
	}
	for _, t := range txns {
		fee := round2(10 + t.Amount*0.02)
		settled := round2(t.Amount - fee)
		payout, err := schedule.ExpectedSettlementDate(domain.AcquirerGlobal, t.TxnDate)
		if err != nil {
			return err
		}
		row := []string{
			t.ID,
			t.TxnDate.Format("02/01/2006"),
			payout.Format("02/01/2006"),
			fmt.Sprintf("%.2f", t.Amount),
			fmt.Sprintf("%.2f", fee),
			fmt.Sprintf("%.2f", settled),
			t.PaymentMethod,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// promptPayEntry mirrors the JSON shape required for the PromptPay file.
// Amount fields use json.RawMessage so they marshal as numeric literals
// (e.g. 49140.00) rather than quoted strings, while preserving the
// fixed 2-decimal precision required by downstream parsers.
type promptPayEntry struct {
	TransactionID string          `json:"transaction_id"`
	TxnDate       string          `json:"txn_date"`
	SettleDate    string          `json:"settle_date"`
	Amount        json.RawMessage `json:"amount"`
	MerchantFee   json.RawMessage `json:"merchant_fee"`
	NetPayout     json.RawMessage `json:"net_payout"`
	Channel       string          `json:"channel"`
}

// money2 returns a JSON numeric literal with exactly 2 decimal places,
// e.g. 49140.00. The caller must ensure the input is a finite, non-negative
// THB amount that has already been rounded to 2 decimals.
func money2(v float64) json.RawMessage {
	return json.RawMessage(fmt.Sprintf("%.2f", v))
}

// writePromptPayJSON writes data/settlements/promptpay.json.
// Tiered fee: <5000 -> 1.5%, 5000-20000 -> 1.8%, >20000 -> 2.2%.
// Settlement date follows the 3-business-day rule.
func writePromptPayJSON(path string, txns []txn) error {
	entries := make([]promptPayEntry, 0, len(txns))
	for _, t := range txns {
		var rate float64
		switch {
		case t.Amount < 5000:
			rate = 0.015
		case t.Amount <= 20000:
			rate = 0.018
		default:
			rate = 0.022
		}
		fee := round2(t.Amount * rate)
		net := round2(t.Amount - fee)
		settle, err := schedule.ExpectedSettlementDate(domain.AcquirerPrompt, t.TxnDate)
		if err != nil {
			return err
		}
		entries = append(entries, promptPayEntry{
			TransactionID: t.ID,
			TxnDate:       t.TxnDate.Format(time.RFC3339),
			SettleDate:    settle.Format(time.RFC3339),
			Amount:        money2(t.Amount),
			MerchantFee:   money2(fee),
			NetPayout:     money2(net),
			Channel:       t.PaymentMethod,
		})
	}
	// MarshalIndent for human-readable diffs; deterministic since slice order is fixed.
	buf, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

// Generate writes all four fixture files under outDir.
// Exposed so the test can invoke it twice with the same seed.
func Generate(seed int64, outDir string) error {
	r := rand.New(rand.NewSource(seed))
	if err := os.MkdirAll(filepath.Join(outDir, "settlements"), 0o755); err != nil {
		return err
	}
	all := generateTxns(r)
	if err := writeTransactionsCSV(filepath.Join(outDir, "transactions.csv"), all); err != nil {
		return err
	}
	if err := writeThaiCSV(filepath.Join(outDir, "settlements", "thai_acquirer.csv"), settledByAcquirer(all, acqThai)); err != nil {
		return err
	}
	if err := writeGlobalPayCSV(filepath.Join(outDir, "settlements", "global_pay.csv"), settledByAcquirer(all, acqGlobalPay)); err != nil {
		return err
	}
	if err := writePromptPayJSON(filepath.Join(outDir, "settlements", "promptpay.json"), settledByAcquirer(all, acqPromptPay)); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := Generate(42, "data"); err != nil {
		fmt.Fprintln(os.Stderr, "gen-testdata failed:", err)
		os.Exit(1)
	}
	fmt.Println("gen-testdata: wrote fixtures under data/")
}
