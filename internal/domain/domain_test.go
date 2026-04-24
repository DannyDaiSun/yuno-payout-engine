package domain

import (
	"testing"
	"time"
)

func TestParseAmountToMinorUnits(t *testing.T) {
	got, err := ParseMinorUnits("1000.25")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 100025 {
		t.Errorf("got %d, want 100025", got)
	}
}

func TestParseAmountRejectsNegative(t *testing.T) {
	_, err := ParseMinorUnits("-5.00")
	if err != ErrNegativeAmount {
		t.Errorf("got %v, want ErrNegativeAmount", err)
	}
}

func TestParseAmountRejectsMoreThanTwoDecimals(t *testing.T) {
	_, err := ParseMinorUnits("100.001")
	if err != ErrTooManyDecimals {
		t.Errorf("got %v, want ErrTooManyDecimals", err)
	}
}

func TestFormatMinorUnits(t *testing.T) {
	got := FormatMinorUnits(100025)
	want := "1000.25"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBangkokMidnightNormalizes(t *testing.T) {
	utc := time.Date(2026, 4, 24, 23, 30, 0, 0, time.UTC)
	got := BangkokMidnight(utc)
	if got.Year() != 2026 || got.Month() != 4 || got.Day() != 25 {
		t.Errorf("got %v, want 2026-04-25 00:00 Bangkok", got)
	}
	if got.Location().String() != "Asia/Bangkok" && got.Location().String() != "ICT" {
		t.Errorf("got tz %v, want Asia/Bangkok or ICT", got.Location())
	}
}
