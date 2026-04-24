package store

import (
	"sync"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

// settlementKey uniquely identifies a settlement by transaction ID + acquirer.
type settlementKey struct {
	TxnID    string
	Acquirer domain.Acquirer
}

// Store is a thread-safe in-memory store for transactions and settlements.
type Store struct {
	mu          sync.RWMutex
	txns        map[string]domain.Transaction
	settlements map[settlementKey]domain.SettlementRecord
}

// New constructs an empty Store.
func New() *Store {
	return &Store{
		txns:        make(map[string]domain.Transaction),
		settlements: make(map[settlementKey]domain.SettlementRecord),
	}
}

// SaveTransaction stores or replaces a transaction by ID (idempotent).
func (s *Store) SaveTransaction(t domain.Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txns[t.ID] = t
}

// GetTransaction returns a transaction by ID and whether it exists.
func (s *Store) GetTransaction(id string) (domain.Transaction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.txns[id]
	return t, ok
}

// ListTransactions returns a snapshot slice of all transactions.
func (s *Store) ListTransactions() []domain.Transaction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Transaction, 0, len(s.txns))
	for _, t := range s.txns {
		out = append(out, t)
	}
	return out
}

// SaveSettlement stores or replaces a settlement by (txnID, acquirer) (idempotent).
func (s *Store) SaveSettlement(r domain.SettlementRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settlements[settlementKey{TxnID: r.TransactionID, Acquirer: r.Acquirer}] = r
}

// FindSettlement returns the settlement matching (txnID, acquirer) and whether it exists.
func (s *Store) FindSettlement(txnID string, acquirer domain.Acquirer) (domain.SettlementRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.settlements[settlementKey{TxnID: txnID, Acquirer: acquirer}]
	return r, ok
}

// ListSettlements returns a snapshot slice of all settlements.
func (s *Store) ListSettlements() []domain.SettlementRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.SettlementRecord, 0, len(s.settlements))
	for _, r := range s.settlements {
		out = append(out, r)
	}
	return out
}
