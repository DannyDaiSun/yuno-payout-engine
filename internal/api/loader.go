package api

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

// LoadTransactionsCSV parses the canonical transaction fixture format:
// id,acquirer,amount,currency,transaction_date,payment_method
func LoadTransactionsCSV(r io.Reader, sourceFile string) ([]domain.Transaction, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("transactions csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("transactions csv: empty file")
	}
	header := rows[0]
	idx := make(map[string]int)
	for i, h := range header {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	required := []string{"id", "acquirer", "amount", "currency", "transaction_date", "payment_method"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("transactions csv: missing column %q", col)
		}
	}
	out := make([]domain.Transaction, 0, len(rows)-1)
	for i, row := range rows[1:] {
		if len(row) < len(header) {
			return nil, fmt.Errorf("transactions csv: row %d short", i+2)
		}
		amt, err := domain.ParseMinorUnits(strings.TrimSpace(row[idx["amount"]]))
		if err != nil {
			return nil, fmt.Errorf("transactions csv: row %d amount: %w", i+2, err)
		}
		dateStr := strings.TrimSpace(row[idx["transaction_date"]])
		txnDate, err := time.ParseInLocation("2006-01-02", dateStr, domain.BangkokTZ())
		if err != nil {
			return nil, fmt.Errorf("transactions csv: row %d date: %w", i+2, err)
		}
		out = append(out, domain.Transaction{
			ID:              strings.TrimSpace(row[idx["id"]]),
			Acquirer:        domain.Acquirer(strings.TrimSpace(row[idx["acquirer"]])),
			AmountMinor:     amt,
			Currency:        strings.TrimSpace(row[idx["currency"]]),
			TransactionDate: txnDate,
			PaymentMethod:   domain.PaymentMethod(strings.TrimSpace(row[idx["payment_method"]])),
		})
	}
	return out, nil
}
