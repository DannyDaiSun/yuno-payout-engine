package anomaly

import (
	"fmt"
	"math"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

type Anomaly struct {
	TransactionID string          `json:"transaction_id"`
	Acquirer      domain.Acquirer `json:"acquirer"`
	GrossAmount   string          `json:"gross_amount"`
	ActualFee     string          `json:"actual_fee"`
	ExpectedFee   string          `json:"expected_fee"`
	DeviationPct  float64         `json:"deviation_pct"`
	Severity      string          `json:"severity"`
}

// pctMinor computes (grossMinor * basisPoints) / 10000 with half-up rounding.
func pctMinor(grossMinor, basisPoints int64) int64 {
	return (grossMinor*basisPoints + 5000) / 10000
}

// ExpectedFee computes the expected fee in minor units for a given acquirer + gross amount.
//   - ThaiAcquirer:        2.5% of gross
//   - GlobalPay:           10 THB + 2% of gross (10 THB = 1000 minor units)
//   - PromptPayProcessor:  tiered: <5000 THB = 1.5%, 5000-20000 = 1.8%, >20000 = 2.2%
func ExpectedFee(acquirer domain.Acquirer, grossMinor int64) (int64, error) {
	switch acquirer {
	case domain.AcquirerThai:
		return pctMinor(grossMinor, 250), nil
	case domain.AcquirerGlobal:
		return 1000 + pctMinor(grossMinor, 200), nil
	case domain.AcquirerPrompt:
		switch {
		case grossMinor < 500000:
			return pctMinor(grossMinor, 150), nil
		case grossMinor <= 2000000:
			return pctMinor(grossMinor, 180), nil
		default:
			return pctMinor(grossMinor, 220), nil
		}
	default:
		return 0, fmt.Errorf("unknown acquirer: %s", acquirer)
	}
}

// Detect inspects each settlement record. Returns anomalies where actual fee differs
// from expected by more than 1 minor unit.
//
// Severity:
//
//	warning  : 1 < deviation < 10% of expected
//	critical : deviation >= 10% of expected (or expected == 0 and actual > 0)
func Detect(settlements []domain.SettlementRecord) []Anomaly {
	out := make([]Anomaly, 0)
	for _, s := range settlements {
		expected, err := ExpectedFee(s.Acquirer, s.GrossMinor)
		if err != nil {
			continue
		}
		diff := s.FeeMinor - expected
		absDiff := diff
		if absDiff < 0 {
			absDiff = -absDiff
		}
		if absDiff <= 1 {
			continue
		}
		var severity string
		var deviationPct float64
		if expected == 0 {
			if s.FeeMinor > 0 {
				severity = "critical"
				deviationPct = math.Inf(1)
			} else {
				continue
			}
		} else {
			ratio := float64(absDiff) / float64(expected)
			deviationPct = float64(diff) / float64(expected) * 100.0
			if ratio >= 0.10 {
				severity = "critical"
			} else {
				severity = "warning"
			}
		}
		out = append(out, Anomaly{
			TransactionID: s.TransactionID,
			Acquirer:      s.Acquirer,
			GrossAmount:   domain.FormatMinorUnits(s.GrossMinor),
			ActualFee:     domain.FormatMinorUnits(s.FeeMinor),
			ExpectedFee:   domain.FormatMinorUnits(expected),
			DeviationPct:  deviationPct,
			Severity:      severity,
		})
	}
	return out
}
