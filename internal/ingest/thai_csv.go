package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

// utf8BOM is the byte sequence some producers prepend to UTF-8
// CSV files. It must be stripped from the first header cell so that
// header-name lookups continue to work.
const utf8BOM = "\uFEFF"

// ParseThaiCSV parses a CSV stream from the Thai acquirer.
// CSV columns: txn_ref, transaction_date, settlement_date, gross_amt, fee_amt, net_amt, payment_method
// Date format: 2006-01-02. Currency is THB. Mapping is by header name (not column position).
func ParseThaiCSV(r io.Reader, sourceFile string) ([]domain.SettlementRecord, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("thai_csv: read header: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], utf8BOM)
	}

	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[h] = i
	}

	required := []string{
		"txn_ref", "transaction_date", "settlement_date",
		"gross_amt", "fee_amt", "net_amt", "payment_method",
	}
	for _, c := range required {
		if _, ok := idx[c]; !ok {
			return nil, fmt.Errorf("thai_csv: missing required column %q", c)
		}
	}

	const dateLayout = "2006-01-02"
	var out []domain.SettlementRecord
	lineNum := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			return nil, fmt.Errorf("thai_csv: line %d: %w", lineNum, err)
		}

		if len(row) < len(header) {
			return nil, fmt.Errorf("thai csv: row %d: short row (got %d fields, want %d)", lineNum, len(row), len(header))
		}

		txnDate, err := time.ParseInLocation(dateLayout, row[idx["transaction_date"]], domain.BangkokTZ())
		if err != nil {
			return nil, fmt.Errorf("thai_csv: line %d: transaction_date: %w", lineNum, err)
		}
		setDate, err := time.ParseInLocation(dateLayout, row[idx["settlement_date"]], domain.BangkokTZ())
		if err != nil {
			return nil, fmt.Errorf("thai_csv: line %d: settlement_date: %w", lineNum, err)
		}

		gross, err := domain.ParseMinorUnits(row[idx["gross_amt"]])
		if err != nil {
			return nil, fmt.Errorf("thai_csv: line %d: gross_amt: %w", lineNum, err)
		}
		fee, err := domain.ParseMinorUnits(row[idx["fee_amt"]])
		if err != nil {
			return nil, fmt.Errorf("thai_csv: line %d: fee_amt: %w", lineNum, err)
		}
		net, err := domain.ParseMinorUnits(row[idx["net_amt"]])
		if err != nil {
			return nil, fmt.Errorf("thai_csv: line %d: net_amt: %w", lineNum, err)
		}

		out = append(out, domain.SettlementRecord{
			TransactionID:   row[idx["txn_ref"]],
			Acquirer:        domain.AcquirerThai,
			GrossMinor:      gross,
			FeeMinor:        fee,
			NetMinor:        net,
			Currency:        "THB",
			TransactionDate: txnDate,
			SettlementDate:  setDate,
			PaymentMethod:   domain.PaymentMethod(row[idx["payment_method"]]),
			SourceFile:      sourceFile,
		})
	}

	return out, nil
}
