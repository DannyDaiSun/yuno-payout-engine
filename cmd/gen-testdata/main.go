// Command gen-testdata generates deterministic fixture files for the
// Bangkok Settlement Maze challenge. Run as: `go run ./cmd/gen-testdata`.
//
// SMOKE PHASE: minimal generator, deterministic with fixed seed.
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
)

// Acquirer names — must match what the engine expects.
const (
	acqThai      = "ThaiAcquirer"
	acqGlobalPay = "GlobalPay"
	acqPromptPay = "PromptPayProcessor"
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
}

// round2 rounds to 2 decimals — money math.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// generateTxns builds the deterministic transaction list.
// Distribution: 30 txns total, 10 per acquirer, round-robin.
func generateTxns(r *rand.Rand) []txn {
	const total = 30
	acquirers := []string{acqThai, acqGlobalPay, acqPromptPay}
	out := make([]txn, 0, total)
	for i := 0; i < total; i++ {
		amt := float64(r.Intn(49901) + 100) // 100..50000 inclusive
		// Spread dates evenly over past 30 days.
		daysAgo := i % 30
		date := today.AddDate(0, 0, -daysAgo)
		out = append(out, txn{
			ID:            fmt.Sprintf("TXN%03d", i+1),
			Acquirer:      acquirers[i%3],
			Amount:        round2(amt),
			Currency:      "THB",
			TxnDate:       date,
			PaymentMethod: paymentMethods[i%len(paymentMethods)],
		})
	}
	return out
}

// filterByAcquirer returns up to n transactions for the given acquirer,
// preserving original order so output is deterministic.
func filterByAcquirer(all []txn, acq string, n int) []txn {
	out := make([]txn, 0, n)
	for _, t := range all {
		if t.Acquirer == acq {
			out = append(out, t)
			if len(out) == n {
				break
			}
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
// fee = 2.5%, settlement = txn_date + 1 day.
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
		settle := t.TxnDate.AddDate(0, 0, 1)
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
// SMOKE: payout = processed + 3 days; fee = 10 + 2% gross. Date format DD/MM/YYYY.
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
		payout := t.TxnDate.AddDate(0, 0, 3)
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
type promptPayEntry struct {
	TransactionID string `json:"transaction_id"`
	TxnDate       string `json:"txn_date"`
	SettleDate    string `json:"settle_date"`
	Amount        string `json:"amount"`
	MerchantFee   string `json:"merchant_fee"`
	NetPayout     string `json:"net_payout"`
	Channel       string `json:"channel"`
}

// writePromptPayJSON writes data/settlements/promptpay.json.
// Tiered fee: <5000 -> 1.5%, 5000-20000 -> 1.8%, >20000 -> 2.2%.
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
		settle := t.TxnDate.AddDate(0, 0, 3)
		entries = append(entries, promptPayEntry{
			TransactionID: t.ID,
			TxnDate:       t.TxnDate.Format(time.RFC3339),
			SettleDate:    settle.Format(time.RFC3339),
			Amount:        fmt.Sprintf("%.2f", t.Amount),
			MerchantFee:   fmt.Sprintf("%.2f", fee),
			NetPayout:     fmt.Sprintf("%.2f", net),
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
	if err := writeThaiCSV(filepath.Join(outDir, "settlements", "thai_acquirer.csv"), filterByAcquirer(all, acqThai, 10)); err != nil {
		return err
	}
	if err := writeGlobalPayCSV(filepath.Join(outDir, "settlements", "global_pay.csv"), filterByAcquirer(all, acqGlobalPay, 10)); err != nil {
		return err
	}
	if err := writePromptPayJSON(filepath.Join(outDir, "settlements", "promptpay.json"), filterByAcquirer(all, acqPromptPay, 10)); err != nil {
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
