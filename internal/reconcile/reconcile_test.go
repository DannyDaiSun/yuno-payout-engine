package reconcile

import (
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

func TestReconcileMarksMatchedAsSettled(t *testing.T) {
	asOf := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	txn := domain.Transaction{
		ID:                 "T1",
		Acquirer:           domain.AcquirerThai,
		AmountMinor:        10000,
		Currency:           "THB",
		TransactionDate:    asOf.Add(-24 * time.Hour),
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: asOf.Add(-1 * time.Hour),
	}
	settlement := domain.SettlementRecord{
		TransactionID:  "T1",
		Acquirer:       domain.AcquirerThai,
		GrossMinor:     10000,
		FeeMinor:       100,
		NetMinor:       9900,
		Currency:       "THB",
		SettlementDate: asOf,
		PaymentMethod:  domain.MethodCreditCard,
	}

	res := Reconcile([]domain.Transaction{txn}, []domain.SettlementRecord{settlement}, asOf)

	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Transaction.ID != "T1" {
		t.Fatalf("expected txn ID T1, got %s", r.Transaction.ID)
	}
	if r.Status != domain.StatusSettled {
		t.Fatalf("expected status settled, got %s", r.Status)
	}
	if r.Settlement == nil || r.Settlement.TransactionID != "T1" {
		t.Fatalf("expected settlement attached for T1, got %+v", r.Settlement)
	}
	if len(res.Discrepancies) != 0 {
		t.Fatalf("expected no discrepancies, got %d", len(res.Discrepancies))
	}
}

func TestReconcileMarksFutureExpectedAsPending(t *testing.T) {
	asOf := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	txn := domain.Transaction{
		ID:                 "T2",
		Acquirer:           domain.AcquirerGlobal,
		AmountMinor:        5000,
		Currency:           "THB",
		TransactionDate:    asOf,
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: asOf.Add(24 * time.Hour),
	}

	res := Reconcile([]domain.Transaction{txn}, nil, asOf)

	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Transaction.ID != "T2" {
		t.Fatalf("expected txn ID T2, got %s", r.Transaction.ID)
	}
	if r.Status != domain.StatusPending {
		t.Fatalf("expected status pending, got %s", r.Status)
	}
	if r.Settlement != nil {
		t.Fatalf("expected nil settlement for unmatched txn, got %+v", r.Settlement)
	}
}
