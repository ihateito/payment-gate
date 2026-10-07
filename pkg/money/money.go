package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrNegativeAmount   = errors.New("amount cannot be negative")
	ErrInvalidCurrency  = errors.New("invalid currency code")
	ErrOverflow         = errors.New("amount overflow")
)

// Currency represents ISO 4217 3-letter currency code (e.g., USD, EUR, RUB).
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	RUB Currency = "RUB"
	GBP Currency = "GBP"
	KZT Currency = "KZT"
)

func (c Currency) IsValid() bool {
	if len(c) != 3 {
		return false
	}
	for i := 0; i < len(c); i++ {
		if c[i] < 'A' || c[i] > 'Z' {
			return false
		}
	}
	return true
}

// Money represents a monetary value stored in the smallest currency unit (e.g. cents, kopecks).
// Storing as int64 completely eliminates floating-point rounding inaccuracies.
type Money struct {
	// Units in minor denomination (e.g., $10.50 = 1050 cents).
	Amount   int64    `json:"amount"`
	Currency Currency `json:"currency"`
}

// New creates a new Money instance from minor units.
func New(amount int64, currency Currency) (Money, error) {
	if !currency.IsValid() {
		return Money{}, fmt.Errorf("%w: %s", ErrInvalidCurrency, currency)
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// MustNew creates a new Money instance or panics if invalid (useful in tests/constants).
func MustNew(amount int64, currency Currency) Money {
	m, err := New(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// FromDecimal converts a major-unit string (e.g. "12.34" or "100") into minor units.
func FromDecimal(value string, currency Currency, decimals int) (Money, error) {
	if !currency.IsValid() {
		return Money{}, fmt.Errorf("%w: %s", ErrInvalidCurrency, currency)
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return Money{}, fmt.Errorf("invalid decimal format: %s", value)
	}

	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("invalid major unit: %w", err)
	}

	factor := int64(1)
	for i := 0; i < decimals; i++ {
		factor *= 10
	}

	var minor int64
	if len(parts) == 2 {
		decPart := parts[1]
		if len(decPart) > decimals {
			decPart = decPart[:decimals] // truncate or round
		} else {
			decPart += strings.Repeat("0", decimals-len(decPart))
		}
		minor, err = strconv.ParseInt(decPart, 10, 64)
		if err != nil {
			return Money{}, fmt.Errorf("invalid minor unit: %w", err)
		}
	}

	total := major*factor + minor
	return Money{Amount: total, Currency: currency}, nil
}

// Add adds two Money instances of the same currency.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("%w: %s != %s", ErrCurrencyMismatch, m.Currency, other.Currency)
	}
	res := m.Amount + other.Amount
	// Overflow check
	if (res < m.Amount) != (other.Amount < 0) {
		return Money{}, ErrOverflow
	}
	return Money{Amount: res, Currency: m.Currency}, nil
}

// Sub subtracts another Money instance.
func (m Money) Sub(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("%w: %s != %s", ErrCurrencyMismatch, m.Currency, other.Currency)
	}
	res := m.Amount - other.Amount
	if (res > m.Amount) != (other.Amount < 0) {
		return Money{}, ErrOverflow
	}
	return Money{Amount: res, Currency: m.Currency}, nil
}

// String formats the Money into human-readable string.
func (m Money) String() string {
	major := m.Amount / 100
	minor := m.Amount % 100
	if minor < 0 {
		minor = -minor
	}
	return fmt.Sprintf("%d.%02d %s", major, minor, m.Currency)
}

// MarshalJSON serializes Money into JSON.
func (m Money) MarshalJSON() ([]byte, error) {
	type Alias Money
	return json.Marshal(&struct {
		Alias
		Formatted string `json:"formatted"`
	}{
		Alias:     Alias(m),
		Formatted: m.String(),
	})
}
