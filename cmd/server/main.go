package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dannydaisun/payout-engine/internal/api"
	"github.com/dannydaisun/payout-engine/internal/domain"
	"github.com/dannydaisun/payout-engine/internal/ingest"
	"github.com/dannydaisun/payout-engine/internal/query"
	"github.com/dannydaisun/payout-engine/internal/schedule"
	"github.com/dannydaisun/payout-engine/internal/store"
	"github.com/gin-gonic/gin"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	seedDir := flag.String("seed", "./data", "fixture directory to load at startup (empty to skip)")
	flag.Parse()

	s := store.New()
	q := query.New(s)

	if *seedDir != "" {
		if err := loadFixtures(s, *seedDir); err != nil {
			log.Fatalf("seed load failed: %v", err)
		}
		log.Printf("seeded %d transactions, %d settlements from %s",
			len(s.ListTransactions()), len(s.ListSettlements()), *seedDir)
	}

	srv := api.NewServer(s, q)
	r := gin.Default()
	srv.Routes(r)
	log.Printf("listening on %s", *addr)
	log.Fatal(r.Run(*addr))
}

func loadFixtures(s *store.Store, dir string) error {
	txnPath := filepath.Join(dir, "transactions.csv")
	if f, err := os.Open(txnPath); err == nil {
		txns, perr := api.LoadTransactionsCSV(f, txnPath)
		f.Close()
		if perr != nil {
			return fmt.Errorf("load transactions: %w", perr)
		}
		for _, t := range txns {
			expDate, err := schedule.ExpectedSettlementDate(t.Acquirer, t.TransactionDate)
			if err != nil {
				return fmt.Errorf("schedule for %s: %w", t.ID, err)
			}
			t.ExpectedSettleDate = expDate
			s.SaveTransaction(t)
		}
	}
	settlementFiles := []struct {
		path     string
		acquirer domain.Acquirer
	}{
		{filepath.Join(dir, "settlements", "thai_acquirer.csv"), domain.AcquirerThai},
		{filepath.Join(dir, "settlements", "global_pay.csv"), domain.AcquirerGlobal},
		{filepath.Join(dir, "settlements", "promptpay.json"), domain.AcquirerPrompt},
	}
	for _, sf := range settlementFiles {
		f, err := os.Open(sf.path)
		if err != nil {
			log.Printf("skip settlement file %s: %v", sf.path, err)
			continue
		}
		var records []domain.SettlementRecord
		switch sf.acquirer {
		case domain.AcquirerThai:
			records, err = ingest.ParseThaiCSV(f, sf.path)
		case domain.AcquirerGlobal:
			records, err = ingest.ParseGlobalCSV(f, sf.path)
		case domain.AcquirerPrompt:
			records, err = ingest.ParsePromptJSON(f, sf.path)
		}
		f.Close()
		if err != nil {
			return fmt.Errorf("parse %s: %w", sf.path, err)
		}
		for _, r := range records {
			s.SaveSettlement(r)
		}
	}
	return nil
}
