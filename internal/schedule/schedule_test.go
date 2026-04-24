package schedule

import (
	"testing"
	"time"

	"github.com/dannydaisun/payout-engine/internal/domain"
)

func TestThaiAcquirerNextBusinessDay(t *testing.T) {
	txn := time.Date(2026, 4, 20, 12, 0, 0, 0, domain.BangkokTZ()) // Mon
	got, err := ExpectedSettlementDate(domain.AcquirerThai, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 21, 0, 0, 0, 0, domain.BangkokTZ())
	if !got.Equal(want) {
		t.Fatalf("ThaiAcquirer: got %s, want %s", got, want)
	}
}

func TestGlobalPayMondayGoesToTuesday(t *testing.T) {
	txn := time.Date(2026, 4, 20, 12, 0, 0, 0, domain.BangkokTZ()) // Mon
	got, err := ExpectedSettlementDate(domain.AcquirerGlobal, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 21, 0, 0, 0, 0, domain.BangkokTZ()) // Tue
	if !got.Equal(want) {
		t.Fatalf("GlobalPay Mon->Tue: got %s, want %s", got, want)
	}
}

func TestGlobalPayWednesdayGoesToFriday(t *testing.T) {
	txn := time.Date(2026, 4, 22, 12, 0, 0, 0, domain.BangkokTZ()) // Wed
	got, err := ExpectedSettlementDate(domain.AcquirerGlobal, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 24, 0, 0, 0, 0, domain.BangkokTZ()) // Fri
	if !got.Equal(want) {
		t.Fatalf("GlobalPay Wed->Fri: got %s, want %s", got, want)
	}
}

func TestPromptPayT3Weekday(t *testing.T) {
	txn := time.Date(2026, 4, 20, 12, 0, 0, 0, domain.BangkokTZ()) // Mon
	got, err := ExpectedSettlementDate(domain.AcquirerPrompt, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 23, 0, 0, 0, 0, domain.BangkokTZ()) // Thu
	if !got.Equal(want) {
		t.Fatalf("PromptPay T+3: got %s, want %s", got, want)
	}
}

func TestThaiAcquirerFridaySkipsToMonday(t *testing.T) {
	txn := time.Date(2026, 4, 24, 12, 0, 0, 0, domain.BangkokTZ()) // Fri
	got, err := ExpectedSettlementDate(domain.AcquirerThai, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 27, 0, 0, 0, 0, domain.BangkokTZ()) // Mon
	if !got.Equal(want) {
		t.Fatalf("ThaiAcquirer Fri->Mon: got %s, want %s", got, want)
	}
}

func TestGlobalPayTuesdaySkipsToFriday(t *testing.T) {
	txn := time.Date(2026, 4, 21, 12, 0, 0, 0, domain.BangkokTZ()) // Tue (same-day ineligible)
	got, err := ExpectedSettlementDate(domain.AcquirerGlobal, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 24, 0, 0, 0, 0, domain.BangkokTZ()) // Fri
	if !got.Equal(want) {
		t.Fatalf("GlobalPay Tue->Fri: got %s, want %s", got, want)
	}
}

func TestGlobalPayFridaySkipsToNextTuesday(t *testing.T) {
	txn := time.Date(2026, 4, 24, 12, 0, 0, 0, domain.BangkokTZ()) // Fri
	got, err := ExpectedSettlementDate(domain.AcquirerGlobal, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 28, 0, 0, 0, 0, domain.BangkokTZ()) // Tue
	if !got.Equal(want) {
		t.Fatalf("GlobalPay Fri->next Tue: got %s, want %s", got, want)
	}
}

func TestPromptPayFridayTxnSettlesWednesday(t *testing.T) {
	txn := time.Date(2026, 4, 24, 12, 0, 0, 0, domain.BangkokTZ()) // Fri
	got, err := ExpectedSettlementDate(domain.AcquirerPrompt, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 29, 0, 0, 0, 0, domain.BangkokTZ()) // Wed (Mon, Tue, Wed = T+3 biz days)
	if !got.Equal(want) {
		t.Fatalf("PromptPay Fri T+3 across weekend: got %s, want %s", got, want)
	}
}

func TestPromptPayT3AcrossWeekend(t *testing.T) {
	txn := time.Date(2026, 4, 22, 12, 0, 0, 0, domain.BangkokTZ()) // Wed
	got, err := ExpectedSettlementDate(domain.AcquirerPrompt, txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 27, 0, 0, 0, 0, domain.BangkokTZ()) // Mon (Thu, Fri, Mon = T+3 skipping Sat/Sun)
	if !got.Equal(want) {
		t.Fatalf("PromptPay Wed T+3 across weekend: got %s, want %s", got, want)
	}
}

func TestUnknownAcquirerReturnsError(t *testing.T) {
	txn := time.Date(2026, 4, 20, 12, 0, 0, 0, domain.BangkokTZ())
	_, err := ExpectedSettlementDate(domain.Acquirer("FakeAcquirer"), txn)
	if err == nil {
		t.Fatalf("expected error for unknown acquirer, got nil")
	}
}
