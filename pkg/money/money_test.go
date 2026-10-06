package money

import (
	"errors"
	"testing"
)

func TestMoney_Add(t *testing.T) {
	m1, _ := New(1050, USD)
	m2, _ := New(250, USD)

	sum, err := m1.Add(m2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.Amount != 1300 || sum.Currency != USD {
		t.Errorf("expected 1300 USD, got %d %s", sum.Amount, sum.Currency)
	}
}

func TestMoney_CurrencyMismatch(t *testing.T) {
	m1, _ := New(100, USD)
	m2, _ := New(100, EUR)

	_, err := m1.Add(m2)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func TestMoney_FromDecimal(t *testing.T) {
	tests := []struct {
		input    string
		currency Currency
		expected int64
	}{
		{"10.50", USD, 1050},
		{"0.99", EUR, 99},
		{"100", USD, 10000},
		{"5.5", RUB, 550},
	}

	for _, tt := range tests {
		m, err := FromDecimal(tt.input, tt.currency, 2)
		if err != nil {
			t.Errorf("FromDecimal(%q) error: %v", tt.input, err)
			continue
		}
		if m.Amount != tt.expected {
			t.Errorf("FromDecimal(%q) = %d, expected %d", tt.input, m.Amount, tt.expected)
		}
	}
}
