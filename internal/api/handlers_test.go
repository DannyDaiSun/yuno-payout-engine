package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
