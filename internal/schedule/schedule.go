package schedule

import (
	"fmt"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

// ExpectedSettlementDate computes the expected settlement date for a given
// acquirer and transaction date. All dates are normalized to Bangkok midnight.
func ExpectedSettlementDate(acquirer domain.Acquirer, txnDate time.Time) (time.Time, error) {
	switch acquirer {
	case domain.AcquirerThai:
		return domain.NextBusinessDay(txnDate), nil
	case domain.AcquirerGlobal:
		return nextGlobalPayWindow(txnDate), nil
	case domain.AcquirerPrompt:
		return domain.AddBusinessDays(txnDate, 3), nil
	default:
		return time.Time{}, fmt.Errorf("schedule: unknown acquirer %q", acquirer)
	}
}

// nextGlobalPayWindow returns the next Tuesday or Friday strictly after txnDate.
// Settlement happens on Tue/Fri windows; same-day is ineligible.
func nextGlobalPayWindow(txnDate time.Time) time.Time {
	d := domain.BangkokMidnight(txnDate).AddDate(0, 0, 1)
	for {
		wd := d.Weekday()
		if wd == time.Tuesday || wd == time.Friday {
			return d
		}
		d = d.AddDate(0, 0, 1)
	}
}
