package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrNegativeAmount  = errors.New("amount must not be negative")
	ErrTooManyDecimals = errors.New("amount must have at most 2 decimal places")
	ErrInvalidAmount   = errors.New("invalid amount format")
)

func ParseMinorUnits(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidAmount
	}
	if strings.HasPrefix(s, "-") {
		return 0, ErrNegativeAmount
	}
	parts := strings.SplitN(s, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	var frac int64
	if len(parts) == 2 {
		fracStr := parts[1]
		if len(fracStr) > 2 {
			return 0, ErrTooManyDecimals
		}
		if len(fracStr) == 1 {
			fracStr += "0"
		}
		frac, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
	}
	return whole*100 + frac, nil
}

func FormatMinorUnits(minor int64) string {
	whole := minor / 100
	frac := minor % 100
	return fmt.Sprintf("%d.%02d", whole, frac)
}
