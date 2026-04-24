package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

type promptRawRecord struct {
	TransactionID string          `json:"transaction_id"`
	TxnDate       string          `json:"txn_date"`
	SettleDate    string          `json:"settle_date"`
	Amount        json.RawMessage `json:"amount"`
	MerchantFee   json.RawMessage `json:"merchant_fee"`
	NetPayout     json.RawMessage `json:"net_payout"`
	Channel       string          `json:"channel"`
}

// parseNumericAmount enforces strict-numeric JSON for monetary fields.
// JSON strings (leading double-quote) are rejected so that mismatches between
// the upstream generator and the parser surface immediately rather than
// silently coercing through json.Number's underlying string type.
func parseNumericAmount(raw json.RawMessage, field string) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("%s: missing or null", field)
	}
	if raw[0] == '"' {
		return 0, fmt.Errorf("%s: must be a JSON number, got string", field)
	}
	return domain.ParseMinorUnits(string(raw))
}

// ParsePromptJSON parses a JSON array stream from the PromptPay processor.
// Fields: transaction_id, txn_date, settle_date, amount, merchant_fee, net_payout, channel.
// Dates are RFC3339. Amounts arrive as JSON numbers; we read them via json.Number then
// reuse domain.ParseMinorUnits to keep precision.
func ParsePromptJSON(r io.Reader, sourceFile string) ([]domain.SettlementRecord, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	var raws []promptRawRecord
	if err := dec.Decode(&raws); err != nil {
		return nil, fmt.Errorf("prompt_json: decode: %w", err)
	}

	out := make([]domain.SettlementRecord, 0, len(raws))
	for i, raw := range raws {
		if raw.TransactionID == "" {
			return nil, fmt.Errorf("prompt_json: record %d: transaction_id is required", i)
		}
		txnDate, err := time.Parse(time.RFC3339, raw.TxnDate)
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: txn_date: %w", i, err)
		}
		setDate, err := time.Parse(time.RFC3339, raw.SettleDate)
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: settle_date: %w", i, err)
		}

		gross, err := parseNumericAmount(raw.Amount, "amount")
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: %w", i, err)
		}
		fee, err := parseNumericAmount(raw.MerchantFee, "merchant_fee")
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: %w", i, err)
		}
		net, err := parseNumericAmount(raw.NetPayout, "net_payout")
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: %w", i, err)
		}

		out = append(out, domain.SettlementRecord{
			TransactionID:   raw.TransactionID,
			Acquirer:        domain.AcquirerPrompt,
			GrossMinor:      gross,
			FeeMinor:        fee,
			NetMinor:        net,
			Currency:        "THB",
			TransactionDate: txnDate,
			SettlementDate:  setDate,
			PaymentMethod:   domain.PaymentMethod(raw.Channel),
			SourceFile:      sourceFile,
		})
	}

	return out, nil
}
