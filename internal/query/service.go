package query

import (
	"errors"
	"fmt"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/reconcile"
	"github.com/dannydaisun/payout-engine/internal/store"
)

var (
	ErrInvalidMonth = errors.New("invalid month format, want YYYY-MM")
	ErrInvalidDays  = errors.New("days must be > 0")
)

type Service struct {
	store *store.Store
}

func New(s *store.Store) *Service {
	return &Service{store: s}
}

type CashflowItem struct {
	Acquirer  domain.Acquirer `json:"acquirer"`
	NetAmount string          `json:"net_amount"`
}

type CashflowResult struct {
	Date       string         `json:"date"`
	Currency   string         `json:"currency"`
	ByAcquirer []CashflowItem `json:"by_acquirer"`
	Total      string         `json:"total"`
}

func (s *Service) ExpectedCashByAcquirer(date time.Time) CashflowResult {
	day := domain.BangkokMidnight(date)
	settlements := s.store.ListSettlements()
	byAcq := make(map[domain.Acquirer]int64)
	for _, r := range settlements {
		if domain.BangkokMidnight(r.SettlementDate).Equal(day) {
			byAcq[r.Acquirer] += r.NetMinor
		}
	}
	items := make([]CashflowItem, 0, len(byAcq))
	var total int64
	for _, acq := range []domain.Acquirer{domain.AcquirerThai, domain.AcquirerGlobal, domain.AcquirerPrompt} {
		amt := byAcq[acq]
		items = append(items, CashflowItem{
			Acquirer:  acq,
			NetAmount: domain.FormatMinorUnits(amt),
		})
		total += amt
	}
	return CashflowResult{
		Date:       day.Format("2006-01-02"),
		Currency:   "THB",
		ByAcquirer: items,
		Total:      domain.FormatMinorUnits(total),
	}
}

type UnsettledTransaction struct {
	ID                 string                  `json:"id"`
	Acquirer           domain.Acquirer         `json:"acquirer"`
	Amount             string                  `json:"amount"`
	TransactionDate    string                  `json:"transaction_date"`
	ExpectedSettleDate string                  `json:"expected_settle_date"`
	Status             domain.SettlementStatus `json:"status"`
	PaymentMethod      domain.PaymentMethod    `json:"payment_method"`
}

type UnsettledResult struct {
	AsOf                  string                 `json:"as_of"`
	WindowDays            int                    `json:"window_days"`
	Currency              string                 `json:"currency"`
	UnsettledTransactions []UnsettledTransaction `json:"unsettled_transactions"`
	Total                 int                    `json:"total"`
}

func (s *Service) UnsettledSince(days int, asOf time.Time) (UnsettledResult, error) {
	if days <= 0 {
		return UnsettledResult{}, ErrInvalidDays
	}
	asOfDay := domain.BangkokMidnight(asOf)
	cutoff := asOfDay.AddDate(0, 0, -days)
	r := reconcile.Reconcile(s.store.ListTransactions(), s.store.ListSettlements(), asOf)
	out := make([]UnsettledTransaction, 0)
	for _, rt := range r.Reconciled {
		if rt.Status == domain.StatusSettled {
			continue
		}
		txnDay := domain.BangkokMidnight(rt.Transaction.TransactionDate)
		if txnDay.Before(cutoff) {
			continue
		}
		out = append(out, UnsettledTransaction{
			ID:                 rt.Transaction.ID,
			Acquirer:           rt.Transaction.Acquirer,
			Amount:             domain.FormatMinorUnits(rt.Transaction.AmountMinor),
			TransactionDate:    txnDay.Format("2006-01-02"),
			ExpectedSettleDate: domain.BangkokMidnight(rt.Transaction.ExpectedSettleDate).Format("2006-01-02"),
			Status:             rt.Status,
			PaymentMethod:      rt.Transaction.PaymentMethod,
		})
	}
	return UnsettledResult{
		AsOf:                  asOfDay.Format("2006-01-02"),
		WindowDays:            days,
		Currency:              "THB",
		UnsettledTransactions: out,
		Total:                 len(out),
	}, nil
}

type FeesItem struct {
	Acquirer domain.Acquirer `json:"acquirer"`
	Fees     string          `json:"fees"`
}

type FeesResult struct {
	Month          string     `json:"month"`
	Currency       string     `json:"currency"`
	FeesByAcquirer []FeesItem `json:"fees_by_acquirer"`
	Total          string     `json:"total"`
}

func (s *Service) FeesByAcquirer(month string) (FeesResult, error) {
	mt, err := time.ParseInLocation("2006-01", month, domain.BangkokTZ())
	if err != nil {
		return FeesResult{}, fmt.Errorf("%w: %s", ErrInvalidMonth, month)
	}
	monthStart := time.Date(mt.Year(), mt.Month(), 1, 0, 0, 0, 0, domain.BangkokTZ())
	monthEnd := monthStart.AddDate(0, 1, 0)
	settlements := s.store.ListSettlements()
	byAcq := make(map[domain.Acquirer]int64)
	for _, r := range settlements {
		bd := domain.BangkokMidnight(r.SettlementDate)
		if bd.Before(monthStart) || !bd.Before(monthEnd) {
			continue
		}
		byAcq[r.Acquirer] += r.FeeMinor
	}
	items := make([]FeesItem, 0, 3)
	var total int64
	for _, acq := range []domain.Acquirer{domain.AcquirerThai, domain.AcquirerGlobal, domain.AcquirerPrompt} {
		amt := byAcq[acq]
		items = append(items, FeesItem{Acquirer: acq, Fees: domain.FormatMinorUnits(amt)})
		total += amt
	}
	return FeesResult{
		Month:          month,
		Currency:       "THB",
		FeesByAcquirer: items,
		Total:          domain.FormatMinorUnits(total),
	}, nil
}

type SettledTransaction struct {
	ID              string               `json:"id"`
	Acquirer        domain.Acquirer      `json:"acquirer"`
	GrossAmount     string               `json:"gross_amount"`
	Fee             string               `json:"fee"`
	NetAmount       string               `json:"net_amount"`
	TransactionDate string               `json:"transaction_date"`
	SettlementDate  string               `json:"settlement_date"`
	PaymentMethod   domain.PaymentMethod `json:"payment_method"`
}

type SettledResult struct {
	AsOf                string               `json:"as_of"`
	WindowDays          int                  `json:"window_days"`
	Currency            string               `json:"currency"`
	SettledTransactions []SettledTransaction `json:"settled_transactions"`
	Total               int                  `json:"total"`
}

// SettledSince returns settled transactions whose transaction date is within the window.
func (s *Service) SettledSince(days int, asOf time.Time) (SettledResult, error) {
	if days <= 0 {
		return SettledResult{}, ErrInvalidDays
	}
	asOfDay := domain.BangkokMidnight(asOf)
	cutoff := asOfDay.AddDate(0, 0, -days)
	r := reconcile.Reconcile(s.store.ListTransactions(), s.store.ListSettlements(), asOf)
	out := make([]SettledTransaction, 0)
	for _, rt := range r.Reconciled {
		if rt.Status != domain.StatusSettled {
			continue
		}
		if rt.Settlement == nil {
			continue
		}
		txnDay := domain.BangkokMidnight(rt.Transaction.TransactionDate)
		if txnDay.Before(cutoff) {
			continue
		}
		out = append(out, SettledTransaction{
			ID:              rt.Transaction.ID,
			Acquirer:        rt.Transaction.Acquirer,
			GrossAmount:     domain.FormatMinorUnits(rt.Settlement.GrossMinor),
			Fee:             domain.FormatMinorUnits(rt.Settlement.FeeMinor),
			NetAmount:       domain.FormatMinorUnits(rt.Settlement.NetMinor),
			TransactionDate: txnDay.Format("2006-01-02"),
			SettlementDate:  domain.BangkokMidnight(rt.Settlement.SettlementDate).Format("2006-01-02"),
			PaymentMethod:   rt.Transaction.PaymentMethod,
		})
	}
	return SettledResult{
		AsOf:                asOfDay.Format("2006-01-02"),
		WindowDays:          days,
		Currency:            "THB",
		SettledTransactions: out,
		Total:               len(out),
	}, nil
}

type OverdueResult struct {
	AsOf      string                 `json:"as_of"`
	Currency  string                 `json:"currency"`
	Overdue   []UnsettledTransaction `json:"overdue"`
	Total     int                    `json:"total"`
}

func (s *Service) Overdue(asOf time.Time) OverdueResult {
	asOfDay := domain.BangkokMidnight(asOf)
	r := reconcile.Reconcile(s.store.ListTransactions(), s.store.ListSettlements(), asOf)
	out := make([]UnsettledTransaction, 0)
	for _, rt := range r.Reconciled {
		if rt.Status != domain.StatusOverdue {
			continue
		}
		out = append(out, UnsettledTransaction{
			ID:                 rt.Transaction.ID,
			Acquirer:           rt.Transaction.Acquirer,
			Amount:             domain.FormatMinorUnits(rt.Transaction.AmountMinor),
			TransactionDate:    domain.BangkokMidnight(rt.Transaction.TransactionDate).Format("2006-01-02"),
			ExpectedSettleDate: domain.BangkokMidnight(rt.Transaction.ExpectedSettleDate).Format("2006-01-02"),
			Status:             rt.Status,
			PaymentMethod:      rt.Transaction.PaymentMethod,
		})
	}
	return OverdueResult{
		AsOf:     asOfDay.Format("2006-01-02"),
		Currency: "THB",
		Overdue:  out,
		Total:    len(out),
	}
}
