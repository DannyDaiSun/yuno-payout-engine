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
// per-transaction status plus a list of discrepancies.
//
// Discrepancy rules:
//   - DiscrepancyUnknownTransaction: a settlement references a transaction ID
//     that does not exist in the txns list.
//   - DiscrepancyAcquirerMismatch: a settlement exists for the txn ID but with
//     a different Acquirer than the transaction. The transaction is not marked
//     as settled (it is treated as missing settlement: overdue or pending).
//   - DiscrepancyAmountMismatch: a settlement matches (TxnID, Acquirer) but the
//     GrossMinor does not equal the transaction's AmountMinor. The transaction
//     is still marked settled (matching by ID is authoritative for status).
func Reconcile(txns []domain.Transaction, settlements []domain.SettlementRecord, asOf time.Time) Result {
	// Index settlements by (txnID, acquirer) for O(1) exact match lookup.
	settlementIdx := make(map[settlementKey]*domain.SettlementRecord, len(settlements))
	// Secondary index: settlements grouped by TxnID alone, used to detect
	// acquirer mismatches (settlement exists for the ID but with a different acquirer).
	settlementsByTxn := make(map[string][]*domain.SettlementRecord, len(settlements))
	for i := range settlements {
		s := &settlements[i]
		settlementIdx[settlementKey{TxnID: s.TransactionID, Acquirer: s.Acquirer}] = s
		settlementsByTxn[s.TransactionID] = append(settlementsByTxn[s.TransactionID], s)
	}

	// Build a set of known transaction IDs to detect "unknown transaction" settlements.
	txnIDs := make(map[string]domain.Acquirer, len(txns))
	for _, t := range txns {
		txnIDs[t.ID] = t.Acquirer
	}

	// Track which settlements were matched (exact match or via acquirer mismatch
	// against a known txn) so we can flag truly unknown ones afterwards.
	consumed := make(map[settlementKey]bool, len(settlements))

	reconciled := make([]domain.ReconciledTransaction, 0, len(txns))
	discrepancies := make([]domain.Discrepancy, 0)

	for _, t := range txns {
		exactKey := settlementKey{TxnID: t.ID, Acquirer: t.Acquirer}
		if rec, ok := settlementIdx[exactKey]; ok {
			consumed[exactKey] = true
			// Amount mismatch: still settled, but flag a discrepancy.
			if rec.GrossMinor != t.AmountMinor {
				discrepancies = append(discrepancies, domain.Discrepancy{
					TransactionID: t.ID,
					Acquirer:      t.Acquirer,
					Reason:        domain.DiscrepancyAmountMismatch,
					Detail: fmt.Sprintf(
						"settlement gross %d does not match transaction amount %d",
						rec.GrossMinor, t.AmountMinor,
					),
				})
			}
			reconciled = append(reconciled, domain.ReconciledTransaction{
				Transaction: t,
				Status:      domain.StatusSettled,
				Settlement:  rec,
			})
			continue
		}

		// No exact match. Check whether a settlement exists for the same txn ID
		// under a different acquirer — that's an acquirer mismatch.
		for _, s := range settlementsByTxn[t.ID] {
			if s.Acquirer != t.Acquirer {
				consumed[settlementKey{TxnID: s.TransactionID, Acquirer: s.Acquirer}] = true
				discrepancies = append(discrepancies, domain.Discrepancy{
					TransactionID: t.ID,
					Acquirer:      s.Acquirer,
					Reason:        domain.DiscrepancyAcquirerMismatch,
					Detail: fmt.Sprintf(
						"settlement acquirer %s does not match transaction acquirer %s for txn %s",
						s.Acquirer, t.Acquirer, t.ID,
					),
				})
			}
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

	// Any settlement not consumed (no matching txn ID at all) is "unknown transaction".
	for k, s := range settlementIdx {
		if consumed[k] {
			continue
		}
		if _, known := txnIDs[s.TransactionID]; known {
			// Known txn ID but neither exact-matched nor flagged as acquirer mismatch.
			// This shouldn't happen given the loop above, but skip defensively.
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
