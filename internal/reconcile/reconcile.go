package reconcile

import (
	"fmt"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

// Result contains the reconciled transactions and any discrepancies found.
type Result struct {
	Reconciled    []domain.ReconciledTransaction
	Discrepancies []domain.Discrepancy
}

// settlementKey uniquely identifies a settlement record by txn ID + acquirer.
type settlementKey struct {
	TxnID    string
	Acquirer domain.Acquirer
}

// Reconcile matches transactions against settlement records and produces a
// per-transaction status plus a list of discrepancies for unknown settlements.
func Reconcile(txns []domain.Transaction, settlements []domain.SettlementRecord, asOf time.Time) Result {
	// Index settlements by (txnID, acquirer) for O(1) lookup.
	settlementIdx := make(map[settlementKey]*domain.SettlementRecord, len(settlements))
	for i := range settlements {
		s := &settlements[i]
		settlementIdx[settlementKey{TxnID: s.TransactionID, Acquirer: s.Acquirer}] = s
	}

	// Track which settlements were matched so we can flag unknown ones afterwards.
	matched := make(map[settlementKey]bool, len(settlements))

	reconciled := make([]domain.ReconciledTransaction, 0, len(txns))
	for _, t := range txns {
		key := settlementKey{TxnID: t.ID, Acquirer: t.Acquirer}
		if rec, ok := settlementIdx[key]; ok {
			matched[key] = true
			reconciled = append(reconciled, domain.ReconciledTransaction{
				Transaction: t,
				Status:      domain.StatusSettled,
				Settlement:  rec,
			})
			continue
		}
		status := domain.StatusOverdue
		if t.ExpectedSettleDate.After(asOf) {
			status = domain.StatusPending
		}
		reconciled = append(reconciled, domain.ReconciledTransaction{
			Transaction: t,
			Status:      status,
			Settlement:  nil,
		})
	}

	discrepancies := make([]domain.Discrepancy, 0)
	for k, s := range settlementIdx {
		if matched[k] {
			continue
		}
		discrepancies = append(discrepancies, domain.Discrepancy{
			TransactionID: s.TransactionID,
			Acquirer:      s.Acquirer,
			Reason:        domain.DiscrepancyUnknownTransaction,
			Detail:        fmt.Sprintf("settlement references unknown transaction %s for acquirer %s", s.TransactionID, s.Acquirer),
		})
	}

	return Result{
		Reconciled:    reconciled,
		Discrepancies: discrepancies,
	}
}
