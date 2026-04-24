package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/api"
	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/query"
	"github.com/dannydaisun/payout-engine/internal/schedule"
	"github.com/dannydaisun/payout-engine/internal/store"
	"github.com/gin-gonic/gin"
)

// TestEndToEndPipeline exercises the full data flow:
// load 6 transactions across 3 acquirers, ingest matching settlements,
// reconcile, and verify cashflow + fees + overdue queries.
func TestEndToEndPipeline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bk := domain.BangkokTZ()

	s := store.New()
	q := query.New(s)
	srv := api.NewServer(s, q)
	r := gin.New()
	srv.Routes(r)

	// 6 transactions: 2 per acquirer
	// Use dates relative to a fixed asOf so we control settled/pending/overdue mix
	asOf := time.Date(2026, 4, 24, 12, 0, 0, 0, bk)
	dayBefore := asOf.AddDate(0, 0, -1)   // April 23
	twoDaysBefore := asOf.AddDate(0, 0, -2) // April 22
	weekBefore := asOf.AddDate(0, 0, -7)   // April 17

	txns := []domain.Transaction{
		{ID: "T1", Acquirer: domain.AcquirerThai, AmountMinor: 100000, Currency: "THB", TransactionDate: dayBefore, PaymentMethod: domain.MethodCreditCard},
		{ID: "T2", Acquirer: domain.AcquirerThai, AmountMinor: 50000, Currency: "THB", TransactionDate: weekBefore, PaymentMethod: domain.MethodCreditCard},
		{ID: "T3", Acquirer: domain.AcquirerGlobal, AmountMinor: 200000, Currency: "THB", TransactionDate: twoDaysBefore, PaymentMethod: domain.MethodPromptPay},
		{ID: "T4", Acquirer: domain.AcquirerGlobal, AmountMinor: 75000, Currency: "THB", TransactionDate: weekBefore, PaymentMethod: domain.MethodBankTransfer},
		{ID: "T5", Acquirer: domain.AcquirerPrompt, AmountMinor: 150000, Currency: "THB", TransactionDate: dayBefore, PaymentMethod: domain.MethodPromptPay},
		{ID: "T6", Acquirer: domain.AcquirerPrompt, AmountMinor: 25000, Currency: "THB", TransactionDate: weekBefore, PaymentMethod: domain.MethodTrueMoneyWallet},
	}
	for i := range txns {
		expDate, err := schedule.ExpectedSettlementDate(txns[i].Acquirer, txns[i].TransactionDate)
		if err != nil {
			t.Fatalf("schedule failed for %s: %v", txns[i].ID, err)
		}
		txns[i].ExpectedSettleDate = expDate
		s.SaveTransaction(txns[i])
	}

	// Ingest 3 settlement files with 1 settled row per acquirer (T1, T3, T5)
	thaiCSV := "txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n" +
		"T1,2026-04-23,2026-04-24,1000.00,25.00,975.00,credit_card\n"
	postOK(t, r, "/ingest/settlements/ThaiAcquirer", thaiCSV)

	globalCSV := "reference_number,processed_on,payout_date,original_amount,processing_fee,settled_amount,type\n" +
		"T3,22/04/2026,24/04/2026,2000.00,50.00,1950.00,promptpay\n"
	postOK(t, r, "/ingest/settlements/GlobalPay", globalCSV)

	promptJSON := `[{"transaction_id":"T5","txn_date":"2026-04-23T00:00:00+07:00","settle_date":"2026-04-26T00:00:00+07:00","amount":1500.00,"merchant_fee":22.50,"net_payout":1477.50,"channel":"promptpay"}]`
	postOK(t, r, "/ingest/settlements/PromptPayProcessor", promptJSON)

	// Cashflow query for April 24 — should sum T1 (Thai 975) + T3 (Global 1950) = 2925.00
	w := getOK(t, r, "/queries/cashflow?date=2026-04-24")
	var cf query.CashflowResult
	mustDecode(t, w.Body.Bytes(), &cf)
	if cf.Total != "2925.00" {
		t.Errorf("cashflow total: got %q, want 2925.00", cf.Total)
	}

	// Fees by acquirer for April — should sum 25 + 50 + 22.50 = 97.50
	w = getOK(t, r, "/queries/fees?month=2026-04")
	var fees query.FeesResult
	mustDecode(t, w.Body.Bytes(), &fees)
	if fees.Total != "97.50" {
		t.Errorf("fees total: got %q, want 97.50", fees.Total)
	}

	// Overdue as of April 24 — T2, T4, T6 (week-old txns with no settlement, expected dates already past)
	w = getOK(t, r, "/queries/overdue?as_of=2026-04-24")
	var overdue query.OverdueResult
	mustDecode(t, w.Body.Bytes(), &overdue)
	if overdue.Total != 3 {
		t.Errorf("overdue count: got %d, want 3", overdue.Total)
	}
}

func postOK(t *testing.T, r http.Handler, path, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST %s: got %d, body=%s", path, w.Code, w.Body.String())
	}
}

func getOK(t *testing.T, r http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: got %d, body=%s", path, w.Code, w.Body.String())
	}
	return w
}

func mustDecode(t *testing.T, body []byte, v interface{}) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(v); err != nil {
		t.Fatalf("decode: %v, body=%s", err, string(body))
	}
}

// ensure strings package not unused
var _ = strings.NewReader
