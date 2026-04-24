package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

type promptRawRecord struct {
	TransactionID string      `json:"transaction_id"`
	TxnDate       string      `json:"txn_date"`
	SettleDate    string      `json:"settle_date"`
	Amount        json.Number `json:"amount"`
	MerchantFee   json.Number `json:"merchant_fee"`
	NetPayout     json.Number `json:"net_payout"`
	Channel       string      `json:"channel"`
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
		txnDate, err := time.Parse(time.RFC3339, raw.TxnDate)
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: txn_date: %w", i, err)
		}
		setDate, err := time.Parse(time.RFC3339, raw.SettleDate)
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: settle_date: %w", i, err)
		}

		gross, err := domain.ParseMinorUnits(string(raw.Amount))
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: amount: %w", i, err)
		}
		fee, err := domain.ParseMinorUnits(string(raw.MerchantFee))
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: merchant_fee: %w", i, err)
		}
		net, err := domain.ParseMinorUnits(string(raw.NetPayout))
		if err != nil {
			return nil, fmt.Errorf("prompt_json: record %d: net_payout: %w", i, err)
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
