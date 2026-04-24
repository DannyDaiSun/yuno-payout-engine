package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dannydaisun/payout-engine/internal/anomaly"
	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/ingest"
	"github.com/dannydaisun/payout-engine/internal/query"
	"github.com/dannydaisun/payout-engine/internal/schedule"
	"github.com/dannydaisun/payout-engine/internal/store"
	"github.com/gin-gonic/gin"
)

type Server struct {
	store *store.Store
	query *query.Service
}

func NewServer(s *store.Store, q *query.Service) *Server {
	return &Server{store: s, query: q}
}

func (srv *Server) Routes(r *gin.Engine) {
	r.GET("/health", srv.health)
	r.POST("/ingest/transactions", srv.ingestTransactions)
	r.POST("/ingest/settlements/:acquirer", srv.ingestSettlements)
	r.GET("/queries/cashflow", srv.cashflow)
	r.GET("/queries/unsettled", srv.unsettled)
	r.GET("/queries/fees", srv.fees)
	r.GET("/queries/overdue", srv.overdue)
	r.GET("/queries/settled", srv.settled)
	r.GET("/queries/anomalies", srv.anomalies)
}

type AnomaliesResult struct {
	Total     int               `json:"total"`
	Anomalies []anomaly.Anomaly `json:"anomalies"`
}

func (srv *Server) anomalies(c *gin.Context) {
	settlements := srv.store.ListSettlements()
	found := anomaly.Detect(settlements)
	c.JSON(http.StatusOK, AnomaliesResult{Total: len(found), Anomalies: found})
}

func (srv *Server) settled(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	var days int
	if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil {
		c.JSON(http.StatusBadRequest, errResp("invalid_days", fmt.Sprintf("days must be int: %s", daysStr)))
		return
	}
	asOfStr := c.Query("as_of")
	asOf := bangkokToday()
	if asOfStr != "" {
		d, err := parseBangkokDate(asOfStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errResp("invalid_date", fmt.Sprintf("as_of must be YYYY-MM-DD: %s", asOfStr)))
			return
		}
		asOf = d
	}
	res, err := srv.query.SettledSince(days, asOf)
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp("invalid_days", err.Error()))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (srv *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type ingestResult struct {
	Acquirer    string `json:"acquirer,omitempty"`
	RecordCount int    `json:"record_count"`
	Status      string `json:"status"`
}

func errResp(code, msg string) gin.H {
	return gin.H{"error": code, "message": msg}
}

func (srv *Server) ingestTransactions(c *gin.Context) {
	records, err := LoadTransactionsCSV(c.Request.Body, "upload")
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp("invalid_transactions_file", err.Error()))
		return
	}
	count := 0
	for _, t := range records {
		expDate, err := schedule.ExpectedSettlementDate(t.Acquirer, t.TransactionDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, errResp("invalid_acquirer", err.Error()))
			return
		}
		t.ExpectedSettleDate = expDate
		srv.store.SaveTransaction(t)
		count++
	}
	c.JSON(http.StatusOK, ingestResult{RecordCount: count, Status: "ok"})
}

func (srv *Server) ingestSettlements(c *gin.Context) {
	acq := domain.Acquirer(c.Param("acquirer"))
	var records []domain.SettlementRecord
	var err error
	switch acq {
	case domain.AcquirerThai:
		records, err = ingest.ParseThaiCSV(c.Request.Body, "upload")
	case domain.AcquirerGlobal:
		records, err = ingest.ParseGlobalCSV(c.Request.Body, "upload")
	case domain.AcquirerPrompt:
		records, err = ingest.ParsePromptJSON(c.Request.Body, "upload")
	default:
		c.JSON(http.StatusBadRequest, errResp("unknown_acquirer", fmt.Sprintf("unknown acquirer: %s", acq)))
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp("invalid_settlement_file", err.Error()))
		return
	}
	for _, r := range records {
		srv.store.SaveSettlement(r)
	}
	c.JSON(http.StatusOK, ingestResult{Acquirer: string(acq), RecordCount: len(records), Status: "ok"})
}

func parseBangkokDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, domain.BangkokTZ())
}

func bangkokTomorrow() time.Time {
	now := time.Now().In(domain.BangkokTZ())
	return domain.BangkokMidnight(now).AddDate(0, 0, 1)
}

func bangkokToday() time.Time {
	return domain.BangkokMidnight(time.Now())
}

func (srv *Server) cashflow(c *gin.Context) {
	dateStr := c.Query("date")
	var date time.Time
	if dateStr == "" {
		date = bangkokTomorrow()
	} else {
		d, err := parseBangkokDate(dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errResp("invalid_date", fmt.Sprintf("date must be YYYY-MM-DD: %s", dateStr)))
			return
		}
		date = d
	}
	c.JSON(http.StatusOK, srv.query.ExpectedCashByAcquirer(date))
}

func (srv *Server) unsettled(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	var days int
	if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil {
		c.JSON(http.StatusBadRequest, errResp("invalid_days", fmt.Sprintf("days must be int: %s", daysStr)))
		return
	}
	asOfStr := c.Query("as_of")
	asOf := bangkokToday()
	if asOfStr != "" {
		d, err := parseBangkokDate(asOfStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errResp("invalid_date", fmt.Sprintf("as_of must be YYYY-MM-DD: %s", asOfStr)))
			return
		}
		asOf = d
	}
	res, err := srv.query.UnsettledSince(days, asOf)
	if err != nil {
		c.JSON(http.StatusBadRequest, errResp("invalid_days", err.Error()))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (srv *Server) fees(c *gin.Context) {
	month := c.Query("month")
	if month == "" {
		month = time.Now().In(domain.BangkokTZ()).Format("2006-01")
	}
	res, err := srv.query.FeesByAcquirer(month)
	if err != nil {
		if errors.Is(err, query.ErrInvalidMonth) {
			c.JSON(http.StatusBadRequest, errResp("invalid_month", err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, errResp("internal_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (srv *Server) overdue(c *gin.Context) {
	asOfStr := c.Query("as_of")
	asOf := bangkokToday()
	if asOfStr != "" {
		d, err := parseBangkokDate(asOfStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errResp("invalid_date", fmt.Sprintf("as_of must be YYYY-MM-DD: %s", asOfStr)))
			return
		}
		asOf = d
	}
	c.JSON(http.StatusOK, srv.query.Overdue(asOf))
}
