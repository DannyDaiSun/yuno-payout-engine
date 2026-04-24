package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

// ParseGlobalCSV parses a CSV stream from the GlobalPay acquirer.
// CSV columns: reference_number, processed_on, payout_date, original_amount, processing_fee, settled_amount, type
// Date format: 02/01/2006. Currency is THB. Mapping is by header name (not column position).
func ParseGlobalCSV(r io.Reader, sourceFile string) ([]domain.SettlementRecord, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("global_csv: read header: %w", err)
	}

	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[h] = i
	}

	required := []string{
		"reference_number", "processed_on", "payout_date",
		"original_amount", "processing_fee", "settled_amount", "type",
	}
	for _, c := range required {
		if _, ok := idx[c]; !ok {
			return nil, fmt.Errorf("global_csv: missing required column %q", c)
		}
	}

	const dateLayout = "02/01/2006"
	var out []domain.SettlementRecord
	lineNum := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			return nil, fmt.Errorf("global_csv: line %d: %w", lineNum, err)
		}

		txnDate, err := time.Parse(dateLayout, row[idx["processed_on"]])
		if err != nil {
			return nil, fmt.Errorf("global_csv: line %d: processed_on: %w", lineNum, err)
		}
		setDate, err := time.Parse(dateLayout, row[idx["payout_date"]])
		if err != nil {
			return nil, fmt.Errorf("global_csv: line %d: payout_date: %w", lineNum, err)
		}

		gross, err := domain.ParseMinorUnits(row[idx["original_amount"]])
		if err != nil {
			return nil, fmt.Errorf("global_csv: line %d: original_amount: %w", lineNum, err)
		}
		fee, err := domain.ParseMinorUnits(row[idx["processing_fee"]])
		if err != nil {
			return nil, fmt.Errorf("global_csv: line %d: processing_fee: %w", lineNum, err)
		}
		net, err := domain.ParseMinorUnits(row[idx["settled_amount"]])
		if err != nil {
			return nil, fmt.Errorf("global_csv: line %d: settled_amount: %w", lineNum, err)
		}

		out = append(out, domain.SettlementRecord{
			TransactionID:   row[idx["reference_number"]],
			Acquirer:        domain.AcquirerGlobal,
			GrossMinor:      gross,
			FeeMinor:        fee,
			NetMinor:        net,
			Currency:        "THB",
			TransactionDate: txnDate,
			SettlementDate:  setDate,
			PaymentMethod:   domain.PaymentMethod(row[idx["type"]]),
			SourceFile:      sourceFile,
		})
	}

	return out, nil
}
