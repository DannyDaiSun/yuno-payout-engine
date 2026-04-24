package domain

import "time"

type Acquirer string

const (
	AcquirerThai   Acquirer = "ThaiAcquirer"
	AcquirerGlobal Acquirer = "GlobalPay"
	AcquirerPrompt Acquirer = "PromptPayProcessor"
)

type PaymentMethod string

const (
	MethodCreditCard      PaymentMethod = "credit_card"
	MethodPromptPay       PaymentMethod = "promptpay"
	MethodTrueMoneyWallet PaymentMethod = "truemoney_wallet"
	MethodBankTransfer    PaymentMethod = "bank_transfer"
)

type SettlementStatus string

const (
	StatusSettled SettlementStatus = "settled"
	StatusPending SettlementStatus = "pending"
	StatusOverdue SettlementStatus = "overdue"
)

type DiscrepancyReason string

const (
	DiscrepancyUnknownTransaction DiscrepancyReason = "unknown_transaction"
	DiscrepancyAcquirerMismatch   DiscrepancyReason = "acquirer_mismatch"
	DiscrepancyAmountMismatch     DiscrepancyReason = "amount_mismatch"
	DiscrepancyDuplicateSettlement DiscrepancyReason = "duplicate_settlement"
	DiscrepancyNetCalculationError DiscrepancyReason = "net_calculation_error"
)

type Transaction struct {
	ID                 string
	Acquirer           Acquirer
	AmountMinor        int64
	Currency           string
	TransactionDate    time.Time
	PaymentMethod      PaymentMethod
	ExpectedSettleDate time.Time
}

type SettlementRecord struct {
	TransactionID   string
	Acquirer        Acquirer
	GrossMinor      int64
	FeeMinor        int64
	NetMinor        int64
	Currency        string
	TransactionDate time.Time
	SettlementDate  time.Time
	PaymentMethod   PaymentMethod
	SourceFile      string
}

type ReconciledTransaction struct {
	Transaction Transaction
	Status      SettlementStatus
	Settlement  *SettlementRecord
}

type Discrepancy struct {
	TransactionID string
	Acquirer      Acquirer
	Reason        DiscrepancyReason
	Detail        string
}
