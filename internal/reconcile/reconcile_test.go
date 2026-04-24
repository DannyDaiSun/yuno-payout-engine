package reconcile

import (
	"strings"
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

func TestReconcileMarksPastDueAsOverdue(t *testing.T) {
	asOf := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	txn := domain.Transaction{
		ID:                 "T3",
		Acquirer:           domain.AcquirerThai,
		AmountMinor:        7500,
		Currency:           "THB",
		TransactionDate:    asOf.Add(-48 * time.Hour),
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: asOf.Add(-24 * time.Hour),
	}

	res := Reconcile([]domain.Transaction{txn}, nil, asOf)

	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Status != domain.StatusOverdue {
		t.Fatalf("expected status overdue for past-due unsettled txn, got %s", r.Status)
	}
	if r.Settlement != nil {
		t.Fatalf("expected nil settlement, got %+v", r.Settlement)
	}
}

func TestReconcileFlagsUnknownSettlementAsDiscrepancy(t *testing.T) {
	asOf := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	settlement := domain.SettlementRecord{
		TransactionID:  "GHOST",
		Acquirer:       domain.AcquirerGlobal,
		GrossMinor:     5000,
		FeeMinor:       50,
		NetMinor:       4950,
		Currency:       "THB",
		SettlementDate: asOf,
		PaymentMethod:  domain.MethodCreditCard,
	}

	res := Reconcile(nil, []domain.SettlementRecord{settlement}, asOf)

	if len(res.Discrepancies) != 1 {
		t.Fatalf("expected 1 discrepancy, got %d", len(res.Discrepancies))
	}
	d := res.Discrepancies[0]
	if d.TransactionID != "GHOST" {
		t.Fatalf("expected discrepancy TransactionID=GHOST, got %s", d.TransactionID)
	}
	if d.Reason != domain.DiscrepancyUnknownTransaction {
		t.Fatalf("expected DiscrepancyUnknownTransaction, got %s", d.Reason)
	}
}

func TestReconcileFlagsAcquirerMismatch(t *testing.T) {
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
		Acquirer:       domain.AcquirerGlobal, // wrong acquirer
		GrossMinor:     10000,
		FeeMinor:       100,
		NetMinor:       9900,
		Currency:       "THB",
		SettlementDate: asOf,
		PaymentMethod:  domain.MethodCreditCard,
	}

	res := Reconcile([]domain.Transaction{txn}, []domain.SettlementRecord{settlement}, asOf)

	// Discrepancy must be present.
	var found *domain.Discrepancy
	for i := range res.Discrepancies {
		if res.Discrepancies[i].Reason == domain.DiscrepancyAcquirerMismatch &&
			res.Discrepancies[i].TransactionID == "T1" {
			found = &res.Discrepancies[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected DiscrepancyAcquirerMismatch for T1, got %+v", res.Discrepancies)
	}

	// The transaction itself must NOT be marked as settled.
	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Status == domain.StatusSettled {
		t.Fatalf("expected acquirer-mismatch txn to NOT be settled, got %s", r.Status)
	}
	if r.Settlement != nil {
		t.Fatalf("expected nil settlement on acquirer-mismatch txn, got %+v", r.Settlement)
	}
}

func TestReconcileFlagsDuplicateSettlement(t *testing.T) {
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
	first := domain.SettlementRecord{
		TransactionID:  "T1",
		Acquirer:       domain.AcquirerThai,
		GrossMinor:     10000,
		FeeMinor:       100,
		NetMinor:       9900,
		Currency:       "THB",
		SettlementDate: asOf,
		PaymentMethod:  domain.MethodCreditCard,
	}
	second := first // same key (T1, AcquirerThai)

	res := Reconcile([]domain.Transaction{txn}, []domain.SettlementRecord{first, second}, asOf)

	// T1 must be settled using the first occurrence.
	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Status != domain.StatusSettled {
		t.Fatalf("expected status settled, got %s", r.Status)
	}
	if r.Settlement == nil {
		t.Fatalf("expected settlement attached for T1, got nil")
	}

	// Exactly one duplicate-settlement discrepancy must be emitted.
	var dupCount int
	for _, d := range res.Discrepancies {
		if d.Reason == domain.DiscrepancyDuplicateSettlement {
			dupCount++
			if d.TransactionID != "T1" {
				t.Fatalf("expected duplicate discrepancy TransactionID=T1, got %s", d.TransactionID)
			}
			if d.Acquirer != domain.AcquirerThai {
				t.Fatalf("expected duplicate discrepancy Acquirer=Thai, got %s", d.Acquirer)
			}
		}
	}
	if dupCount != 1 {
		t.Fatalf("expected exactly 1 DiscrepancyDuplicateSettlement, got %d (all=%+v)", dupCount, res.Discrepancies)
	}
}

func TestReconcileFlagsTripleDuplicateOnce(t *testing.T) {
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
	s := domain.SettlementRecord{
		TransactionID:  "T1",
		Acquirer:       domain.AcquirerThai,
		GrossMinor:     10000,
		FeeMinor:       100,
		NetMinor:       9900,
		Currency:       "THB",
		SettlementDate: asOf,
		PaymentMethod:  domain.MethodCreditCard,
	}

	// Three settlements all sharing the same (TxnID, Acquirer).
	res := Reconcile([]domain.Transaction{txn}, []domain.SettlementRecord{s, s, s}, asOf)

	// Single-with-count approach: exactly one DiscrepancyDuplicateSettlement,
	// with the count "3" in its detail.
	var found *domain.Discrepancy
	count := 0
	for i := range res.Discrepancies {
		if res.Discrepancies[i].Reason == domain.DiscrepancyDuplicateSettlement {
			count++
			found = &res.Discrepancies[i]
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 DiscrepancyDuplicateSettlement (single-with-count), got %d (all=%+v)", count, res.Discrepancies)
	}
	if found == nil {
		t.Fatalf("expected a duplicate discrepancy, got none")
	}
	if !strings.Contains(found.Detail, "3 times") {
		t.Fatalf("expected detail to mention '3 times', got %q", found.Detail)
	}

	// Transaction is still settled.
	if len(res.Reconciled) != 1 || res.Reconciled[0].Status != domain.StatusSettled {
		t.Fatalf("expected T1 settled, got %+v", res.Reconciled)
	}
}

// TestReconcileCarriesGrossFeeNet ensures the matched settlement attached to
// a reconciled transaction carries the original GrossMinor / FeeMinor /
// NetMinor values intact (not nil, not zeroed).
func TestReconcileCarriesGrossFeeNet(t *testing.T) {
	asOf := time.Date(2026, 4, 24, 12, 0, 0, 0, domain.BangkokTZ())
	txn := domain.Transaction{
		ID:                 "T1",
		Acquirer:           domain.AcquirerThai,
		AmountMinor:        100000,
		Currency:           "THB",
		TransactionDate:    asOf.Add(-24 * time.Hour),
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: asOf.Add(-1 * time.Hour),
	}
	settlement := domain.SettlementRecord{
		TransactionID:  "T1",
		Acquirer:       domain.AcquirerThai,
		GrossMinor:     100000,
		FeeMinor:       2500,
		NetMinor:       97500,
		Currency:       "THB",
		SettlementDate: asOf,
		PaymentMethod:  domain.MethodCreditCard,
	}

	res := Reconcile([]domain.Transaction{txn}, []domain.SettlementRecord{settlement}, asOf)

	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Settlement == nil {
		t.Fatalf("expected settlement attached for T1, got nil")
	}
	if r.Settlement.GrossMinor != 100000 {
		t.Fatalf("GrossMinor: got %d, want 100000", r.Settlement.GrossMinor)
	}
	if r.Settlement.FeeMinor != 2500 {
		t.Fatalf("FeeMinor: got %d, want 2500", r.Settlement.FeeMinor)
	}
	if r.Settlement.NetMinor != 97500 {
		t.Fatalf("NetMinor: got %d, want 97500", r.Settlement.NetMinor)
	}
}

// TestReconcileAsOfBeforeTxnDoesNotMarkOverdue covers the edge case where the
// transaction date itself is in the future relative to asOf. Because status is
// driven by ExpectedSettleDate vs asOf (not TransactionDate vs asOf), an
// expected settle date strictly after asOf must mark the txn pending — never
// overdue — even if the underlying transaction date is also after asOf.
func TestReconcileAsOfBeforeTxnDoesNotMarkOverdue(t *testing.T) {
	bk := domain.BangkokTZ()
	asOf := time.Date(2026, 4, 23, 12, 0, 0, 0, bk)
	txn := domain.Transaction{
		ID:                 "T_FUTURE",
		Acquirer:           domain.AcquirerThai,
		AmountMinor:        50000,
		Currency:           "THB",
		TransactionDate:    time.Date(2026, 4, 25, 0, 0, 0, 0, bk),
		PaymentMethod:      domain.MethodCreditCard,
		ExpectedSettleDate: time.Date(2026, 4, 26, 0, 0, 0, 0, bk),
	}

	res := Reconcile([]domain.Transaction{txn}, nil, asOf)

	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Status != domain.StatusPending {
		t.Fatalf("expected status pending (expected > asOf), got %s", r.Status)
	}
	if r.Status == domain.StatusOverdue {
		t.Fatalf("must not be overdue when expected settle date is after asOf")
	}
	if r.Settlement != nil {
		t.Fatalf("expected nil settlement for unmatched txn, got %+v", r.Settlement)
	}
}

func TestReconcileFlagsAmountMismatch(t *testing.T) {
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
		GrossMinor:     9000, // mismatch with txn.AmountMinor
		FeeMinor:       100,
		NetMinor:       8900,
		Currency:       "THB",
		SettlementDate: asOf,
		PaymentMethod:  domain.MethodCreditCard,
	}

	res := Reconcile([]domain.Transaction{txn}, []domain.SettlementRecord{settlement}, asOf)

	// Discrepancy with DiscrepancyAmountMismatch must be present.
	var found *domain.Discrepancy
	for i := range res.Discrepancies {
		if res.Discrepancies[i].Reason == domain.DiscrepancyAmountMismatch &&
			res.Discrepancies[i].TransactionID == "T1" {
			found = &res.Discrepancies[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected DiscrepancyAmountMismatch for T1, got %+v", res.Discrepancies)
	}

	// Transaction is still settled (matched by ID + acquirer).
	if len(res.Reconciled) != 1 {
		t.Fatalf("expected 1 reconciled txn, got %d", len(res.Reconciled))
	}
	r := res.Reconciled[0]
	if r.Status != domain.StatusSettled {
		t.Fatalf("expected amount-mismatch txn to still be settled, got %s", r.Status)
	}
	if r.Settlement == nil {
		t.Fatalf("expected settlement attached for T1, got nil")
	}
}
