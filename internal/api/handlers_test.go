package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/query"
	"github.com/dannydaisun/payout-engine/internal/store"
	"github.com/gin-gonic/gin"
)

func setupServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := store.New()
	q := query.New(s)
	srv := NewServer(s, q)
	r := gin.New()
	srv.Routes(r)
	return r
}

func TestHealthReturnsOK(t *testing.T) {
	r := setupServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestIngestRejectsUnknownAcquirerPath(t *testing.T) {
	r := setupServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ingest/settlements/FakeAcquirer", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestQueriesRejectsBadDateParam(t *testing.T) {
	r := setupServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/queries/cashflow?date=foo", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// TestQueriesCashflowDefaultsToBangkokTomorrow verifies that GET /queries/cashflow
// with no `?date=` parameter defaults to Bangkok tomorrow (T+1 BKK), not UTC.
func TestQueriesCashflowDefaultsToBangkokTomorrow(t *testing.T) {
	r := setupServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/queries/cashflow", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Date string `json:"date"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	want := domain.BangkokMidnight(time.Now()).AddDate(0, 0, 1).Format("2006-01-02")
	if resp.Date != want {
		t.Errorf("default cashflow date: got %q, want %q (Bangkok tomorrow)", resp.Date, want)
	}
}

func TestQueriesSettledRejectsBadDays(t *testing.T) {
	r := setupServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/queries/settled?days=abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestQueriesForecastRejectsBadDays(t *testing.T) {
	r := setupServer(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/queries/forecast?days=99", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// TestIngestSameFileTwiceIsIdempotent verifies uploading the same Thai
// settlements CSV twice does not double-count fees in the monthly query.
// SaveSettlement keys by (txnID, acquirer), so re-ingesting the same file
// must replace the row, not append.
func TestIngestSameFileTwiceIsIdempotent(t *testing.T) {
	r := setupServer(t)

	csvBody := "txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method\n" +
		"TXN_IDEMPOTENT,2026-04-20,2026-04-21,1000.00,25.00,975.00,credit_card\n"

	// First upload.
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/ingest/settlements/ThaiAcquirer", strings.NewReader(csvBody))
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first upload: got %d, want 200; body=%s", w1.Code, w1.Body.String())
	}

	// Second upload (same content).
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/ingest/settlements/ThaiAcquirer", strings.NewReader(csvBody))
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second upload: got %d, want 200; body=%s", w2.Code, w2.Body.String())
	}

	// Query fees for April 2026 (Bangkok TZ); the settlement_date is 2026-04-21 BKK.
	wq := httptest.NewRecorder()
	reqq := httptest.NewRequest(http.MethodGet, "/queries/fees?month=2026-04", nil)
	r.ServeHTTP(wq, reqq)
	if wq.Code != http.StatusOK {
		t.Fatalf("fees query: got %d, want 200; body=%s", wq.Code, wq.Body.String())
	}
	var resp struct {
		Total string `json:"total"`
	}
	if err := json.Unmarshal(wq.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode fees response: %v; body=%s", err, wq.Body.String())
	}
	if resp.Total != "25.00" {
		t.Errorf("fees total after duplicate upload: got %q, want 25.00 (single fee, not doubled)", resp.Total)
	}
}
