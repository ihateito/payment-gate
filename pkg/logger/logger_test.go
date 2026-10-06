package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestMaskPAN(t *testing.T) {
	input := "Payment processed for card 4111111111111111 securely"
	expected := "Payment processed for card 411111******1111 securely"
	got := MaskPAN(input)
	if got != expected {
		t.Errorf("MaskPAN() = %q, expected %q", got, expected)
	}
}

func TestContextLogger(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  "INFO",
		Format: "json",
		Output: &buf,
	})

	ctx := context.WithValue(context.Background(), TraceIDKey, "trace-abc-123")
	ctx = context.WithValue(ctx, IdempotencyKeyKey, "idemp-xyz-789")

	l.InfoContext(ctx, "transaction authorized", "amount", 5000)

	output := buf.String()
	if !strings.Contains(output, `"trace_id":"trace-abc-123"`) {
		t.Errorf("expected trace_id in output, got: %s", output)
	}
	if !strings.Contains(output, `"idempotency_key":"idemp-xyz-789"`) {
		t.Errorf("expected idempotency_key in output, got: %s", output)
	}
}
